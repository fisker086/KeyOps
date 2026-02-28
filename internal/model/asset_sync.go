package model

import (
	"time"
)

// AssetSyncConfig 资产同步配置
type AssetSyncConfig struct {
	ID             string     `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null"`
	Type           string     `json:"type" gorm:"not null"` // prometheus, zabbix, cmdb, custom
	Enabled        bool       `json:"enabled" gorm:"default:true"`
	URL            string     `json:"url" gorm:"not null"`
	AuthType       string     `json:"auth_type"` // none, basic, token, oauth
	Username       string     `json:"username"`
	Password       string     `json:"password"`
	Token          string     `json:"token"`
	SyncInterval   int        `json:"sync_interval"` // 同步间隔（分钟）
	LastSyncTime   *time.Time `json:"last_sync_time"`
	LastSyncStatus string     `json:"last_sync_status"` // success, failed
	SyncedCount    int        `json:"synced_count"`
	ErrorMessage   string     `json:"error_message"`
	Config         string     `json:"config" gorm:"type:text"` // JSON配置
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AssetSyncConfig) TableName() string {
	return "asset_sync_configs"
}

// AssetSyncLog 资产同步日志
type AssetSyncLog struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	ConfigID     string    `json:"config_id" gorm:"index;not null"`
	Status       string    `json:"status"` // success, failed
	SyncedCount  int       `json:"synced_count"`
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	Duration     int       `json:"duration"` // 同步耗时（秒）
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (AssetSyncLog) TableName() string {
	return "asset_sync_logs"
}
