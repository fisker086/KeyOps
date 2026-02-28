package model

// Monitor Prometheus 监控查询语句
type Monitor struct {
	ID        uint      `gorm:"column:id; primary_key; AUTO_INCREMENT" json:"id,omitempty"`
	Name      string    `gorm:"column:name; type:varchar(100); uniqueIndex:idx_name; not null" json:"name" binding:"required"` // 监控图表类型/名称
	Expr      string    `gorm:"column:expr; type:text; not null" json:"expr" binding:"required"`                                // 查询监控表达式（PromQL）
	CreatedBy string    `gorm:"column:created_by; type:varchar(36)" json:"created_by"`                                         // 创建用户ID
	UpdatedBy string    `gorm:"column:updated_by; type:varchar(36)" json:"updated_by"`                                         // 更新用户ID
	BaseModel
}

// TableName 表名
func (Monitor) TableName() string {
	return "monitors"
}

