package bill

import (
	"time"

	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/logger"
)

func (s *BillService) GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword, costType, baseCurrency string) (*repository.BreakdownResult, error) {
	costType = NormalizeCostType(costType)
	baseCurrency = NormalizeDisplayCurrency(baseCurrency)
	t0 := time.Now()
	result, err := s.repo.GetExpensesBreakdown(startDate, endDate, granularity, groupBy, vendor, serviceCode, keyword, costType)
	if err != nil {
		logger.Warnf("[BillBreakdown] failed costType=%s groupBy=%s range=%s..%s elapsed=%s err=%v",
			costType, groupBy, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), time.Since(t0).Round(time.Millisecond), err)
		return nil, err
	}
	days := 0
	if result != nil {
		days = len(result.Breakdown)
		result.CostType = costType
	}
	logger.Infof("[BillBreakdown] ok costType=%s groupBy=%s range=%s..%s days=%d dims=%d elapsed=%s",
		costType, groupBy, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"),
		days, len(result.Totals), time.Since(t0).Round(time.Millisecond))

	if result != nil {
		for date, dims := range result.Breakdown {
			for group, cost := range dims {
				result.Breakdown[date][group] = s.currencyFromUSD(baseCurrency, cost)
			}
		}
		for group, cost := range result.Totals {
			result.Totals[group] = s.currencyFromUSD(baseCurrency, cost)
		}
	}
	if result != nil {
		excluded, marketplace, auxErr := s.repo.GetAuxiliaryCostsByDateRange(startDate, endDate)
		if auxErr == nil {
			result.ExcludedCost = s.currencyFromUSD(baseCurrency, excluded)
			result.MarketplaceCost = s.currencyFromUSD(baseCurrency, marketplace)
		}
	}
	return result, nil
}
