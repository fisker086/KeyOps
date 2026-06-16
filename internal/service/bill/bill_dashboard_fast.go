package bill

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/repository"
)

// DashboardSummaryData 总览 KPI（轻量接口，优先返回）
type DashboardSummaryData struct {
	CurrentMonthCost          float64 `json:"current_month_cost"`
	CurrentMonthEffectiveCost float64 `json:"current_month_effective_cost"`
	CurrentMonthListCost      float64 `json:"current_month_list_cost"`
	CurrentMonthExcludedCost  float64 `json:"current_month_excluded_cost"`
	CurrentMonthMarketplace   float64 `json:"current_month_marketplace_cost"`
	LastMonthCost             float64 `json:"last_month_cost"`
	LastMonthEffectiveCost    float64 `json:"last_month_effective_cost"`
	LastMonthListCost         float64 `json:"last_month_list_cost"`
	LastMonthExcludedCost     float64 `json:"last_month_excluded_cost"`
	LastMonthMarketplace      float64 `json:"last_month_marketplace_cost"`
	ForecastCost              float64 `json:"forecast_cost"`
	ForecastEffectiveCost     float64 `json:"forecast_effective_cost"`
	ForecastBasisDays         int     `json:"forecast_basis_days"`
	ChangePercent             float64 `json:"change_percent"`
	EffectiveChangePercent    float64 `json:"effective_change_percent"`
	DiscountRate              float64 `json:"discount_rate"`
	EffectiveDiscountRate     float64 `json:"effective_discount_rate"`
	// YCloud Excel「本月费用总额」口径（与摊销分摊价 effective 不同，见 monthly_bill_* 字段）
	LastMonthGrossTotal       float64 `json:"last_month_gross_total"`
	LastMonthServiceDiscount  float64 `json:"last_month_service_discount"`
	LastMonthNetServiceCost   float64 `json:"last_month_net_service_cost"`
	LastMonthSupportBillable  float64 `json:"last_month_support_billable"`
	LastMonthBillTotal        float64 `json:"last_month_bill_total"`
	CurrentMonthBillTotal     float64 `json:"current_month_bill_total"`
	BaseCurrency              string  `json:"base_currency"`
	PeriodStart               string  `json:"period_start"`
	PeriodEnd                 string  `json:"period_end"`
	IsPartialMonth            bool    `json:"is_partial_month"`
	DaysInMonth               int     `json:"days_in_month"`
	DaysElapsed               int     `json:"days_elapsed"`
	IsNewAccount              bool    `json:"is_new_account"`
}

// DashboardChartsData 总览图表维度（读预聚合表）
type DashboardChartsData struct {
	BaseCurrency    string                     `json:"base_currency"`
	CostByVendor    map[string]VendorCostEntry `json:"cost_by_vendor"`
	CostByService   map[string]float64         `json:"cost_by_service"`
	CostByRegion    map[string]float64         `json:"cost_by_region"`
	TopServiceName  string                     `json:"top_service_name"`
	TopServiceShare float64                    `json:"top_service_share"`
}

// DashboardTrendData 费用走势（独立加载）
type DashboardTrendData struct {
	BaseCurrency string      `json:"base_currency"`
	CostTrend    []DailyCost `json:"cost_trend"`
}

// DashboardFullData 总览全量数据（合并 4 个接口，消除多次 HTTP 往返）
type DashboardFullData struct {
	Summary      *DashboardSummaryData `json:"summary"`
	Charts       *DashboardChartsData  `json:"charts"`
	Trend        *DashboardTrendData   `json:"trend"`
	TopResources []TopResource         `json:"top_resources"`
}

var (
	aggregateRebuildMu sync.Mutex
	aggregateRebuildCh = make(map[string]chan struct{})
)

// ensureDashboardAggregates 确保当月预聚合表已构建。
// 第一个请求者同步执行重建，后续请求者等待前一个完成，避免全部回退到慢查询。
func (s *BillService) ensureDashboardAggregates(cycle string) {
	has, err := s.repo.HasDashboardAggregates(cycle)
	if err != nil || has {
		return
	}
	aggregateRebuildMu.Lock()
	if ch, ok := aggregateRebuildCh[cycle]; ok {
		aggregateRebuildMu.Unlock()
		<-ch
		return
	}
	ch := make(chan struct{})
	aggregateRebuildCh[cycle] = ch
	aggregateRebuildMu.Unlock()

	defer func() {
		s.repo.RebuildDashboardAggregates(cycle)
		aggregateRebuildMu.Lock()
		delete(aggregateRebuildCh, cycle)
		close(ch)
		aggregateRebuildMu.Unlock()
	}()
}

// GetDashboardSummary 仅 KPI / 折扣 / 预测（读 bill_daily_cost 预聚合）
func (s *BillService) GetDashboardSummary(baseCurrency string) (*DashboardSummaryData, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	if cached, ok := getCachedSummary(baseCurrency); ok {
		return cached, nil
	}

	periodCosts, err := s.resolveDashboardPeriodCosts(time.Now())
	if err != nil {
		return nil, err
	}

	currentCost := s.currencyFromUSD(baseCurrency, periodCosts.currentEffective)
	currentEff := s.currencyFromUSD(baseCurrency, periodCosts.currentEff)
	currentList := s.currencyFromUSD(baseCurrency, periodCosts.currentList)
	lastCost := s.currencyFromUSD(baseCurrency, periodCosts.lastEffective)
	lastEff := s.currencyFromUSD(baseCurrency, periodCosts.lastEff)
	lastList := s.currencyFromUSD(baseCurrency, periodCosts.lastList)

	var changePercent float64
	if periodCosts.lastSameEffective > 0 {
		lastSame := s.currencyFromUSD(baseCurrency, periodCosts.lastSameEffective)
		changePercent = ((currentCost - lastSame) / lastSame) * 100
	}

	var effectiveChangePercent float64
	if periodCosts.lastSameEff > 0 {
		lastSameEff := s.currencyFromUSD(baseCurrency, periodCosts.lastSameEff)
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

	now := time.Now()
	thisCycle := now.Format("2006-01")
	lastCycle := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0).Format("2006-01")
	curExcluded, curMarketplace, _ := s.repo.GetAuxiliaryCostsByCycle(thisCycle)
	lastExcluded, lastMarketplace, _ := s.repo.GetAuxiliaryCostsByCycle(lastCycle)
	lastYCloud := s.buildYCloudMonthlyBill(lastCycle, lastExcluded, baseCurrency)
	curYCloud := s.buildYCloudMonthlyBill(thisCycle, curExcluded, baseCurrency)

	out := &DashboardSummaryData{
		CurrentMonthCost:          currentCost,
		CurrentMonthEffectiveCost: currentEff,
		CurrentMonthListCost:      currentList,
		CurrentMonthExcludedCost:  s.currencyFromUSD(baseCurrency, curExcluded),
		CurrentMonthMarketplace:   s.currencyFromUSD(baseCurrency, curMarketplace),
		LastMonthCost:             lastCost,
		LastMonthEffectiveCost:    lastEff,
		LastMonthListCost:         lastList,
		LastMonthExcludedCost:     s.currencyFromUSD(baseCurrency, lastExcluded),
		LastMonthMarketplace:      s.currencyFromUSD(baseCurrency, lastMarketplace),
		LastMonthGrossTotal:       lastYCloud.GrossTotal,
		LastMonthServiceDiscount:  lastYCloud.ServiceDiscount,
		LastMonthNetServiceCost:   lastYCloud.NetServiceCost,
		LastMonthSupportBillable:  lastYCloud.SupportBillable,
		LastMonthBillTotal:        lastYCloud.MonthlyBillTotal,
		CurrentMonthBillTotal:     curYCloud.MonthlyBillTotal,
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
		DaysInMonth:               periodCosts.daysInMonth,
		DaysElapsed:               periodCosts.daysElapsed,
		IsNewAccount:              periodCosts.lastEffective == 0 && periodCosts.currentEffective > 0,
	}
	setCachedSummary(baseCurrency, out)
	return out, nil
}

func (s *BillService) buildYCloudMonthlyBill(cycle string, excludedServiceCost float64, baseCurrency string) YCloudMonthlyBill {
	gross, err := s.repo.GetMonthlyGrossTotalByCycle(cycle)
	if err != nil {
		log.Printf("[Dashboard] ycloud monthly bill cycle=%s err=%v", cycle, err)
	}
	bill := buildYCloudMonthlyBill(gross, excludedServiceCost, defaultEDPDiscountRate)
	return YCloudMonthlyBill{
		GrossTotal:       s.currencyFromUSD(baseCurrency, bill.GrossTotal),
		ServiceDiscount:  s.currencyFromUSD(baseCurrency, bill.ServiceDiscount),
		NetServiceCost:   s.currencyFromUSD(baseCurrency, bill.NetServiceCost),
		SupportBillable:  s.currencyFromUSD(baseCurrency, bill.SupportBillable),
		MonthlyBillTotal: s.currencyFromUSD(baseCurrency, bill.MonthlyBillTotal),
		EDPDiscountRate:  bill.EDPDiscountRate,
	}
}

// GetDashboardCharts 厂商/服务/区域分布（预聚合表，毫秒级）
func (s *BillService) GetDashboardCharts(baseCurrency string) (*DashboardChartsData, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	if cached, ok := getCachedCharts(baseCurrency); ok {
		return cached, nil
	}

	cycle := s.resolveDashboardChartsCycle(time.Now())
	s.ensureDashboardAggregates(cycle)
	has, _ := s.repo.HasDashboardAggregates(cycle)
	var costByVendor map[string]VendorCostEntry
	var costByService map[string]float64
	var costByRegion map[string]float64

	if has {
		vRows, _ := s.repo.ListDashboardAggregates(cycle, repository.AggTypeVendor, 0)
		sRows, _ := s.repo.ListDashboardAggregates(cycle, repository.AggTypeService, 0)
		rRows, _ := s.repo.ListDashboardAggregates(cycle, repository.AggTypeRegion, 0)

		costByVendor = make(map[string]VendorCostEntry, len(vRows))
		for _, r := range vRows {
			costByVendor[r.AggKey] = VendorCostEntry{
				Cost:          s.currencyFromUSD(baseCurrency, r.CostUSD),
				EffectiveCost: s.currencyFromUSD(baseCurrency, r.EffectiveCostUSD),
				Currency:      baseCurrency,
			}
		}
		costByService = make(map[string]float64, len(sRows))
		for _, r := range sRows {
			costByService[r.AggKey] = s.currencyFromUSD(baseCurrency, r.CostUSD)
		}
		costByRegion = make(map[string]float64, len(rRows))
		for _, r := range rRows {
			costByRegion[r.AggKey] = s.currencyFromUSD(baseCurrency, r.CostUSD)
		}
	} else {
		var err error
		costByVendor, err = s.loadVendorCosts(cycle, baseCurrency)
		if err != nil {
			return nil, err
		}
		costByService, _ = s.repo.GetCostByService(cycle)
		for k, v := range costByService {
			costByService[k] = s.currencyFromUSD(baseCurrency, v)
		}
		costByRegion, _ = s.repo.GetCostByRegion(cycle)
		for k, v := range costByRegion {
			costByRegion[k] = s.currencyFromUSD(baseCurrency, v)
		}
	}

	topServiceName := ""
	topServiceShare := 0.0
	if len(costByService) > 0 {
		totalSvc, maxSvc := 0.0, 0.0
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

	out := &DashboardChartsData{
		BaseCurrency:    baseCurrency,
		CostByVendor:    costByVendor,
		CostByService:   costByService,
		CostByRegion:    costByRegion,
		TopServiceName:  topServiceName,
		TopServiceShare: topServiceShare,
	}
	setCachedCharts(baseCurrency, out)
	return out, nil
}

func (s *BillService) loadVendorCosts(cycle, baseCurrency string) (map[string]VendorCostEntry, error) {
	raw, err := s.repo.GetCostByVendor(cycle)
	if err != nil {
		return nil, err
	}
	out := make(map[string]VendorCostEntry, len(raw))
	for v, vc := range raw {
		usdCost := s.vendorNativeToUSD(v, vc.Cost)
		usdEff := s.vendorNativeToUSD(v, vc.EffectiveCost)
		out[v] = VendorCostEntry{Cost: s.currencyFromUSD(baseCurrency, usdCost), EffectiveCost: s.currencyFromUSD(baseCurrency, usdEff), Currency: baseCurrency}
	}
	return out, nil
}

// GetDashboardFull 合并总览全量数据（单次 HTTP 请求，替代 4 次独立调用）
// 内部并行执行 summary、charts、trend、top-resources，减少网络往返和序列化开销
func (s *BillService) GetDashboardFull(baseCurrency string) (*DashboardFullData, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	var (
		summary   *DashboardSummaryData
		charts    *DashboardChartsData
		trend     *DashboardTrendData
		resources []TopResource
	)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		var err error
		summary, err = s.GetDashboardSummary(baseCurrency)
		if err != nil {
			log.Printf("[BillDashboard] GetDashboardFull summary error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		charts, err = s.GetDashboardCharts(baseCurrency)
		if err != nil {
			log.Printf("[BillDashboard] GetDashboardFull charts error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		trend, err = s.GetDashboardTrend(baseCurrency)
		if err != nil {
			log.Printf("[BillDashboard] GetDashboardFull trend error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		resources, err = s.GetDashboardTopResources(baseCurrency, 10)
		if err != nil {
			log.Printf("[BillDashboard] GetDashboardFull resources error: %v", err)
		}
	}()
	wg.Wait()
	return &DashboardFullData{
		Summary:      summary,
		Charts:       charts,
		Trend:        trend,
		TopResources: resources,
	}, nil
}

// GetDashboardTrend 近 30 天日趋势（从 bill_daily_cost 读取）
func (s *BillService) GetDashboardTrend(baseCurrency string) (*DashboardTrendData, error) {
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	if cached, ok := getCachedTrend(baseCurrency); ok {
		return cached, nil
	}

	trendEnd := time.Now()
	trendStart := dashboardTrendStartDay(trendEnd)

	rows, err := s.repo.GetDailyCostRange(trendStart, trendEnd)
	if err != nil {
		return nil, fmt.Errorf("get trend: %w", err)
	}

	type dailyAccum struct {
		Cost          float64
		EffectiveCost float64
	}
	dateMap := make(map[string]dailyAccum)
	for _, r := range rows {
		d := dateMap[r.Date]
		eff := r.EffectiveCost
		if eff <= 0 {
			eff = r.Cost
		}
		d.Cost += s.currencyFromUSD(baseCurrency, r.Cost)
		d.EffectiveCost += s.currencyFromUSD(baseCurrency, eff)
		dateMap[r.Date] = d
	}

	result := make([]DailyCost, 0, len(dateMap))
	day := trendStart
	for !day.After(trendEnd) {
		d := day.Format("2006-01-02")
		da := dateMap[d]
		result = append(result, DailyCost{Date: d, Cost: da.Cost, EffectiveCost: da.EffectiveCost})
		day = day.AddDate(0, 0, 1)
	}

	out := &DashboardTrendData{BaseCurrency: baseCurrency, CostTrend: result}
	setCachedTrend(baseCurrency, out)
	return out, nil
}
