package model

import (
	"time"
)

// Setting 系统设置表
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Category  string    `gorm:"size:50;index" json:"category"` // system, ldap, sso, security, audit, notification, terminal, upload, host_monitor, windows
	Type      string    `gorm:"size:20" json:"type"`           // string, number, boolean, json
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SettingResponse 设置响应
type SettingResponse struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

// SettingsCategory 设置分类
const (
	CategorySystem       = "system"
	CategoryLDAP         = "ldap"
	CategorySSO          = "sso"
	CategorySecurity     = "security"
	CategoryAudit        = "audit"
	CategoryNotification = "notification"
	CategoryTerminal     = "terminal"
	CategoryUpload       = "upload"
	CategoryHostMonitor  = "host_monitor"
	CategoryWindows      = "windows"
	CategoryRegistry        = "registry"         // 容器仓库：harbor / ecr / nexus
	CategoryRelease         = "release"         // 发布配置：Git 拉取、制品仓库等
	CategoryReleaseApproval = "release_approval" // 发布审批（飞书/钉钉/企业微信）
	CategoryAiAssistant    = "ai_assistant"     // AI 运维助手：LLM API Key、Base URL、Model 等
)

// TableName 指定表名
func (Setting) TableName() string {
	return "settings"
}

// HostMonitorConfig 主机监控配置
type HostMonitorConfig struct {
	Enabled    bool   `json:"enabled"`    // 是否启用监控
	Interval   int    `json:"interval"`   // 检测间隔（分钟）
	Method     string `json:"method"`     // 检测方式: tcp, icmp, http
	Timeout    int    `json:"timeout"`    // 超时时间（秒）
	Concurrent int    `json:"concurrent"` // 最大并发数
}

// MonitorMethod 监控方式常量
const (
	MonitorMethodTCP  = "tcp"  // TCP端口检测
	MonitorMethodICMP = "icmp" // ICMP Ping检测
	MonitorMethodHTTP = "http" // HTTP检测
)
