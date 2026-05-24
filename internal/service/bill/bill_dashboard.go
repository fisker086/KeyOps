package bill

import (
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/repository"
)

type VendorCostEntry struct {
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`
}

type DashboardData struct {
	CurrentMonthCost float64                      `json:"current_month_cost"`
	LastMonthCost    float64                      `json:"last_month_cost"`
	ForecastCost     float64                      `json:"forecast_cost"`
	ChangePercent    float64                      `json:"change_percent"`
	BaseCurrency     string                       `json:"base_currency"`
	TopResources     []TopResource                `json:"top_resources"`
	CostByVendor     map[string]VendorCostEntry   `json:"cost_by_vendor"`
	CostByService    map[string]float64           `json:"cost_by_service"`
	CostByRegion     map[string]float64           `json:"cost_by_region"`
	CostTrend        []DailyCost                  `json:"cost_trend"`
}

type TopResource struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ServiceCode  string  `json:"service_code"`
	Cost         float64 `json:"cost"`
	Vendor       string  `json:"vendor"`
	Currency     string  `json:"currency"`
}

type DailyCost struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// GetDashboardData 获取 Dashboard 数据
// baseCurrency: "CNY" or "USD" — 聚合图表统一转为此币种显示
func (s *BillService) GetDashboardData(baseCurrency string) (*DashboardData, error) {
	now := time.Now()
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	type queryResult struct {
		currentCost   float64
		lastCost      float64
		topResources  []map[string]interface{}
		costByVendor  map[string]repository.VendorCost
		costByService map[string]float64
		costByRegion  map[string]float64
		costTrend     []map[string]interface{}
		errs          []error
	}

	var res queryResult
	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		res.currentCost, _ = s.repo.GetTotalCostByMonth(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.lastCost, _ = s.repo.GetTotalCostByMonth(lastMonth)
	}()
	go func() {
		defer wg.Done()
		res.topResources, _ = s.repo.GetTopResources(thisMonth, 10)
	}()
	go func() {
		defer wg.Done()
		res.costByVendor, _ = s.repo.GetCostByVendor(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costByService, _ = s.repo.GetCostByService(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costByRegion, _ = s.repo.GetCostByRegion(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costTrend, _ = s.repo.GetDailyCostTrend(now.AddDate(0, -30, 0), now)
	}()

	wg.Wait()

	if baseCurrency == "" {
		baseCurrency = "CNY"
	}
	currencyFromUSD := func(v float64) float64 {
		if baseCurrency == "CNY" {
			return v * 7.2
		}
		return v
	}

	var changePercent float64
	if res.lastCost > 0 {
		changePercent = ((res.currentCost - res.lastCost) / res.lastCost) * 100
	}

	_, monthEnd := getMonthRange(now)
	daysInMonth := monthEnd.Day()
	forecastCost := res.currentCost
	if now.Day() > 0 {
		forecastCost = res.currentCost / float64(now.Day()) * float64(daysInMonth)
	}

	topResourcesResult := make([]TopResource, 0)
	for _, m := range res.topResources {
		tr := TopResource{
			ResourceID:   getString(m, "resource_id"),
			ResourceName: getString(m, "resource_name"),
			ServiceCode:  getString(m, "service_code"),
			Cost:         getFloat(m, "cost"),
			Vendor:       getString(m, "vendor"),
			Currency:     getString(m, "currency"),
		}
		topResourcesResult = append(topResourcesResult, tr)
	}

	costTrendResult := make([]DailyCost, 0)
	for _, m := range res.costTrend {
		dc := DailyCost{
			Date: getString(m, "date"),
			Cost: currencyFromUSD(getFloat(m, "cost")),
		}
		costTrendResult = append(costTrendResult, dc)
	}

	vendorCost := make(map[string]VendorCostEntry, len(res.costByVendor))
	for v, vc := range res.costByVendor {
		vendorCost[v] = VendorCostEntry{Cost: vc.Cost, Currency: vc.Currency}
	}

	// 转换聚合数据到目标基币
	costByService := make(map[string]float64, len(res.costByService))
	for k, v := range res.costByService {
		costByService[k] = currencyFromUSD(v)
	}
	costByRegion := make(map[string]float64, len(res.costByRegion))
	for k, v := range res.costByRegion {
		costByRegion[k] = currencyFromUSD(v)
	}

	return &DashboardData{
		CurrentMonthCost: currencyFromUSD(res.currentCost),
		LastMonthCost:    currencyFromUSD(res.lastCost),
		ForecastCost:     currencyFromUSD(forecastCost),
		ChangePercent:    changePercent,
		BaseCurrency:     baseCurrency,
		TopResources:     topResourcesResult,
		CostByVendor:     vendorCost,
		CostByService:    costByService,
		CostByRegion:     costByRegion,
		CostTrend:        costTrendResult,
	}, nil
}

type Recommendation struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Savings     float64             `json:"savings"`
	Resources   int                 `json:"resources"`
	Priority    string              `json:"priority"`
	Items       []RecommendationItem `json:"items,omitempty"`
}

type RecommendationItem struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	Vendor       string  `json:"vendor"`
	Cost         float64 `json:"cost"`
	Region       string  `json:"region"`
	Currency     string  `json:"currency"`
}

const maxRecommendationItems = 20

func truncateRecommendationItems(items []RecommendationItem) []RecommendationItem {
	if len(items) <= maxRecommendationItems {
		return items
	}
	return items[:maxRecommendationItems]
}

// GetRecommendations 获取优化建议
func (s *BillService) GetRecommendations() ([]Recommendation, error) {
	recommendations := []Recommendation{}

	var idleResources []repository.IdleResource
	var largeResources []repository.IdleResource
	var idleErr, largeErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		idleResources, idleErr = s.repo.GetIdleResources()
	}()
	go func() {
		defer wg.Done()
		largeResources, largeErr = s.repo.GetLargeResources()
	}()
	wg.Wait()

	_ = idleErr
	_ = largeErr

	// 1. 闲置资源
	if len(idleResources) > 0 {
		idleCost := 0.0
		items := make([]RecommendationItem, 0, len(idleResources))
		for _, r := range idleResources {
			idleCost += r.Cost
			items = append(items, RecommendationItem{
				ResourceID:   r.ResourceID,
				ResourceName: r.ResourceName,
				Vendor:       r.Vendor,
				Cost:         r.Cost,
				Region:       r.Region,
				Currency:     r.Currency,
			})
		}
		recommendations = append(recommendations, Recommendation{
			ID:          "idle_resources",
			Type:        "idle",
			Title:       "闲置资源",
			Description: "检测到未使用的资源，可考虑释放以节省成本",
			Savings:     idleCost,
			Resources:   len(idleResources),
			Priority:    "high",
			Items:       truncateRecommendationItems(items),
		})
	}

	// 2. 大规格资源
	if len(largeResources) > 0 {
		largeCost := 0.0
		items := make([]RecommendationItem, 0, len(largeResources))
		for _, r := range largeResources {
			largeCost += r.Cost
			items = append(items, RecommendationItem{
				ResourceID:   r.ResourceID,
				ResourceName: r.ResourceName,
				Vendor:       r.Vendor,
				Cost:         r.Cost,
				Region:       r.Region,
				Currency:     r.Currency,
			})
		}
		recommendations = append(recommendations, Recommendation{
			ID:          "rightsizing",
			Type:        "rightsizing",
			Title:       "大规格资源",
			Description: "检测到大规格资源（月均费用 > 500），建议评估是否可降配",
			Savings:     largeCost * 0.3,
			Resources:   len(largeResources),
			Priority:    "medium",
			Items:       truncateRecommendationItems(items),
		})
	}

	return recommendations, nil
}
