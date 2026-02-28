package model

import (
	"time"
)

// Deployment 部署记录
type Deployment struct {
	ID                 string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	ProjectName        string     `gorm:"type:varchar(255);not null;index" json:"project_name"`
	ProjectID          string     `gorm:"type:varchar(100);index" json:"project_id"`
	EnvID              string     `gorm:"type:varchar(100);index" json:"env_id"`
	EnvName            string     `gorm:"type:varchar(100)" json:"env_name"`
	ClusterID          string     `gorm:"type:varchar(100);index" json:"cluster_id"`
	ClusterName        string     `gorm:"type:varchar(100)" json:"cluster_name"`
	Namespace          string     `gorm:"type:varchar(100)" json:"namespace"`
	DeployType         string     `gorm:"type:varchar(50);not null;index" json:"deploy_type"` // jenkins, k8s, gitops, argocd, helm
	DeployConfig       string     `gorm:"type:json" json:"deploy_config"`                     // JSON格式存储不同发布方式的配置
	Version            string     `gorm:"type:varchar(100)" json:"version"`
	ArtifactURL        string     `gorm:"type:text" json:"artifact_url"`
	JenkinsJob         string     `gorm:"type:varchar(255)" json:"jenkins_job"`                 // 向后兼容字段
	JenkinsBuildNumber int        `gorm:"type:int" json:"jenkins_build_number"`                 // 向后兼容字段
	K8sYAML            string     `gorm:"type:text" json:"k8s_yaml"`                            // 向后兼容字段
	K8sKind            string     `gorm:"type:varchar(50)" json:"k8s_kind"`                     // 向后兼容字段
	VerifyEnabled      bool       `gorm:"default:false" json:"verify_enabled"`                  // 是否启用kubedog验证
	VerifyTimeout      int        `gorm:"default:300" json:"verify_timeout"`                    // 验证超时时间（秒），默认300秒
	Status             string     `gorm:"type:varchar(20);default:pending;index" json:"status"` // pending, running, success, failed, cancelled
	LogPath            string     `gorm:"type:text" json:"log_path"`
	BuildLog           string     `gorm:"type:longtext" json:"build_log"` // Jenkins构建日志内容（保存完整日志，即使job被删除也能查看）
	Duration           int        `gorm:"type:int" json:"duration"`       // 秒
	Description        string     `gorm:"type:text" json:"description"`
	CreatedBy          string     `gorm:"type:varchar(36);index" json:"created_by"`
	CreatedByName      string     `gorm:"type:varchar(100)" json:"created_by_name"`
	StartedAt          *time.Time `gorm:"type:timestamp" json:"started_at"`
	CompletedAt        *time.Time `gorm:"type:timestamp" json:"completed_at"`
	CreatedAt          time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (Deployment) TableName() string {
	return "deployments"
}

// DeploymentStatus 部署状态常量
const (
	DeploymentStatusPending   = "pending"
	DeploymentStatusRunning   = "running"
	DeploymentStatusSuccess   = "success"
	DeploymentStatusFailed    = "failed"
	DeploymentStatusCancelled = "cancelled"
)
