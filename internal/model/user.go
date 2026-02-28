package model

import (
	"time"
)

// User 平台用户
type User struct {
	ID                     string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Username               string     `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`
	Password               string     `json:"-" gorm:"type:varchar(255);not null"` // 不在JSON中暴露
	SSHPublicKey           string     `json:"sshPublicKey,omitempty" gorm:"type:text"`
	SSHPrivateKeyEncrypted string     `json:"-" gorm:"type:text;column:ssh_private_key_encrypted"`         // 不在JSON中暴露
	AuthMethod             string     `json:"authMethod" gorm:"type:varchar(20);default:'password';index"` // password, publickey, both
	SSHKeyGeneratedAt      *time.Time `json:"sshKeyGeneratedAt,omitempty" gorm:"type:timestamp"`
	SSHKeyFingerprint      string     `json:"sshKeyFingerprint,omitempty" gorm:"type:varchar(255)"`
	Email                  string     `json:"email" gorm:"type:varchar(100);uniqueIndex"`
	FullName               string     `json:"fullName" gorm:"type:varchar(100)"`
	Role                   string     `json:"role" gorm:"type:varchar(20);default:'user'"` // admin, user
	Status                 string     `json:"status" gorm:"type:varchar(20);default:'active';index"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty" gorm:"type:timestamp;index"`
	ExpirationWarningSent  bool       `json:"expirationWarningSent" gorm:"type:boolean;default:false"`
	AutoDisableOnExpiry    bool       `json:"autoDisableOnExpiry" gorm:"type:boolean;default:true"`
	LastLoginTime          *time.Time `json:"lastLoginTime" gorm:"type:timestamp"`
	LastLoginIP            string     `json:"lastLoginIp" gorm:"type:varchar(45)"`
	OrganizationID         *string    `json:"organizationId,omitempty" gorm:"type:varchar(36);index"` // 所属部门ID

	// 2FA相关字段
	TwoFactorEnabled     bool       `json:"twoFactorEnabled" gorm:"column:two_factor_enabled;type:boolean;default:false"`
	TwoFactorSecret      string     `json:"-" gorm:"column:two_factor_secret;type:varchar(255)"` // 2FA密钥，不在JSON中暴露
	TwoFactorBackupCodes string     `json:"-" gorm:"column:two_factor_backup_codes;type:text"`   // 备用码，不在JSON中暴露
	TwoFactorVerifiedAt  *time.Time `json:"twoFactorVerifiedAt,omitempty" gorm:"column:two_factor_verified_at;type:timestamp"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}

// PlatformLoginRecord 平台登录记录（登录到ZJump平台本身，不是连接虚拟机）
type PlatformLoginRecord struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID    string    `json:"userId" gorm:"type:varchar(36);not null;index"`
	Username  string    `json:"username" gorm:"type:varchar(50);not null"`
	LoginIP   string    `json:"loginIp" gorm:"type:varchar(45)"`
	UserAgent string    `json:"userAgent" gorm:"type:varchar(255)"`
	LoginTime time.Time `json:"loginTime" gorm:"type:timestamp;not null;index"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:'active';index"` // active, logged_out
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (PlatformLoginRecord) TableName() string {
	return "platform_login_records" // 使用独立的表，不与虚拟机登录记录混淆
}

// ==================================================================================
// DEPRECATED: 以下两个模型已废弃，新权限架构使用：
// User → UserGroup (user_groups) → PermissionRule (permission_rules) → (SystemUser + HostGroup)
// 保留这些模型是为了向后兼容，但建议在新系统中不再使用
// ==================================================================================

// UserGroupPermission 用户-主机分组权限关联 [DEPRECATED]
// Deprecated: 使用新的 UserGroup + PermissionRule 架构替代
type UserGroupPermission struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    string    `json:"userId" gorm:"type:varchar(36);not null;index"`
	GroupID   string    `json:"groupId" gorm:"type:varchar(36);not null;index"`
	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (UserGroupPermission) TableName() string {
	return "user_group_permissions"
}

// UserHostPermission 用户-主机权限关联（单个主机）[DEPRECATED]
// Deprecated: 使用新的 UserGroup + PermissionRule 架构替代
type UserHostPermission struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    string    `json:"userId" gorm:"type:varchar(36);not null;index"`
	HostID    string    `json:"hostId" gorm:"type:varchar(36);not null;index"`
	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (UserHostPermission) TableName() string {
	return "user_host_permissions"
}

// UserWithGroups 用户及其关联的分组
type UserWithGroups struct {
	User
	GroupIDs []string `json:"groupIds" gorm:"-"`
	HostIDs  []string `json:"hostIds" gorm:"-"` // 单独授权的主机ID列表
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// 2FA相关字段
	TwoFactorCode string `json:"twoFactorCode,omitempty"`
	BackupCode    string `json:"backupCode,omitempty"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
	// 2FA相关字段
	RequiresTwoFactor   bool `json:"requiresTwoFactor,omitempty"`
	TwoFactorEnabled    bool `json:"twoFactorEnabled,omitempty"`
	NeedsTwoFactorSetup bool `json:"needsTwoFactorSetup,omitempty"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"fullName"`
}

// PlatformLoginRecordsResponse 平台登录记录响应
type PlatformLoginRecordsResponse struct {
	Records []PlatformLoginRecord `json:"records"`
	Total   int64                 `json:"total"`
}

// UserExpirationLog 用户过期日志
type UserExpirationLog struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID       string     `json:"userId" gorm:"type:varchar(36);not null;index"`
	Username     string     `json:"username" gorm:"type:varchar(50);not null;index"`
	Action       string     `json:"action" gorm:"type:varchar(50);not null;index"` // warning_sent, expired, disabled, renewed
	ExpiresAt    *time.Time `json:"expiresAt" gorm:"type:timestamp"`
	NewExpiresAt *time.Time `json:"newExpiresAt" gorm:"type:timestamp"`
	Reason       string     `json:"reason" gorm:"type:text"`
	PerformedBy  string     `json:"performedBy" gorm:"type:varchar(36)"` // Admin user ID
	CreatedAt    time.Time  `json:"createdAt" gorm:"autoCreateTime;index"`
}

func (UserExpirationLog) TableName() string {
	return "user_expiration_logs"
}

// ExpirationNotificationConfig 过期通知配置
type ExpirationNotificationConfig struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Type                 string    `json:"type" gorm:"type:varchar(50);not null;uniqueIndex"` // user, permission
	WarningDays          int       `json:"warningDays" gorm:"type:int;not null;default:7"`
	Enabled              bool      `json:"enabled" gorm:"type:boolean;default:true"`
	NotificationChannels string    `json:"notificationChannels" gorm:"type:text"` // JSON array
	MessageTemplate      string    `json:"messageTemplate" gorm:"type:text"`
	CreatedAt            time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ExpirationNotificationConfig) TableName() string {
	return "expiration_notification_config"
}

// TwoFactorConfig 双因素认证全局配置
type TwoFactorConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Enabled   bool      `json:"enabled" gorm:"type:boolean;default:false"`       // 是否启用全局2FA
	Issuer    string    `json:"issuer" gorm:"type:varchar(100);default:'ZJump'"` // 2FA应用名称
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (TwoFactorConfig) TableName() string {
	return "two_factor_config"
}

// TwoFactorSetupRequest 2FA设置请求
type TwoFactorSetupRequest struct {
	Secret     string `json:"secret" binding:"required"`
	Code       string `json:"code" binding:"required"`
	BackupCode string `json:"backupCode,omitempty"`
}

// TwoFactorVerifyRequest 2FA验证请求
type TwoFactorVerifyRequest struct {
	Code       string `json:"code" binding:"required"`
	BackupCode string `json:"backupCode,omitempty"`
}

// TwoFactorSetupResponse 2FA设置响应
type TwoFactorSetupResponse struct {
	QRCode      string   `json:"qrCode"`      // 二维码数据URL
	Secret      string   `json:"secret"`      // 密钥（用于手动输入）
	BackupCodes []string `json:"backupCodes"` // 备用码
}

// TwoFactorStatus 2FA状态
type TwoFactorStatus struct {
	Enabled     bool       `json:"enabled"`
	VerifiedAt  *time.Time `json:"verifiedAt,omitempty"`
	BackupCodes []string   `json:"backupCodes,omitempty"`
}
