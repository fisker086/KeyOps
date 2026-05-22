package model

import (
	"time"
)

// CloudAccount 云账户凭证管理
type CloudAccount struct {
	ID             uint            `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Name           string          `gorm:"column:name;type:varchar(100)" json:"name"`
	CloudType      string          `gorm:"column:cloud_type;type:varchar(50);index" json:"cloud_type"`
	AccessKeyID    string          `gorm:"column:access_key_id;type:varchar(200)" json:"access_key_id,omitempty"`
	SecretAccessKey string        `gorm:"column:secret_access_key;type:varchar(500)" json:"secret_access_key,omitempty"`
	Region         string          `gorm:"column:region;type:varchar(50)" json:"region"`
	BucketName     string          `gorm:"column:bucket_name;type:varchar(200)" json:"bucket_name,omitempty"`
	BucketPrefix   string          `gorm:"column:bucket_prefix;type:varchar(100)" json:"bucket_prefix,omitempty"`
	ReportName     string          `gorm:"column:report_name;type:varchar(100)" json:"report_name,omitempty"`
	AccountID      string          `gorm:"column:account_id;type:varchar(100);index" json:"account_id"`
	Status         string          `gorm:"column:status;type:varchar(20);default:'active'" json:"status"`
	LastImportAt   *time.Time     `gorm:"column:last_import_at" json:"last_import_at,omitempty"`
	LastImportError string        `gorm:"column:last_import_error;type:text" json:"last_import_error,omitempty"`
	SyncCron        string `gorm:"column:sync_cron;type:varchar(100)" json:"sync_cron,omitempty"`
	NotifyEnabled   *bool  `gorm:"column:notify_enabled;default:false" json:"notify_enabled"`
	NotifyChannelID uint   `gorm:"column:notify_channel_id" json:"notify_channel_id"`
	BaseModel
}

func (CloudAccount) TableName() string {
	return "bill_cloud_accounts"
}
