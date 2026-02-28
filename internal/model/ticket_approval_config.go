package model

import (
	"time"
	"gorm.io/datatypes"
)

// TicketApprovalConfig 工单审批配置表
type TicketApprovalConfig struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	TemplateID    uint           `gorm:"not null;index" json:"template_id"`
	Platform      string         `gorm:"type:varchar(20);not null;index" json:"platform"` // 平台: dingtalk/feishu/wework
	ApprovalCode  string         `gorm:"type:varchar(100)" json:"approval_code"`          // 审批模板Code
	ApprovalFlow  datatypes.JSON `gorm:"type:json;not null" json:"approval_flow"`         // 审批流程配置 (JSON)
	AutoApprove   bool           `gorm:"default:false" json:"auto_approve"`               // 是否自动审批
	TimeoutHours  int            `gorm:"default:24" json:"timeout_hours"`                 // 超时时间(小时)
	CreatedAt     time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TicketApprovalConfig) TableName() string {
	return "ticket_approval_configs"
}

