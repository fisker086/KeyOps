package model

import (
	"time"
)

// ReleaseRun 发布代码运行记录（由 Git Webhook 或手动触发）
type ReleaseRun struct {
	ID                string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ApplicationID     string     `gorm:"type:varchar(36);index" json:"application_id"`     // 可选，关联应用
	RepoURL           string     `gorm:"type:varchar(512);not null;index" json:"repo_url"`
	Branch            string     `gorm:"type:varchar(255);not null;index" json:"branch"`
	CommitSHA         string     `gorm:"type:varchar(64);not null;index" json:"commit_sha"`
	CommitMessage     string     `gorm:"type:text" json:"commit_message"`
	Ref               string     `gorm:"type:varchar(512)" json:"ref"`                      // 如 refs/heads/main
	Source            string     `gorm:"type:varchar(20);default:webhook" json:"source"`    // webhook, manual, rollback
	Status            string     `gorm:"type:varchar(20);default:pending;index" json:"status"` // pending, running, success, failed, cancelled
	TriggeredBy       string     `gorm:"type:varchar(100)" json:"triggered_by"`            // webhook 时可为空或 git user
	CreatedBy         string     `gorm:"type:varchar(36);index" json:"created_by"`         // 手动触发时的用户 ID
	RollbackFromRunID   string     `gorm:"type:varchar(36);index" json:"rollback_from_run_id"`   // 回滚时指向被回滚的 run
	DeployStrategy      string     `gorm:"type:varchar(32)" json:"deploy_strategy"`             // 单次覆盖：rolling, blue_green, canary, multi_lane
	DeployedEnvironment string     `gorm:"type:varchar(32);index" json:"deployed_environment"`  // 实际部署到的环境（执行时写入，用于查「上次 prod 成功」）
	StartedAt           *time.Time `gorm:"type:timestamp" json:"started_at"`
	CompletedAt       *time.Time `gorm:"type:timestamp" json:"completed_at"`
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (ReleaseRun) TableName() string {
	return "release_runs"
}

const (
	ReleaseRunSourceWebhook  = "webhook"
	ReleaseRunSourceManual  = "manual"
	ReleaseRunSourceRollback = "rollback"
	ReleaseRunStatusPending   = "pending"
	ReleaseRunStatusRunning   = "running"
	ReleaseRunStatusSuccess  = "success"
	ReleaseRunStatusFailed   = "failed"
	ReleaseRunStatusCancelled = "cancelled"
)

// 发布策略（与 binding 一致，执行时传 Jenkins 参数）
const (
	DeployStrategyRolling    = "rolling"
	DeployStrategyBlueGreen = "blue_green"
	DeployStrategyCanary    = "canary"
	DeployStrategyMultiLane = "multi_lane"
)
