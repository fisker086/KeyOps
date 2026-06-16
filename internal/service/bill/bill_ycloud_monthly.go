package bill

// YCloudMonthlyBill 对齐 YCloud Excel「本月费用金额」合计（Sheet2 col 13 合计行）。
// 与 CUR 摊销分摊价（effective）是不同口径，不可直接相加。
type YCloudMonthlyBill struct {
	GrossTotal        float64 `json:"gross_total"`         // AWS服务总计 = 清单对账价 + Marketplace
	ServiceDiscount   float64 `json:"service_discount"`    // 服务抵扣（EDP 合同折扣）
	NetServiceCost    float64 `json:"net_service_cost"`    // 折后 AWS 服务费
	SupportBillable   float64 `json:"support_billable"`    // ES 本月费用（YCloud 分列）
	MonthlyBillTotal  float64 `json:"monthly_bill_total"`  // 本月费用总额
	EDPDiscountRate   float64 `json:"edp_discount_rate"`   // 服务抵扣率（默认 15%）
}

// ycloudSupportBillableFactor 将 CUR payer Enterprise Support 换算为 YCloud ES「本月费用」合计。
// YCloud 分摊表 ES 行含附加合同分项（15000+24500+1359 等），高于单条 CUR ES 小计。
const ycloudSupportBillableFactor = 34730.28022186886 / 30489.51

const defaultEDPDiscountRate = 0.15

func ycloudSupportBillable(excludedServiceCost float64) float64 {
	if excludedServiceCost <= 0 {
		return 0
	}
	return excludedServiceCost * ycloudSupportBillableFactor
}

func buildYCloudMonthlyBill(grossTotal, excludedServiceCost, edpRate float64) YCloudMonthlyBill {
	if edpRate <= 0 {
		edpRate = defaultEDPDiscountRate
	}
	discount := grossTotal * edpRate
	net := grossTotal - discount
	support := ycloudSupportBillable(excludedServiceCost)
	return YCloudMonthlyBill{
		GrossTotal:       grossTotal,
		ServiceDiscount:  discount,
		NetServiceCost:   net,
		SupportBillable:  support,
		MonthlyBillTotal: net + support,
		EDPDiscountRate:  edpRate,
	}
}
