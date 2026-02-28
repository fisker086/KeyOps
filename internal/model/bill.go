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
	ResourceType  string          `gorm:"column:resource_type; type:varchar(50)" json:"resource_type"`         // 资源类型
	ResourceCode  string          `gorm:"column:resource_code; type:varchar(50)" json:"resource_code"`         // 资源类型代码
	ServiceType   string          `gorm:"column:service_type; type:varchar(50)" json:"service_type,omitempty"` // 服务类型，腾讯云没有此字段
	ServiceCode   string          `gorm:"column:service_code; type:varchar(50)" json:"service_code,omitempty"` // 服务类型代码，腾讯云没有此字段
	ConsumeAmount decimal.Decimal `gorm:"column:consume_amount; type:decimal(25,15)" json:"consume_amount"`    // 费用总额
	SummaryID     uint            `gorm:"column:summary_id; type:uint" json:"summary_id"`                      // 关联的summary表ID
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillSummaryDetail) TableName() string {
	return "bill_summary_detail"
}

// BillRecord 账单消费记录
type BillRecord struct {
	ID            uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor        string          `gorm:"column:vendor; type:varchar(50)" json:"vendor" binding:"required"`    // 云厂商，tencent、huawei-langgemap、huawei-bjlg
	Cycle         string          `gorm:"column:cycle; type:varchar(10)" json:"cycle" binding:"required"`      // 账单月份
	InstanceID    string          `gorm:"column:instance_id; type:varchar(200)" json:"instance_id"`            // 资源ID
	ResourceName  string          `gorm:"column:resource_name; type:varchar(200)" json:"resource_name"`        // 资源名称
	SpecDesc      string          `gorm:"column:spec_desc; type:text" json:"spec_desc"`                        // 资源配置
	ConsumeAmount decimal.Decimal `gorm:"column:consume_amount; type:decimal(25,15)" json:"consume_amount"`    // 费用
	ResourceType  string          `gorm:"column:resource_type; type:varchar(50)" json:"resource_type"`         // 资源类型
	ResourceCode  string          `gorm:"column:resource_code; type:varchar(50)" json:"resource_code"`         // 资源类型代码
	ServiceType   string          `gorm:"column:service_type; type:varchar(50)" json:"service_type,omitempty"` // 服务类型，腾讯云没有此字段
	ServiceCode   string          `gorm:"column:service_code; type:varchar(50)" json:"service_code,omitempty"` // 服务类型代码，腾讯云没有此字段
	Extra         string          `gorm:"column:extra; type:text" json:"extra"`                                // 扩展字段
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillRecord) TableName() string {
	return "bill_records"
}

// BillPrice 单价管理，适用于专有云
type BillPrice struct {
	ID          uint            `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Vendor      string          `gorm:"column:vendor; type:varchar(50)" json:"vendor" binding:"required"` // 云厂商
	ResourceType string         `gorm:"column:resource_type; type:varchar(50)" json:"resource_type"`   // 资源类型
	Scale       string          `gorm:"column:scale; type:varchar(50)" json:"scale"`                     // 规格，如 1:2, 1:4
	Cluster     string          `gorm:"column:cluster; type:varchar(50)" json:"cluster"`                 // 集群
	Price       decimal.Decimal `gorm:"column:price; type:decimal(25,15)" json:"price"`                 // 单价
	Description string          `gorm:"column:description; type:text" json:"description"`                // 描述
	BaseModel
}

// TableName 统一加上bill_前缀
func (BillPrice) TableName() string {
	return "bill_price"
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

