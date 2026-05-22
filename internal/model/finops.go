package model

// Budget 预算
type Budget struct {
	ID                 uint      `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Name               string    `gorm:"column:name;type:varchar(100)" json:"name"`
	Amount             float64   `gorm:"column:amount;type:decimal(25,15)" json:"amount"`
	Period             string    `gorm:"column:period;type:varchar(20)" json:"period"`                                  // monthly/quarterly/yearly
	StartDate          string    `gorm:"column:start_date;type:varchar(10)" json:"start_date"`
	EndDate            string    `gorm:"column:end_date;type:varchar(10)" json:"end_date"`
	AlertThreshold     float64   `gorm:"column:alert_threshold;type:decimal(5,2)" json:"alert_threshold"` // 告警阈值百分比
	AlertChannelIDs    string    `gorm:"column:alert_channel_ids;type:text" json:"alert_channel_ids,omitempty"` // JSON数组：[1,2,3]
	OrgID              string    `gorm:"column:org_id;type:varchar(50)" json:"org_id,omitempty"`
	Owner              string    `gorm:"column:owner;type:varchar(100)" json:"owner,omitempty"`
	Status             string    `gorm:"column:status;type:varchar(20);default:'active'" json:"status"`
	BaseModel
}

func (Budget) TableName() string {
	return "finops_budgets"
}

// Pool 资源池
type Pool struct {
	ID          uint      `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Name        string    `gorm:"column:name;type:varchar(100)" json:"name"`
	Description string    `gorm:"column:description;type:text" json:"description,omitempty"`
	LimitAmount float64   `gorm:"column:limit_amount;type:decimal(25,15)" json:"limit_amount"` // 费用限制
	OrgID       string    `gorm:"column:org_id;type:varchar(50)" json:"org_id,omitempty"`
	Owner       string    `gorm:"column:owner;type:varchar(100)" json:"owner,omitempty"`
	Members     string    `gorm:"column:members;type:json" json:"members,omitempty"` // JSON数组
	Status      string    `gorm:"column:status;type:varchar(20);default:'active'" json:"status"`
	BaseModel
}

func (Pool) TableName() string {
	return "finops_pools"
}

// Policy 策略（TTL、电源管理等）
type Policy struct {
	ID          uint      `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Name        string    `gorm:"column:name;type:varchar(100)" json:"name"`
	Type        string    `gorm:"column:type;type:varchar(50)" json:"type"` // ttl/power_off/tag
	Action      string    `gorm:"column:action;type:varchar(50)" json:"action"` // stop/terminate/tag
	Conditions  string    `gorm:"column:conditions;type:json" json:"conditions"` // JSON条件
	TargetResources string `gorm:"column:target_resources;type:text" json:"target_resources,omitempty"` // 目标资源筛选
	Enabled     bool      `gorm:"column:enabled;default:true" json:"enabled"`
	Schedule    string    `gorm:"column:schedule;type:varchar(100)" json:"schedule,omitempty"` // cron表达式
	Description string    `gorm:"column:description;type:text" json:"description,omitempty"`
	BaseModel
}

func (Policy) TableName() string {
	return "finops_policies"
}
