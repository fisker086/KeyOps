package model

import (
	"time"
)

// ApplicationDeployBinding 应用-发布绑定模型
type ApplicationDeployBinding struct {
	ID                string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ApplicationID     string     `json:"applicationId" gorm:"type:varchar(36);not null;index"`  // 应用ID
	DeployType        string     `json:"deployType" gorm:"type:varchar(20);not null;index"`    // 发布类型: jenkins, argocd
	DeployConfigID    string     `json:"deployConfigId" gorm:"type:varchar(100);not null;index"` // 发布配置ID
	DeployConfigName  string     `json:"deployConfigName" gorm:"type:varchar(255)"`            // 发布配置名称
	Environment       string     `json:"environment" gorm:"type:varchar(50);index"`            // 环境: dev, test, qa, staging, prod
	JenkinsJob        string     `json:"jenkinsJob" gorm:"type:varchar(255)"`                  // Jenkins Job名称
	ArgoCDApplication string     `json:"argocdApplication" gorm:"type:varchar(255)"`           // ArgoCD Application名称
	DeployStrategy    string     `json:"deployStrategy" gorm:"type:varchar(32)"`                // 发布策略: rolling, blue_green, canary, multi_lane
	StrategyOptions   string     `json:"strategyOptions" gorm:"type:text"`                     // JSON，如 canary 比例、泳道 ID
	PipelineID        string     `json:"pipelineId" gorm:"type:varchar(36);index"`            // 关联的流水线定义 ID，空则用默认
	WebhookToken      string     `json:"webhookToken" gorm:"type:varchar(64);uniqueIndex"`    // Git 推送校验 token，绑定创建时生成
	Enabled           bool       `json:"enabled" gorm:"default:true;index"`                   // 是否启用
	Description       string     `json:"description" gorm:"type:text"`                        // 描述
	CreatedBy         string     `json:"createdBy" gorm:"type:varchar(36)"`                   // 创建用户ID
	CreatedAt         time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ApplicationDeployBinding) TableName() string {
	return "application_deploy_bindings"
}

// CreateApplicationDeployBindingRequest 创建应用-发布绑定请求
type CreateApplicationDeployBindingRequest struct {
	ApplicationID     string `json:"applicationId" binding:"required"`
	DeployType        string `json:"deployType" binding:"required,oneof=jenkins argocd"`
	DeployConfigID    string `json:"deployConfigId" binding:"required"`
	DeployConfigName  string `json:"deployConfigName"`
	Environment       string `json:"environment" binding:"required,oneof=dev test qa staging prod"`
	JenkinsJob        string `json:"jenkinsJob"`        // 当 deployType=jenkins 时必填
	ArgoCDApplication string `json:"argocdApplication"`  // 当 deployType=argocd 时必填
	DeployStrategy    string `json:"deployStrategy"`    // rolling, blue_green, canary, multi_lane
	StrategyOptions   string `json:"strategyOptions"`   // JSON
	PipelineID        string `json:"pipelineId"`        // 流水线定义 ID，空则默认
	Enabled           bool   `json:"enabled"`
	Description       string `json:"description"`
}

// UpdateApplicationDeployBindingRequest 更新应用-发布绑定请求
type UpdateApplicationDeployBindingRequest struct {
	DeployConfigID    string `json:"deployConfigId"`
	DeployConfigName  string `json:"deployConfigName"`
	Environment       string `json:"environment" binding:"omitempty,oneof=dev test qa staging prod"`
	JenkinsJob        string `json:"jenkinsJob"`
	ArgoCDApplication string `json:"argocdApplication"`
	DeployStrategy    string `json:"deployStrategy"`
	StrategyOptions   string `json:"strategyOptions"`
	PipelineID        string `json:"pipelineId"`
	Enabled           *bool  `json:"enabled"`
	Description       string `json:"description"`
}

// ApplicationDeployBindingInfo 应用-发布绑定信息（用于API响应，包含应用信息）
type ApplicationDeployBindingInfo struct {
	ID                string    `json:"id"`
	ApplicationID     string    `json:"applicationId"`
	ApplicationName   string    `json:"applicationName"`
	DeployType        string    `json:"deployType"`
	DeployConfigID    string    `json:"deployConfigId"`
	DeployConfigName  string    `json:"deployConfigName"`
	Environment       string    `json:"environment"`
	JenkinsJob        string    `json:"jenkinsJob"`
	ArgoCDApplication string    `json:"argocdApplication"`
	DeployStrategy    string    `json:"deployStrategy"`
	StrategyOptions   string    `json:"strategyOptions"`
	PipelineID        string    `json:"pipelineId"`
	WebhookToken      string    `json:"webhookToken"`
	WebhookURL        string    `json:"webhookUrl"`   // 前端展示用，由后端根据 token 拼出
	Enabled           bool      `json:"enabled"`
	Description       string    `json:"description"`
	CreatedBy         string    `json:"createdBy"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ListApplicationDeployBindingsRequest 查询应用-发布绑定列表请求
type ListApplicationDeployBindingsRequest struct {
	ApplicationID string `form:"applicationId"`
	DeployType    string `form:"deployType"`
	Environment   string `form:"environment"`
	Enabled       *bool  `form:"enabled"`
	Page          int    `form:"page"`
	PageSize      int    `form:"pageSize"`
}

// GetApplicationsForDeployRequest 获取可用于发布的应用列表请求
type GetApplicationsForDeployRequest struct {
	DeployType  string `form:"deployType" binding:"required,oneof=jenkins argocd"`
	Environment string `form:"environment"`
	Keyword     string `form:"keyword"` // 应用名称关键字搜索
}

