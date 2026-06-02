package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FormTemplate 表单模板
type FormTemplate struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UUID           string         `gorm:"type:varchar(36);uniqueIndex" json:"uuid"` // 对外稳定标识，用于审批回调 path 等
	Name           string         `gorm:"type:varchar(100);not null" json:"name"`
	Category       string         `gorm:"type:varchar(50)" json:"category"`
	Description    string         `gorm:"type:text" json:"description"`
	Schema         datatypes.JSON `gorm:"column:schema;type:json;not null" json:"schema"`
	ApprovalConfig datatypes.JSON `gorm:"type:json" json:"approval_config"`
	Status         string         `gorm:"type:varchar(20);default:active" json:"status"`
	Version        string         `gorm:"type:varchar(20);default:1.0.0" json:"version"`
	CreatedBy      string         `gorm:"type:varchar(50)" json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// TableName 指定表名
func (FormTemplate) TableName() string {
	return "form_templates"
}

// BeforeCreate 创建时自动生成 UUID（不暴露自增 id 给外部回调）
func (t *FormTemplate) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(t.UUID) == "" {
		t.UUID = uuid.New().String()
	}
	return nil
}
