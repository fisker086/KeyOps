package bill

// CUR field name constants — single source of truth for all field name lookups.
// Supports both CUR 2.0 (path format: lineItem/Foo) and classic underscore format.

// UnblendedCostFieldKeys 清单对账价（consume_amount）：UnblendedCost，每行在 AWS 账单上的标价
var UnblendedCostFieldKeys = []string{
	"Line_item_unblended_cost",
	"lineItem/UnblendedCost",
}

// AmortizedCostFieldKeys 摊销分摊价（effective_cost）的兜底字段，当 RI/SP 字段不存在时用 amortized cost
var AmortizedCostFieldKeys = []string{
	"Line_item_blended_cost",
	"lineItem/BlendedCost",
	"lineItem/NetAmortizedCost",
	"lineItem/AmortizedCost",
}

var SavingsPlanRateKeys = []string{
	"savings_plan_savings_plan_rate",
	"savingsPlan/SavingsPlanRate",
}

var UsageAmountKeys = []string{
	"lineItem/UsageAmount",
	"Line_item_usage_amount",
	"usage_amount",
}

// General field lookup groups
var ResourceIDKeys = []string{
	"lineItem/ResourceId",
	"Line_item_resource_id",
	"resourceId",
	"ResourceId",
	"resource_id",
}

var ProductCodeKeys = []string{
	"lineItem/ProductCode",
	"Line_item_product_code",
	"product/ProductCode",
	"ProductCode",
	"product_code",
}

var ServiceNameKeys = []string{
	"ProductName",
	"product_name",
	"product/servicename",
	"Product_servicename",
	"product/servicecode",
	"product/ServiceName",
	"service_name",
}

var ProductFamilyKeys = []string{
	"product/productFamily",
	"Product_product_family",
	"product/ProductFamily",
	"ProductDetail",
	"product_detail",
	"resource_type",
}

var RegionKeys = []string{
	"product/region",
	"Product_region",
	"product/Region",
	"lineItem/AvailabilityZone",
	"Line_item_availability_zone",
	"availabilityZone",
	"region",
}

var InstanceTypeKeys = []string{
	"product/instanceType",
	"Product_instance_type",
	"product/InstanceType",
	"instance_type",
}

var LineItemTypeKeys = []string{
	"lineItem/LineItemType",
	"Line_item_line_item_type",
	"lineItemType",
}

var UsageTypeKeys = []string{
	"lineItem/UsageType",
	"Line_item_usage_type",
	"UsageType",
}

var OperationKeys = []string{
	"lineItem/Operation",
	"Line_item_operation",
	"Operation",
}

var CurrencyKeys = []string{
	"lineItem/CurrencyCode",
	"Line_item_currency_code",
	"currency",
}

var AccountIDKeys = []string{
	"lineItem/UsageAccountId",
	"Line_item_usage_account_id",
	"bill/PayerAccountId",
}

var ResourceNameKeys = []string{
	"resourceTags/user:Name",
	"Resource_tags_user_name",
	"resourceTags/user:name",
	"resource_name",
	"NickName",
	"nick_name",
	"InstanceName",
	"instanceName",
	"HostName",
	"hostname",
	"ItemName",
	"item_name",
}

var AZKeys = []string{
	"lineItem/AvailabilityZone",
	"Line_item_availability_zone",
	"availabilityZone",
}

// BillingEntityKeys 用于识别 Marketplace 费用
var BillingEntityKeys = []string{
	"bill/BillingEntity",
	"Bill_billing_entity",
	"bill/InvoicingEntity",
	"Bill_invoicing_entity",
}

// Individual field name constants (for non-lookup direct access)
const (
	FieldUsageStartDate   = "lineItem/UsageStartDate"
	FieldBlendedCostExpr  = "lineItem/BlendedCost"
	FieldProductCodeMongo = "lineItem/ProductCode"
	FieldProductRegion    = "product/region"
)
