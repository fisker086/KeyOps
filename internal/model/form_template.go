package model

import (
	"time"
	"gorm.io/datatypes"
)

// FormTemplate 表单模板
type FormTemplate struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"type:varchar(100);not null" json:"name"`
	Category      string         `gorm:"type:varchar(50)" json:"category"`
	Description   string         `gorm:"type:text" json:"description"`
	Schema        datatypes.JSON `gorm:"column:schema;type:json;not null" json:"schema"`
	ApprovalConfig datatypes.JSON `gorm:"type:json" json:"approval_config"`
	Status        string         `gorm:"type:varchar(20);default:active" json:"status"`
	Version       string         `gorm:"type:varchar(20);default:1.0.0" json:"version"`
	CreatedBy     string         `gorm:"type:varchar(50)" json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// TableName 指定表名
func (FormTemplate) TableName() string {
	return "form_templates"
}

