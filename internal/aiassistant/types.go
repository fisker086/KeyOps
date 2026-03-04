package aiassistant

// SessionStep 单步记录
type SessionStep struct {
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Session 会话
type Session struct {
	SessionID   string        `json:"session_id"`
	Task        string        `json:"task"`
	EnvID       string        `json:"env_id,omitempty"`
	Role        string        `json:"role,omitempty"`
	ModelID     string        `json:"model_id,omitempty"`   // 模型配置 ID
	ModelName   string        `json:"model_name,omitempty"` // 显示名，创建时写入
	ScheduleID  string        `json:"schedule_id,omitempty"`
	CreatedBy   string        `json:"created_by,omitempty"`
	Status      string        `json:"status"` // running, completed, error
	StartTime   string        `json:"start_time"`
	Steps       []SessionStep `json:"steps"`
	FinalAnswer string        `json:"final_answer,omitempty"`
}

// SessionSummary 会话摘要（列表用）
type SessionSummary struct {
	SessionID  string `json:"session_id"`
	Task       string `json:"task"`
	StartTime  string `json:"start_time"`
	Status     string `json:"status"`
	EnvID      string `json:"env_id,omitempty"`
	Role       string `json:"role,omitempty"`
	ModelID    string `json:"model_id,omitempty"`
	ModelName  string `json:"model_name,omitempty"` // 显示用
	ScheduleID string `json:"schedule_id,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
}

// Schedule 定时任务
type Schedule struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	EnvID                  string   `json:"env_id"`
	ModelID                string   `json:"model_id,omitempty"` // 模型配置 ID，空则使用第一个
	Cron                   string   `json:"cron"`
	TaskPrompt             string   `json:"task_prompt"`
	Role                   string   `json:"role,omitempty"`
	LarkBotID              string   `json:"lark_bot_id,omitempty"`
	LarkGroupName          string   `json:"lark_group_name,omitempty"`
	LarkFolderID           string   `json:"lark_folder_id,omitempty"`
	Enabled                bool     `json:"enabled"`
	ResponsibleUser        string   `json:"responsible_user,omitempty"`
	NotificationChannelIDs []uint   `json:"notification_channel_ids,omitempty"` // 监控告警渠道 ID 列表，巡检结果通知到这些渠道
	CreatedAt              string   `json:"created_at,omitempty"`
}

// Environment 目标环境（数据来源：数据库表 ai_assistant_environments，由 store.ListEnvironments / GetEnvironment 加载）
// 支持三种技能：Prometheus、Grafana、K8s 集群，可组合配置
// K8sClusterID 关联 K8s 管理中的集群 ID，选择后该环境可用于 K8s 巡检等场景
type Environment struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	PromURL        string                 `json:"prom_url,omitempty"`
	GrafURL        string                 `json:"graf_url,omitempty"`
	GrafToken      string                 `json:"graf_token,omitempty"`
	Cluster        string                 `json:"cluster,omitempty"`
	K8sClusterID   string                 `json:"k8s_cluster_id,omitempty"`   // 关联 K8s 集群 ID（来自 k8s 管理）
	AllowedRoleIDs []string               `json:"allowed_role_ids,omitempty"`   // 允许使用该环境的平台角色ID列表，空表示所有角色可用
	ExtraConfig    map[string]interface{} `json:"extra_config,omitempty"`      // 保留字段，DB 兼容；界面已移除，仅支持 Prometheus/Grafana/K8s
	EnabledSkills  []string               `json:"enabled_skills,omitempty"`    // 当前环境启用的技能（prometheus/grafana/k8s），API 返回时由 handler 填充
}

// ModelConfig 模型配置（API 响应用，列表可隐藏 api_key）
type ModelConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`   // 列表时可为空，详情/编辑时返回
	BaseURL  string `json:"base_url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	MaxSteps int    `json:"max_steps,omitempty"`
	Sort     int    `json:"sort,omitempty"`
}

// Expert 专家角色
type Expert struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SystemPrompt   string   `json:"system_prompt"`
	SkillID        string   `json:"skill_id,omitempty"`        // 关联技能（如 k8s-install），非空时前置注入技能知识库，数据库 system_prompt 在后可覆盖
	IsCustom       bool     `json:"is_custom,omitempty"`
	AllowedRoleIDs []string `json:"allowed_role_ids,omitempty"` // 允许使用该专家的平台角色ID列表，空表示所有角色可用
}
