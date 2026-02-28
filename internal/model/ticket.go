package model

import (
	"time"
	"gorm.io/datatypes"
)

// Ticket 工单
type Ticket struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	TicketNumber      string         `gorm:"type:varchar(50);uniqueIndex" json:"ticket_number"`
	TemplateID        *uint          `gorm:"index" json:"template_id,omitempty"`
	Type              string         `gorm:"type:varchar(20);default:daily;index" json:"type"` // 工单类型: daily(日常工单), deployment(发布工单)
	Title             string         `gorm:"type:varchar(200);not null" json:"title"`
	FormData          datatypes.JSON `gorm:"type:json;not null" json:"form_data"`
	Status            string         `gorm:"type:varchar(20);default:draft;index" json:"status"`
	Priority          string         `gorm:"type:varchar(20);default:normal" json:"priority"`
	ApplicantID       string         `gorm:"type:varchar(50);not null;index" json:"applicant_id"`
	ApplicantName     string         `gorm:"type:varchar(100);not null" json:"applicant_name"`
	ApplicantEmail    string         `gorm:"type:varchar(100)" json:"applicant_email"`
	ApprovalPlatform  string         `gorm:"type:varchar(20)" json:"approval_platform"`
	ApprovalInstanceID string        `gorm:"type:varchar(100)" json:"approval_instance_id"`
	ApprovalURL       string         `gorm:"type:varchar(500)" json:"approval_url"`
	CurrentApprover   string         `gorm:"type:varchar(100)" json:"current_approver"`
	Approvers         datatypes.JSON `gorm:"type:json" json:"approvers"`
	ApprovalSteps     datatypes.JSON `gorm:"type:json" json:"approval_steps"`
	ApprovalResult    string         `gorm:"type:varchar(20)" json:"approval_result"`
	ApprovalComment   string         `gorm:"type:text" json:"approval_comment"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	
	Template          FormTemplate   `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

// TableName 指定表名
func (Ticket) TableName() string {
	return "tickets"
}

