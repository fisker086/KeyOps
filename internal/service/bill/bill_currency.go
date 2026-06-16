package bill

import (
	"strings"

	"github.com/fisker086/keyops/pkg/config"
)

// NormalizeDisplayCurrency 统一展示币种（API 入参大小写容错）。
func NormalizeDisplayCurrency(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD":
		return "USD"
	default:
		return "CNY"
	}
}

func (s *BillService) effectiveUSDToCNYRate() float64 {
	return config.EffectiveUSDToCNYRate(s.usdToCNYRate)
}

// vendorNativeToUSD 将各云厂商原生金额统一为 USD（库内 AWS 为 USD，国内云为 CNY）。
func (s *BillService) vendorNativeToUSD(vendor string, amount float64) float64 {
	if amount == 0 {
		return 0
	}
	if vendor == "aws" {
		return amount
	}
	return amount / s.effectiveUSDToCNYRate()
}

// usdToDisplayCurrency 将 USD 基准金额换算为界面展示币种。
// USD → 原样；CNY → × billing.usd_to_cny_rate。
func (s *BillService) usdToDisplayCurrency(displayCurrency string, amountUSD float64) float64 {
	if amountUSD == 0 {
		return 0
	}
	if NormalizeDisplayCurrency(displayCurrency) == "CNY" {
		return amountUSD * s.effectiveUSDToCNYRate()
	}
	return amountUSD
}

// currencyFromUSD 将库内 USD 基准金额换算为展示币种。
func (s *BillService) currencyFromUSD(displayCurrency string, amountUSD float64) float64 {
	return s.usdToDisplayCurrency(displayCurrency, amountUSD)
}
