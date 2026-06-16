package bill

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/logger"
)

// dashboardTrendDays 成本总览「费用趋势」展示的天数（含当天）。
const dashboardTrendDays = 30

func dashboardTrendStartDay(anchor time.Time) time.Time {
	d := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location())
	return d.AddDate(0, 0, -(dashboardTrendDays - 1))
}

type VendorCostEntry struct {
	Cost          float64 `json:"cost"`
	EffectiveCost float64 `json:"effective_cost"`
	Currency      string  `json:"currency"`
}

type DashboardData struct {
	CurrentMonthCost          float64                    `json:"current_month_cost"`
	CurrentMonthEffectiveCost float64                    `json:"current_month_effective_cost"`
	CurrentMonthListCost      float64                    `json:"current_month_list_cost"`
	LastMonthCost             float64                    `json:"last_month_cost"`
	LastMonthEffectiveCost    float64                    `json:"last_month_effective_cost"`
	LastMonthListCost         float64                    `json:"last_month_list_cost"`
	ForecastCost              float64                    `json:"forecast_cost"`
	ForecastEffectiveCost     float64                    `json:"forecast_effective_cost"`
	ForecastBasisDays         int                        `json:"forecast_basis_days"`
	ChangePercent             float64                    `json:"change_percent"`
	EffectiveChangePercent    float64                    `json:"effective_change_percent"`
	DiscountRate              float64                    `json:"discount_rate"`
	EffectiveDiscountRate     float64                    `json:"effective_discount_rate"`
	BaseCurrency              string                     `json:"base_currency"`
	PeriodStart               string                     `json:"period_start"`
	PeriodEnd                 string                     `json:"period_end"`
	IsPartialMonth            bool                       `json:"is_partial_month"`
	TopResources              []TopResource              `json:"top_resources"`
	CostByVendor              map[string]VendorCostEntry `json:"cost_by_vendor"`
	CostByService             map[string]float64         `json:"cost_by_service"`
	CostByRegion              map[string]float64         `json:"cost_by_region"`
	CostTrend                 []DailyCost                `json:"cost_trend"`
	TopServiceShare           float64                    `json:"top_service_share"`
	TopServiceName            string                     `json:"top_service_name"`
	DaysInMonth               int                        `json:"days_in_month"`
	DaysElapsed               int                        `json:"days_elapsed"`
	IsNewAccount              bool                       `json:"is_new_account"`
}

type TopResource struct {
	ResourceID    string  `json:"resource_id"`
	ResourceName  string  `json:"resource_name"`
	ServiceCode   string  `json:"service_code"`
	Cost          float64 `json:"cost"`
	EffectiveCost float64 `json:"effective_cost"`
	Vendor        string  `json:"vendor"`
	Currency      string  `json:"currency"`
}

type DailyCost struct {
	Date          string  `json:"date"`
	Cost          float64 `json:"cost"`
	EffectiveCost float64 `json:"effective_cost"`
}

type dashboardPeriodCosts struct {
	currentEffective  float64
	currentEff        float64
	currentList       float64
	lastEffective     float64
	lastEff           float64
	lastList          float64
	lastSameEffective float64
	lastSameEff       float64
	periodStart       time.Time
	periodEnd         time.Time
	daysElapsed       int
	daysInMonth       int
	isPartialMonth    bool
}

// resolveDashboardChartsCycle 本月无账单时，图表/Top 资源回退到最近有数据的月份
func (s *BillService) resolveDashboardChartsCycle(now time.Time) string {
	thisCycle := now.Format("2006-01")
	if cost, err := s.repo.GetTotalCostByMonth(thisCycle); err == nil && cost > 0 {
		return thisCycle
	}
	if has, err := s.repo.HasDashboardAggregates(thisCycle); err == nil && has {
		return thisCycle
	}
	lastCycle := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0).Format("2006-01")
	if cost, err := s.repo.GetTotalCostByMonth(lastCycle); err == nil && cost > 0 {
		return lastCycle
	}
	return thisCycle
}

// resolveDashboardPeriodCosts 对齐 ~/bill：按用量日期区间汇总，非 bill_records.cycle 整月硬筛
func (s *BillService) resolveDashboardPeriodCosts(now time.Time) (dashboardPeriodCosts, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	anchor := today

	thisStart, _ := getMonthRange(anchor)
	thisEnd := anchor
	if thisEnd.Before(thisStart) {
		thisEnd = thisStart
	}

	lastAnchor := thisStart.AddDate(0, 0, -1)
	lastStart, lastEnd := getMonthRange(lastAnchor)

	daysElapsed := thisEnd.Day()
	_, monthEnd := getMonthRange(anchor)
	daysInMonth := monthEnd.Day()
	isPartial := thisEnd.Day() != daysInMonth

	out := dashboardPeriodCosts{
		periodStart:    thisStart,
		periodEnd:      thisEnd,
		daysElapsed:    daysElapsed,
		daysInMonth:    daysInMonth,
		isPartialMonth: isPartial,
	}

	rangeStart := thisStart
	if lastStart.Before(rangeStart) {
		rangeStart = lastStart
	}
	rows, err := s.repo.GetDailyCostRange(rangeStart, thisEnd)
	if err != nil {
		return out, fmt.Errorf("get daily cost range: %w", err)
	}

	thisStartStr := thisStart.Format("2006-01-02")
	thisEndStr := thisEnd.Format("2006-01-02")
	lastStartStr := lastStart.Format("2006-01-02")
	lastEndStr := lastEnd.Format("2006-01-02")

	var currentCost, currentEff, currentList, lastCost, lastEff, lastList float64
	var daysWithData int
	var hasLastData bool
	seenDates := make(map[string]bool)
	for _, r := range rows {
		eff := r.EffectiveCost
		if eff <= 0 {
			eff = r.Cost
		}
		// bill_daily_cost 存各 vendor 原生金额（AWS 为 USD）；展示层再换算
		usdCost := r.Cost
		usdEff := eff
		usdList := r.ListCost

		if r.Date >= thisStartStr && r.Date <= thisEndStr {
			currentCost += usdCost
			currentEff += usdEff
			currentList += usdList
			if !seenDates[r.Date] {
				seenDates[r.Date] = true
				daysWithData++
			}
		}
		if r.Date >= lastStartStr && r.Date <= lastEndStr {
			lastCost += usdCost
			lastEff += usdEff
			lastList += usdList
			hasLastData = true
		}
	}

	// bill_daily_cost 有数据 → 直接返回
	if daysWithData > 0 || currentCost > 0 || lastCost > 0 {
		if daysWithData > 0 {
			out.daysElapsed = daysWithData
		}
		out.currentEffective = currentCost
		out.currentEff = currentEff
		out.currentList = currentList
		out.lastEffective = lastCost
		out.lastEff = lastEff
		out.lastList = lastList

		lastSameEndStr := lastStart.AddDate(0, 0, out.daysElapsed-1).Format("2006-01-02")
		var lastSameCost, lastSameEff float64
		for _, r := range rows {
			if r.Date >= lastStartStr && r.Date <= lastSameEndStr {
				eff := r.EffectiveCost
				if eff <= 0 {
					eff = r.Cost
				}
				lastSameCost += r.Cost
				lastSameEff += eff
			}
		}
		out.lastSameEffective = lastSameCost
		out.lastSameEff = lastSameEff

		// bill_daily_cost 缺少上月数据时，从 bill_records 回退
		if !hasLastData {
			lastMonth := lastStart.Format("2006-01")
			last, _ := s.repo.GetTotalCostByMonth(lastMonth)
			out.lastEffective = last
			out.lastEff = last
			out.lastList = last
			out.lastSameEffective = last * float64(out.daysElapsed) / float64(lastEnd.Day())
			out.lastSameEff = last * float64(out.daysElapsed) / float64(lastEnd.Day())
		}

		return out, nil
	}

	// 降级：bill_daily_cost 无数据，回退到按月总计
	thisMonth := thisStart.Format("2006-01")
	lastMonth := lastStart.Format("2006-01")
	cur, _ := s.repo.GetTotalCostByMonth(thisMonth)
	last, _ := s.repo.GetTotalCostByMonth(lastMonth)
	out.currentEffective = cur
	out.currentEff = cur
	out.currentList = cur
	out.lastEffective = last
	out.lastEff = last
	out.lastList = last
	out.lastSameEffective = last * float64(out.daysElapsed) / float64(lastEnd.Day())
	out.lastSameEff = last * float64(out.daysElapsed) / float64(lastEnd.Day())
	return out, nil
}

// GetDashboardData 获取 Dashboard 数据（不含 Top 资源，见 GetDashboardTopResources）
// baseCurrency: "CNY" or "USD" — 聚合图表统一转为此币种显示
func (s *BillService) GetDashboardData(baseCurrency string) (*DashboardData, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	if cached, ok := getCachedDashboard(baseCurrency); ok {
		return cached, nil
	}

	now := time.Now()
	thisMonth := now.Format("2006-01")

	type queryResult struct {
		costByVendor  map[string]repository.VendorCost
		costByService map[string]float64
		costByRegion  map[string]float64
		dailyTrend    []DailyCost
	}

	var res queryResult
	var periodCosts dashboardPeriodCosts
	var periodErr error
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		periodCosts, periodErr = s.resolveDashboardPeriodCosts(now)
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
		start := dashboardTrendStartDay(now)
		rows, err := s.repo.GetDailyCostRange(start, now)
		if err != nil {
			return
		}
		type dailyRow struct {
			Date          string
			Cost          float64
			EffectiveCost float64
		}
		dateMap := make(map[string]dailyRow)
		for _, r := range rows {
			d := dateMap[r.Date]
			d.Date = r.Date
			eff := r.EffectiveCost
			if eff <= 0 {
				eff = r.Cost
			}
			d.Cost += r.Cost
			d.EffectiveCost += eff
			dateMap[r.Date] = d
		}
		trend := make([]DailyCost, 0)
		day := start
		for !day.After(now) {
			d := day.Format("2006-01-02")
			dr := dateMap[d]
			trend = append(trend, DailyCost{Date: d, Cost: dr.Cost, EffectiveCost: dr.EffectiveCost})
			day = day.AddDate(0, 0, 1)
		}
		res.dailyTrend = trend
	}()

	wg.Wait()
	if periodErr != nil {
		return nil, periodErr
	}

	currencyFromUSD := func(v float64) float64 {
		return s.currencyFromUSD(baseCurrency, v)
	}

	currentCost := currencyFromUSD(periodCosts.currentEffective)
	currentEff := currencyFromUSD(periodCosts.currentEff)
	currentList := currencyFromUSD(periodCosts.currentList)
	lastCost := currencyFromUSD(periodCosts.lastEffective)
	lastEff := currencyFromUSD(periodCosts.lastEff)
	lastList := currencyFromUSD(periodCosts.lastList)

	var changePercent float64
	if periodCosts.lastSameEffective > 0 {
		lastSame := currencyFromUSD(periodCosts.lastSameEffective)
		changePercent = ((currentCost - lastSame) / lastSame) * 100
	}

	var effectiveChangePercent float64
	if periodCosts.lastSameEff > 0 {
		lastSameEff := currencyFromUSD(periodCosts.lastSameEff)
		effectiveChangePercent = ((currentEff - lastSameEff) / lastSameEff) * 100
	}

	var discountRate float64
	if periodCosts.lastList > 0 {
		discountRate = 1 - periodCosts.lastEffective/periodCosts.lastList
		if discountRate < 0 {
			discountRate = 0
		}
	}

	var effectiveDiscountRate float64
	if periodCosts.lastList > 0 {
		effectiveDiscountRate = 1 - periodCosts.lastEff/periodCosts.lastList
		if effectiveDiscountRate < 0 {
			effectiveDiscountRate = 0
		}
	}

	forecastCost := currentCost
	if periodCosts.daysElapsed > 0 {
		forecastCost = currentCost / float64(periodCosts.daysElapsed) * float64(periodCosts.daysInMonth)
	}

	forecastEffective := currentEff
	if periodCosts.daysElapsed > 0 {
		forecastEffective = currentEff / float64(periodCosts.daysElapsed) * float64(periodCosts.daysInMonth)
	}

	// 趋势按日展示（近 30 天）；统一换算到基准币种
	costTrendResult := make([]DailyCost, 0, len(res.dailyTrend))
	for _, d := range res.dailyTrend {
		costTrendResult = append(costTrendResult, DailyCost{Date: d.Date, Cost: currencyFromUSD(d.Cost), EffectiveCost: currencyFromUSD(d.EffectiveCost)})
	}

	// 厂商成本统一换算到展示币种（库内 AWS 为 USD）
	vendorCost := make(map[string]VendorCostEntry, len(res.costByVendor))
	for v, vc := range res.costByVendor {
		usdCost := s.vendorNativeToUSD(v, vc.Cost)
		usdEff := s.vendorNativeToUSD(v, vc.EffectiveCost)
		vendorCost[v] = VendorCostEntry{Cost: currencyFromUSD(usdCost), EffectiveCost: currencyFromUSD(usdEff), Currency: baseCurrency}
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

	topServiceName := ""
	topServiceShare := 0.0
	if len(costByService) > 0 {
		totalSvc := 0.0
		maxSvc := 0.0
		for name, cost := range costByService {
			totalSvc += cost
			if cost > maxSvc {
				maxSvc = cost
				topServiceName = name
			}
		}
		if totalSvc > 0 {
			topServiceShare = (maxSvc / totalSvc) * 100
		}
	}

	out := &DashboardData{
		CurrentMonthCost:          currentCost,
		CurrentMonthEffectiveCost: currentEff,
		CurrentMonthListCost:      currentList,
		LastMonthCost:             lastCost,
		LastMonthEffectiveCost:    lastEff,
		LastMonthListCost:         lastList,
		ForecastCost:              forecastCost,
		ForecastEffectiveCost:     forecastEffective,
		ForecastBasisDays:         periodCosts.daysElapsed,
		ChangePercent:             math.Round(changePercent*10) / 10,
		EffectiveChangePercent:    math.Round(effectiveChangePercent*10) / 10,
		DiscountRate:              math.Round(discountRate*1000) / 1000,
		EffectiveDiscountRate:     math.Round(effectiveDiscountRate*1000) / 1000,
		BaseCurrency:              baseCurrency,
		PeriodStart:               periodCosts.periodStart.Format("2006-01-02"),
		PeriodEnd:                 periodCosts.periodEnd.Format("2006-01-02"),
		IsPartialMonth:            periodCosts.isPartialMonth,
		TopResources:              nil,
		CostByVendor:              vendorCost,
		CostByService:             costByService,
		CostByRegion:              costByRegion,
		CostTrend:                 costTrendResult,
		TopServiceName:            topServiceName,
		TopServiceShare:           math.Round(topServiceShare*100) / 100,
		DaysInMonth:               periodCosts.daysInMonth,
		DaysElapsed:               periodCosts.daysElapsed,
		IsNewAccount:              periodCosts.lastEffective == 0 && periodCosts.currentEffective > 0,
	}
	setCachedDashboard(baseCurrency, out)
	return out, nil
}

// GetDashboardTopResources 本月 Top 资源（独立接口，避免拖慢 Dashboard 主接口）
func (s *BillService) GetDashboardTopResources(baseCurrency string, limit int) ([]TopResource, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	if limit <= 0 {
		limit = 10
	}
	thisMonth := s.resolveDashboardChartsCycle(time.Now())
	if cached, ok := getCachedTopResources(thisMonth, baseCurrency); ok {
		return cached, nil
	}

	s.ensureDashboardAggregates(thisMonth)
	result := make([]TopResource, 0, limit)
	if has, _ := s.repo.HasDashboardAggregates(thisMonth); has {
		rows, err := s.repo.ListDashboardAggregates(thisMonth, repository.AggTypeInstance, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			result = append(result, TopResource{
				ResourceID:    r.AggKey,
				ResourceName:  r.ResourceName,
				ServiceCode:   r.SubKey,
				Cost:          s.currencyFromUSD(baseCurrency, r.CostUSD),
				EffectiveCost: s.currencyFromUSD(baseCurrency, r.EffectiveCostUSD),
				Vendor:        r.Vendor,
				Currency:      baseCurrency,
			})
		}
		setCachedTopResources(thisMonth, baseCurrency, result)
		return result, nil
	}

	rows, err := s.repo.GetTopResources(thisMonth, limit)
	if err != nil {
		return nil, err
	}
	for _, m := range rows {
		vendor := getString(m, "vendor")
		rawCost := getFloat(m, "cost")
		usdCost := s.vendorNativeToUSD(vendor, rawCost)
		result = append(result, TopResource{
			ResourceID:   getString(m, "resource_id"),
			ResourceName: getString(m, "resource_name"),
			ServiceCode:  getString(m, "service_code"),
			Cost:         s.currencyFromUSD(baseCurrency, usdCost),
			Vendor:       vendor,
			Currency:     baseCurrency,
		})
	}
	setCachedTopResources(thisMonth, baseCurrency, result)
	return result, nil
}

type Recommendation struct {
	ID          string               `json:"id"`
	Type        string               `json:"type"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Savings     float64              `json:"savings"`
	Resources   int                  `json:"resources"`
	Priority    string               `json:"priority"`
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
	if cached, ok := getCachedRecommendations(); ok {
		return cached, nil
	}
	leader, wait := acquireRecommendationsInflight()
	if !leader {
		<-wait
		if cached, ok := getCachedRecommendations(); ok {
			return cached, nil
		}
		return nil, fmt.Errorf("recommendations load failed")
	}

	recommendations := []Recommendation{}
	thisMonth := time.Now().Format("2006-01")

	var idleResources []repository.IdleResource
	var largeResources []repository.IdleResource
	var untaggedResources []repository.IdleResource
	var newRegions []repository.IdleResource
	var consolidationCandidates []repository.IdleResource
	var topService string
	var topShare float64
	var discountRate float64
	var idleErr, largeErr, untagErr, regionErr, consolErr, shareErr, discErr error

	var wg sync.WaitGroup
	wg.Add(7)
	go func() {
		defer wg.Done()
		idleResources, idleErr = s.repo.GetIdleResources()
	}()
	go func() {
		defer wg.Done()
		largeResources, largeErr = s.repo.GetLargeResources()
	}()
	go func() {
		defer wg.Done()
		untaggedResources, untagErr = s.repo.GetUntaggedResources(thisMonth, 20)
	}()
	go func() {
		defer wg.Done()
		newRegions, regionErr = s.repo.GetNewRegions(thisMonth)
	}()
	go func() {
		defer wg.Done()
		consolidationCandidates, consolErr = s.repo.GetConsolidationCandidates(thisMonth)
	}()
	go func() {
		defer wg.Done()
		topService, topShare, shareErr = s.repo.GetTopServiceShare(thisMonth)
	}()
	go func() {
		defer wg.Done()
		discountRate, discErr = s.repo.GetDailyDiscountRate(thisMonth)
	}()
	wg.Wait()

	var errs []error
	for _, e := range []error{idleErr, largeErr, untagErr, regionErr, consolErr, shareErr, discErr} {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) > 0 {
		logger.Warnf("[BillDashboard] %d recommendation queries failed, returning partial results: %v", len(errs), errs)
	}

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

	// 3. 成本集中度过高
	if topService != "" && topShare > 0.5 {
		recommendations = append(recommendations, Recommendation{
			ID:          "cost_concentration",
			Type:        "concentration",
			Title:       "成本集中度过高",
			Description: fmt.Sprintf("「%s」占比 %.0f%%，存在单点绑定风险，建议分散至多服务", topService, topShare*100),
			Savings:     0,
			Resources:   1,
			Priority:    "medium",
			Items:       nil,
		})
	}

	// 4. 无标签资源
	if len(untaggedResources) > 0 {
		untagCost := 0.0
		items := make([]RecommendationItem, 0, len(untaggedResources))
		for _, r := range untaggedResources {
			untagCost += r.Cost
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
			ID:          "untagged_resources",
			Type:        "tagging",
			Title:       "无标签资源",
			Description: fmt.Sprintf("检测到 %d 个资源无标签（费用总计 %.0f），无法按成本中心分摊", len(untaggedResources), untagCost),
			Savings:     untagCost,
			Resources:   len(untaggedResources),
			Priority:    "low",
			Items:       truncateRecommendationItems(items),
		})
	}

	// 5. 新厂商/新区域异常
	if len(newRegions) > 0 {
		regionCost := 0.0
		items := make([]RecommendationItem, 0, len(newRegions))
		for _, r := range newRegions {
			regionCost += r.Cost
			items = append(items, RecommendationItem{
				ResourceID:   r.ResourceID,
				ResourceName: r.ResourceName,
				Vendor:       r.Vendor,
				Cost:         r.Cost,
				Region:       r.Region,
				Currency:     r.Currency,
			})
		}
		priority := "medium"
		if regionCost > 1000 {
			priority = "high"
		}
		recommendations = append(recommendations, Recommendation{
			ID:          "new_regions",
			Type:        "anomaly",
			Title:       "新厂商/新区域异常",
			Description: fmt.Sprintf("上月未出现的新区域本月产生费用 %.0f，请确认是否为预期行为", regionCost),
			Savings:     regionCost,
			Resources:   len(newRegions),
			Priority:    priority,
			Items:       truncateRecommendationItems(items),
		})
	}

	// 6. 折扣率偏低
	if discountRate >= 0 && discountRate < 0.05 && discountRate > 0 {
		recommendations = append(recommendations, Recommendation{
			ID:          "low_discount",
			Type:        "savings",
			Title:       "折扣率偏低",
			Description: fmt.Sprintf("上月折扣率仅 %.1f%%，远低于标价。建议评估购买预留实例或节省计划以降低成本", discountRate*100),
			Savings:     0,
			Resources:   1,
			Priority:    "medium",
			Items:       nil,
		})
	}

	// 7. 相似规格合并
	if len(consolidationCandidates) > 0 {
		items := make([]RecommendationItem, 0, len(consolidationCandidates))
		for _, r := range consolidationCandidates {
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
			ID:          "consolidation",
			Type:        "rightsizing",
			Title:       "相似规格合并",
			Description: "检测到同 region 同规格的多个实例，建议评估能否合入大规格实例",
			Savings:     0,
			Resources:   len(consolidationCandidates),
			Priority:    "low",
			Items:       truncateRecommendationItems(items),
		})
	}

	setCachedRecommendations(recommendations)
	finishRecommendationsInflight(recommendations, nil)
	return recommendations, nil
}
