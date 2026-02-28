package model

import (
	"time"
)

// DBInstance 数据库实例模型
type DBInstance struct {
	ID              uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string     `json:"name" gorm:"type:varchar(255);not null"`
	DBType          string     `json:"dbType" gorm:"type:varchar(50);not null;default:'mysql';index"`
	Host            string     `json:"host" gorm:"type:varchar(255);not null"`
	Port            int        `json:"port" gorm:"type:int;not null;default:3306"`
	Username        string     `json:"username" gorm:"type:varchar(255)"`
	Password        string     `json:"-" gorm:"type:varchar(255);not null"` // 不在JSON中暴露
	DatabaseName    string     `json:"databaseName" gorm:"type:varchar(255)"`
	AuthDatabase    string     `json:"authDatabase" gorm:"type:varchar(255)"` // MongoDB 认证数据库
	Charset         string     `json:"charset" gorm:"type:varchar(50);default:'utf8mb4'"`
	ConnectionString string    `json:"connectionString" gorm:"type:text"` // MongoDB/Redis 连接字符串
	SSLEnabled      bool       `json:"sslEnabled" gorm:"default:false"`
	SSLCert          string     `json:"sslCert" gorm:"type:text"`
	Description     string     `json:"description" gorm:"type:text"`
	IsEnabled       bool       `json:"isEnabled" gorm:"default:true;index"`
	CreatedBy       string     `json:"createdBy" gorm:"type:varchar(36);not null;index"`
	CreatedAt       time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (DBInstance) TableName() string {
	return "db_instances"
}

// DBPermissionMetadata 数据库权限元数据
type DBPermissionMetadata struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         string     `json:"userId" gorm:"type:varchar(36);not null;index"`
	InstanceID     uint       `json:"instanceId" gorm:"type:bigint;not null;index"`
	DatabaseName   string     `json:"databaseName" gorm:"type:varchar(255)"`
	Table          string     `json:"tableName" gorm:"column:table_name;type:varchar(255)"`
	PermissionType string     `json:"permissionType" gorm:"type:varchar(50);not null"`
	GrantedBy      string     `json:"grantedBy" gorm:"type:varchar(36);not null"`
	GrantedAt      time.Time  `json:"grantedAt" gorm:"autoCreateTime"`
	ExpiresAt      *time.Time `json:"expiresAt" gorm:"type:timestamp;index"`
	Description    string     `json:"description" gorm:"type:text"`
	CreatedAt      time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (DBPermissionMetadata) TableName() string {
	return "db_permission_metadata"
}

// QueryLog 查询日志模型
type QueryLog struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         string     `json:"userId" gorm:"type:varchar(36);not null;index"`
	Username       string     `json:"username" gorm:"type:varchar(50);not null"`
	InstanceID     uint       `json:"instanceId" gorm:"type:bigint;not null;index"`
	InstanceName   string     `json:"instanceName" gorm:"type:varchar(255);not null"`
	DBType         string     `json:"dbType" gorm:"type:varchar(50);not null;index"`
	DatabaseName   string     `json:"databaseName" gorm:"type:varchar(255);index"`
	QueryContent   string     `json:"queryContent" gorm:"type:text;not null"`
	QueryType      string     `json:"queryType" gorm:"type:varchar(50);index"`
	AffectedRows   int        `json:"affectedRows" gorm:"default:0"`
	ResultCount    int        `json:"resultCount" gorm:"default:0"`
	ExecutionTimeMs int       `json:"executionTimeMs" gorm:"type:int"`
	Status         string     `json:"status" gorm:"type:varchar(20);default:'success';index"`
	ErrorMessage   string     `json:"errorMessage" gorm:"type:text"`
	ResultPreview  string     `json:"resultPreview" gorm:"type:text"`
	ClientIP       string     `json:"clientIp" gorm:"type:varchar(45)"`
	UserAgent      string     `json:"userAgent" gorm:"type:varchar(255)"`
	CreatedAt      time.Time  `json:"createdAt" gorm:"autoCreateTime;index"`
}

func (QueryLog) TableName() string {
	return "query_logs"
}
