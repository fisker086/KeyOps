package bill

import "strings"

func NormalizeCostType(costType string) string {
	switch strings.ToLower(strings.TrimSpace(costType)) {
	case "effective", "amortized", "amortised":
		return "effective"
	default:
		return "unblended"
	}
}

func IsEffectiveCostType(costType string) bool {
	return NormalizeCostType(costType) == "effective"
}
