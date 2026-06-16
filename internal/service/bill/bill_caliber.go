package bill

import "strings"

// ycloudGlacierDeepArchiveAsMarketplace YCloud Excel 将指定 usage account 的 S3 Glacier Deep Archive 计入 marketplace（非 CUR billing_entity）。
var ycloudGlacierDeepArchiveAsMarketplace = map[string]bool{
	"689860182975": true, // -log ($)
}

// isYCloudMarketplaceCost 对齐 YCloud Excel「marketplace费用」列：CUR billing_entity=Marketplace，及 -log Glacier Deep Archive。
func isYCloudMarketplaceCost(usageAccountID, serviceType, billingEntity string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(billingEntity)), "marketplace") {
		return true
	}
	usage := strings.TrimSpace(usageAccountID)
	if !ycloudGlacierDeepArchiveAsMarketplace[usage] {
		return false
	}
	st := strings.ToLower(strings.TrimSpace(serviceType))
	return strings.Contains(st, "glacier deep archive")
}

// isExcludedFromServiceTotal 对齐 YCloud/财务 Excel「AWS 服务费用」小计：
// 付款账号（payer）上的 Enterprise Support 单独列示（ES support 合同行），不计入 AWS 服务费。
func isExcludedFromServiceTotal(payerAccountID, usageAccountID, serviceType, serviceCode string) bool {
	payer := strings.TrimSpace(payerAccountID)
	usage := strings.TrimSpace(usageAccountID)
	if payer == "" || usage == "" || usage != payer {
		return false
	}

	st := strings.ToLower(strings.TrimSpace(serviceType))
	sc := strings.ToLower(strings.TrimSpace(serviceCode))

	if strings.Contains(st, "support") && strings.Contains(st, "enterprise") {
		return true
	}
	return sc == "awssupportenterprise"
}
