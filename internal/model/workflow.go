package model

import (
	"time"

	"gorm.io/datatypes"
)

// Workflow 表示工单/工作流，草稿与正式工单共用此表，通过 status 区分
type Workflow struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	JobID        string         `gorm:"type:varchar(40);uniqueIndex" json:"jobid"`
	Title        string         `gorm:"type:varchar(200);not null" json:"title"`
	WorkflowType string         `gorm:"type:varchar(50);not null" json:"workflow_type"`
	Status       string         `gorm:"type:varchar(20);default:draft;index" json:"status"`
	Comment      string         `gorm:"type:text" json:"comment"`
	Labels       datatypes.JSON `gorm:"type:json" json:"labels"`
	ApplicantID  string         `gorm:"type:varchar(50);index" json:"applicant_id"`
	ApplicantName string        `gorm:"type:varchar(100)" json:"applicant_name"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`

	Steps []WorkflowStep `gorm:"foreignKey:JobID;references:JobID" json:"steps"`
}

func (Workflow) TableName() string {
	return "workflows"
}

// WorkflowStep 表示工作流中的步骤
type WorkflowStep struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	JobID            string         `gorm:"type:varchar(40);index:idx_job_step;uniqueIndex:uk_job_step" json:"jobid"`
	StepID           string         `gorm:"type:varchar(64);index:idx_job_step;uniqueIndex:uk_job_step" json:"step_id"`
	StepType         string         `gorm:"type:varchar(50)" json:"step_type"`
	StepName         string         `gorm:"type:varchar(200)" json:"step_name"`
	StepComment      string         `gorm:"type:text" json:"step_comment"`
	StepStatus       int            `gorm:"type:int;default:0" json:"step_status"` // 0 等待 1 成功 2 失败 3 执行中 4 拒绝 5 跳过 7 回滚中
	FuncKwargsJSON   datatypes.JSON `gorm:"type:json" json:"func_kwargs_json"`
	WhoHasPermission datatypes.JSON `gorm:"type:json" json:"who_has_permission"`
	RequireSteps     datatypes.JSON `gorm:"type:json" json:"require_steps"`
	StepOrder        int            `gorm:"type:int;default:0" json:"step_order"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

func (WorkflowStep) TableName() string {
	return "workflow_steps"
}

// WorkflowComment 用于步骤评论/操作记录（精简版）
type WorkflowComment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JobID     string    `gorm:"type:varchar(40);index" json:"jobid"`
	StepID    string    `gorm:"type:varchar(64);index" json:"step_id"`
	UserID    string    `gorm:"type:varchar(50)" json:"user_id"`
	UserName  string    `gorm:"type:varchar(100)" json:"user_name"`
	Action    string    `gorm:"type:varchar(30)" json:"action"`
	Comment   string    `gorm:"type:text" json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (WorkflowComment) TableName() string {
	return "workflow_comments"
}

