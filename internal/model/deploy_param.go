package model

import "time"

// DeployParam 部署参数定义表（只读参考数据）
// 存储 Helm/K8s 部署参数的元数据，每个项目可自定义参数值
type DeployParam struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string    `gorm:"type:varchar(200);not null;uniqueIndex" json:"name"` // 字段名称
	DataType       string    `gorm:"type:varchar(50);not null" json:"data_type"`         // 数据类型
	ChineseName    string    `gorm:"type:varchar(200);not null" json:"chinese_name"`     // 中文名称
	ValidationRule string    `gorm:"type:text" json:"validation_rule"`                   // 校验规则（正则表达式）
	Description    string    `gorm:"type:text" json:"description"`                       // 说明
	GroupName      string    `gorm:"type:varchar(100);default:''" json:"group_name"`     // 分组名称
	SortOrder      int       `gorm:"default:0" json:"sort_order"`                        // 排序
	Category       string    `gorm:"type:varchar(50);default:'basic'" json:"category"`   // 分类: basic(基础), k8s(K8s配置), probe(探针), network(网络), storage(存储), cronjob(定时任务), other(其他)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (DeployParam) TableName() string {
	return "deploy_params"
}

// AppDeployParamConfig 应用部署参数配置
type AppDeployParamConfig struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID      string    `gorm:"type:varchar(36);not null;uniqueIndex:idx_app_env_param" json:"app_id"`
	Env        string    `gorm:"type:varchar(20);not null;uniqueIndex:idx_app_env_param" json:"env"`
	ParamName  string    `gorm:"type:varchar(200);not null;uniqueIndex:idx_app_env_param" json:"param_name"`
	ParamValue string    `gorm:"type:text" json:"param_value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (AppDeployParamConfig) TableName() string {
	return "app_deploy_param_configs"
}

// AppDeployParamDefault 全局默认参数值
type AppDeployParamDefault struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ParamName    string    `gorm:"type:varchar(200);not null;uniqueIndex" json:"param_name"`
	DefaultValue string    `gorm:"type:text" json:"default_value"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AppDeployParamDefault) TableName() string {
	return "app_deploy_param_defaults"
}

// ResolvedDeployParam 解析后的最终参数值
type ResolvedDeployParam struct {
	ParamName   string `json:"param_name"`
	ChineseName string `json:"chinese_name"`
	DataType    string `json:"data_type"`
	Value       string `json:"value"`
	Source      string `json:"source"` // "default" | "app"
}
