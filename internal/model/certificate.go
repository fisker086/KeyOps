package model

import (
	"time"
	"gorm.io/datatypes"
)

// DomainCertificate 域名证书（监控域名的SSL证书过期情况）
type DomainCertificate struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	Domain           string     `gorm:"type:varchar(255);uniqueIndex:idx_domain_port;not null" json:"domain" binding:"required"`
	Port             int        `gorm:"default:443;uniqueIndex:idx_domain_port" json:"port"`
	SSLCertificate   string     `gorm:"type:text" json:"ssl_certificate"`
	SSLCertificateKey string    `gorm:"type:text" json:"ssl_certificate_key"`
	StartTime        *time.Time `gorm:"type:timestamp" json:"start_time"`
	ExpireTime       *time.Time `gorm:"type:timestamp;index" json:"expire_time"`
	ExpireDays       int        `gorm:"default:0" json:"expire_days"` // 过期剩余天数
	IsMonitor        bool       `gorm:"default:true;index" json:"is_monitor"` // 是否监控
	AutoUpdate       bool       `gorm:"default:true" json:"auto_update"`      // 是否自动更新
	ConnectStatus    *bool      `gorm:"type:boolean" json:"connect_status"`   // 连接状态
	AlertDays        int            `gorm:"default:30" json:"alert_days"`        // 告警天数阈值（剩余天数小于等于此值时发送告警）
	AlertTemplateID  *uint          `gorm:"index" json:"alert_template_id"`       // 告警模板ID
	AlertChannelIDs  datatypes.JSON `gorm:"type:json" json:"alert_channel_ids"`  // 告警渠道ID数组（JSON格式，如 [1,2,3]）
	LastAlertTime    *time.Time     `gorm:"type:timestamp;index" json:"last_alert_time"` // 最后一次发送告警的时间（用于防止重复发送）
	Comment          string     `gorm:"type:text" json:"comment"`
	BaseModel
}

func (DomainCertificate) TableName() string {
	return "domain_certificates"
}

// SSLCertificate SSL证书（手动管理的SSL证书文件）
type SSLCertificate struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	Domain           string     `gorm:"type:varchar(255);not null;index" json:"domain" binding:"required"`
	SSLCertificate   string     `gorm:"type:text" json:"ssl_certificate"`
	SSLCertificateKey string    `gorm:"type:text" json:"ssl_certificate_key"`
	StartTime        *time.Time `gorm:"type:timestamp" json:"start_time"`
	ExpireTime       *time.Time `gorm:"type:timestamp;index" json:"expire_time"`
	Comment          string     `gorm:"type:text" json:"comment"`
	BaseModel
}

func (SSLCertificate) TableName() string {
	return "ssl_certificates"
}

// HostedCertificate 托管证书（托管在系统中的证书文件）
type HostedCertificate struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	Domain           string     `gorm:"type:varchar(255);not null;index" json:"domain" binding:"required"`
	SSLCertificate   string     `gorm:"type:text" json:"ssl_certificate"`
	SSLCertificateKey string    `gorm:"type:text" json:"ssl_certificate_key"`
	StartTime        *time.Time `gorm:"type:timestamp" json:"start_time"`
	ExpireTime       *time.Time `gorm:"type:timestamp;index" json:"expire_time"`
	Comment          string     `gorm:"type:text" json:"comment"`
	BaseModel
}

func (HostedCertificate) TableName() string {
	return "hosted_certificates"
}
