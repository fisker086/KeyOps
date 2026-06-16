package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// BaseModel 基础模型，包含公共字段
type BaseModel struct {
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// BillSummary 月度汇总账单
type BillSummary struct {
	ID            uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor        string          `gorm:"column:vendor; type:varchar(50); uniqueIndex:idx_vendor_cycle" json:"vendor" binding:"required"` // 云厂商，tencent、huawei-langgemap、huawei-bjlg
	Cycle         string          `gorm:"column:cycle; type:varchar(10); uniqueIndex:idx_vendor_cycle" json:"cycle" binding:"required"`   // 账单月份，格式：2024-01
	ConsumeAmount decimal.Decimal `gorm:"column:consume_amount; type:decimal(25,15)" json:"consume_amount"`                               // 费用总额
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillSummary) TableName() string {
	return "bill_summary"
}

// BillSummaryDetail 月度汇总账单详情
type BillSummaryDetail struct {
	ID            uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	ResourceType  string          `gorm:"column:resource_type; type:text" json:"resource_type"`             // 资源类型
	ResourceCode  string          `gorm:"column:resource_code; type:text" json:"resource_code"`             // 资源类型代码
	ServiceType   string          `gorm:"column:service_type; type:text" json:"service_type,omitempty"`     // 服务类型，腾讯云没有此字段
	ServiceCode   string          `gorm:"column:service_code; type:text" json:"service_code,omitempty"`     // 服务类型代码，腾讯云没有此字段
	ConsumeAmount decimal.Decimal `gorm:"column:consume_amount; type:decimal(25,15)" json:"consume_amount"` // 费用总额
	SummaryID     uint            `gorm:"column:summary_id; type:uint" json:"summary_id"`                   // 关联的summary表ID
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillSummaryDetail) TableName() string {
	return "bill_summary_detail"
}

// BillRecord 账单消费记录
type BillRecord struct {
	ID              uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor          string          `gorm:"column:vendor; type:varchar(50);index:idx_br_cycle_vendor,priority:2" json:"vendor" binding:"required"`                                                                                                                                           // 云厂商
	Cycle           string          `gorm:"column:cycle; type:varchar(10);index:idx_br_cycle_vendor,priority:1;index:idx_br_cycle_svc,priority:1;index:idx_br_cycle_region,priority:1;index:idx_br_cycle_acc,priority:1;index:idx_br_cycle_inst,priority:1" json:"cycle" binding:"required"` // 账单月份
	InstanceID      string          `gorm:"column:instance_id; type:varchar(200);index:idx_br_cycle_inst,priority:2" json:"instance_id"`                                                                                                                                                     // 资源ID
	ResourceName    string          `gorm:"column:resource_name; type:text" json:"resource_name"`                                                                                                                                                                                            // 资源名称
	SpecDesc        string          `gorm:"column:spec_desc; type:text" json:"spec_desc"`                                                                                                                                                                                                    // 资源配置
	ConsumeAmount   decimal.Decimal `gorm:"column:consume_amount; type:decimal(25,15)" json:"consume_amount"`                                                                                                                                                                                // 清单对账价（UnblendedCost），每行在账单上的标价，不含 Marketplace
	EffectiveCost   decimal.Decimal `gorm:"column:effective_cost; type:decimal(25,15)" json:"effective_cost,omitempty"`                                                                                                                                                                      // 摊销分摊价（RI/SP 折扣后的实际成本）
	ListCost        decimal.Decimal `gorm:"column:list_cost;type:decimal(25,6)" json:"list_cost,omitempty"`                                                                                                                                                                                  // 目录价（折前标价），用于计算节省
	MarketplaceCost      decimal.Decimal `gorm:"column:marketplace_cost; type:decimal(25,15)" json:"marketplace_cost,omitempty"`                                                                                                                                                                  // Marketplace 第三方费用（单独列示）
	ExcludedServiceCost  decimal.Decimal `gorm:"column:excluded_service_cost; type:decimal(25,15)" json:"excluded_service_cost,omitempty"`                                                                                                                                                      // 不计入 AWS 服务费小计（付款账号 Support/Shield）
	UsageDate       string          `gorm:"column:usage_date;type:varchar(10);index:idx_br_usage_date" json:"usage_date,omitempty"`
	ResourceType    string          `gorm:"column:resource_type; type:text" json:"resource_type"`                                                  // 资源类型
	ResourceCode    string          `gorm:"column:resource_code; type:text" json:"resource_code"`                                                  // 资源类型代码
	ServiceType     string          `gorm:"column:service_type; type:text" json:"service_type,omitempty"`                                          // 服务类型
	ServiceCode     string          `gorm:"column:service_code; type:varchar(50);index:idx_br_cycle_svc,priority:2" json:"service_code,omitempty"` // 服务代码
	Region          string          `gorm:"column:region; type:varchar(50);index:idx_br_cycle_region,priority:2" json:"region,omitempty"`          // 区域
	AccountID       string          `gorm:"column:account_id; type:varchar(100)" json:"account_id,omitempty"`                                      // 云账户ID
	CloudAccountID  uint            `gorm:"column:cloud_account_id;index:idx_br_cycle_acc,priority:2" json:"cloud_account_id,omitempty"`           // 系统云账户ID
	Tags            string          `gorm:"column:tags; type:text" json:"tags,omitempty"`                                                          // 标签(JSON格式)
	Extra           string          `gorm:"column:extra; type:text" json:"extra"`                                                                  // 扩展字段
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillRecord) TableName() string {
	return "bill_records"
}

// BillDashboardAggregate 账单总览预聚合（同步后写入，避免总览页在线扫 bill_records）
type BillDashboardAggregate struct {
	ID               uint    `gorm:"column:id;primaryKey;autoIncrement" json:"id,omitempty"`
	Cycle            string  `gorm:"column:cycle;type:varchar(10);index:idx_bda_cycle_type,priority:1;not null" json:"cycle"`
	AggType          string  `gorm:"column:agg_type;type:varchar(20);index:idx_bda_cycle_type,priority:2;not null" json:"agg_type"`
	AggKey           string  `gorm:"column:agg_key;type:varchar(200);not null" json:"agg_key"`
	SubKey           string  `gorm:"column:sub_key;type:varchar(200)" json:"sub_key,omitempty"`
	Vendor           string  `gorm:"column:vendor;type:varchar(50)" json:"vendor,omitempty"`
	Currency         string  `gorm:"column:currency;type:varchar(10)" json:"currency,omitempty"`
	CostUSD          float64 `gorm:"column:cost_usd;type:decimal(25,6);not null;default:0" json:"cost_usd"`
	EffectiveCostUSD float64 `gorm:"column:effective_cost_usd;type:decimal(25,6);not null;default:0" json:"effective_cost_usd"`
	ResourceName     string  `gorm:"column:resource_name;type:varchar(200)" json:"resource_name,omitempty"`
	BaseModel
}

func (BillDashboardAggregate) TableName() string {
	return "bill_dashboard_aggregates"
}

// BillPrice 单价管理，适用于专有云
type BillPrice struct {
	ID            uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor        string          `gorm:"column:vendor; type:varchar(50)" json:"vendor" binding:"required"` // 云厂商
	ResourceType  string          `gorm:"column:resource_type; type:varchar(50)" json:"resource_type"`      // 资源类型
	Spec          string          `gorm:"column:spec; type:varchar(50)" json:"spec"`                        // 规格，如 1:2, 1:4 (对应scale)
	UnitPrice     decimal.Decimal `gorm:"column:unit_price; type:decimal(25,15)" json:"unit_price"`         // 单价 (对应price)
	Currency      string          `gorm:"column:currency; type:varchar(10)" json:"currency"`                // 货币：USD, CNY
	Unit          string          `gorm:"column:unit; type:varchar(50)" json:"unit"`                        // 单位：GB, Hour
	Region        string          `gorm:"column:region; type:varchar(50)" json:"region"`                    // 区域
	EffectiveDate string          `gorm:"column:effective_date; type:date" json:"effective_date"`           // 生效日期
	Description   string          `gorm:"column:description; type:text" json:"description"`                 // 描述
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillPrice) TableName() string {
	return "bill_price"
}

// BillPricing 云定价（来自 AWS/Azure 定价 API）
type BillPricing struct {
	ID           uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	CloudType    string          `gorm:"column:cloud_type; type:varchar(50)" json:"cloud_type"`            // 云厂商：aws, alibaba, tencent
	ServiceCode  string          `gorm:"column:service_code; type:varchar(50)" json:"service_code"`        // 服务代码
	InstanceType string          `gorm:"column:instance_type; type:varchar(50)" json:"instance_type"`      // 实例类型
	Region       string          `gorm:"column:region; type:varchar(50)" json:"region"`                    // 区域
	PricePerUnit decimal.Decimal `gorm:"column:price_per_unit; type:decimal(25,15)" json:"price_per_unit"` // 单价
	Currency     string          `gorm:"column:currency; type:varchar(10)" json:"currency"`                // 货币：USD, CNY
	Unit         string          `gorm:"column:unit; type:varchar(50)" json:"unit"`                        // 单位：GB, Hour
	SKU          string          `gorm:"column:sku; type:varchar(100)" json:"sku"`                         // SKU
	BaseModel
}

func (BillPricing) TableName() string {
	return "bill_pricing"
}

// 金额字段存各 vendor 原生币种（AWS/YCloud 为 USD）；展示层按 usd_to_cny_rate 换算。
type BillDailyCost struct {
	ID            uint    `gorm:"column:id;primaryKey;autoIncrement" json:"id,omitempty"`
	Date          string  `gorm:"column:date;type:varchar(10);index:idx_bdc_date,priority:1;not null" json:"date"`
	Vendor        string  `gorm:"column:vendor;type:varchar(50);index:idx_bdc_date,priority:2;not null" json:"vendor"`
	Cost          float64 `gorm:"column:cost;type:decimal(25,6);not null;default:0" json:"cost"`
	EffectiveCost float64 `gorm:"column:effective_cost;type:decimal(25,6);not null;default:0" json:"effective_cost"`
	ListCost      float64 `gorm:"column:list_cost;type:decimal(25,6);not null;default:0" json:"list_cost"`
	BaseModel
}

func (BillDailyCost) TableName() string {
	return "bill_daily_cost"
}

// BillResource 云资源清单
type BillResource struct {
	ID             uint      `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor         string    `gorm:"column:vendor; type:varchar(50)" json:"vendor"`          // 云厂商
	CloudAccountID uint      `gorm:"column:cloud_account_id" json:"cloud_account_id"`        // 系统云账户ID
	AccountID      string    `gorm:"column:account_id; type:varchar(100)" json:"account_id"` // 云厂商账户ID
	ResourceID     string    `gorm:"column:resource_id; type:text" json:"resource_id"`       // 资源ID
	ResourceType   string    `gorm:"column:resource_type; type:text" json:"resource_type"`   // 资源类型
	ResourceName   string    `gorm:"column:resource_name; type:text" json:"resource_name"`   // 资源名称
	InstanceType   string    `gorm:"column:instance_type; type:text" json:"instance_type"`   // 实例类型
	Region         string    `gorm:"column:region; type:varchar(50)" json:"region"`          // 区域
	Zone           string    `gorm:"column:zone; type:varchar(50)" json:"zone"`              // 可用区
	Tags           string    `gorm:"column:tags; type:text" json:"tags"`                     // 标签JSON
	Status         string    `gorm:"column:status; type:varchar(50)" json:"status"`          // 状态
	FirstSeen      time.Time `gorm:"column:first_seen" json:"first_seen,omitempty"`          // 首次出现在账单中的时间
	LastSeen       time.Time `gorm:"column:last_seen" json:"last_seen,omitempty"`            // 最近出现在账单中的时间
	BaseModel
}

func (BillResource) TableName() string {
	return "bill_resources"
}

// Scope functions for query building

// RecordsByVendor scope，根据云厂商查询
func RecordsByVendor(vendor string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("vendor = ?", vendor)
	}
}

// RecordsByCycle scope，根据月份查询
func RecordsByCycle(cycle string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("cycle = ?", cycle)
	}
}

// RecordsByResourceCode scope，根据resourceCode查询
func RecordsByResourceCode(resourceCode string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("resource_code = ?", resourceCode)
	}
}

// RecordsByServiceCode scope, 根据serviceCode查询
func RecordsByServiceCode(serviceCode string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("service_code = ?", serviceCode)
	}
}
