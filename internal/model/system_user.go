package model

import "time"

// SystemUser 系统用户（目标主机上的操作系统用户）
type SystemUser struct {
	ID       string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name     string `json:"name" gorm:"type:varchar(100);not null"`           // 显示名称
	Username string `json:"username" gorm:"type:varchar(100);not null;index"` // OS用户名

	// 认证信息 (必须明确选择 password 或 key，不支持 auto)
	AuthType   string `json:"authType" gorm:"type:varchar(20);default:'password'"` // password, key
	Password   string `json:"password,omitempty" gorm:"type:text"`
	PrivateKey string `json:"privateKey,omitempty" gorm:"type:text"`
	Passphrase string `json:"passphrase,omitempty" gorm:"type:text"`

	// 协议和设置
	Protocol string `json:"protocol" gorm:"type:varchar(20);default:'ssh';index"` // ssh, rdp

	// 其他设置
	Priority    int    `json:"priority" gorm:"default:0;index"` // 优先级，数字越大越优先
	Description string `json:"description" gorm:"type:text"`
	Status      string `json:"status" gorm:"type:varchar(20);default:'active';index"` // active, inactive

	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (SystemUser) TableName() string {
	return "system_users"
}

// Role 角色（平台角色）
type Role struct {
	ID          string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name        string `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 显示相关
	Color    string `json:"color" gorm:"type:varchar(20)"`
	Icon     string `json:"icon" gorm:"type:varchar(50)"`
	Priority int    `json:"priority" gorm:"default:0;index"`

	Status    string    `json:"status" gorm:"type:varchar(20);default:'active';index"` // active, inactive
	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Role) TableName() string {
	return "roles"
}

// RoleMember 角色成员关系
type RoleMember struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	RoleID      string    `json:"roleId" gorm:"type:varchar(36);not null;index"`
	UserID      string    `json:"userId" gorm:"type:varchar(36);not null;index"`
	AddedBy     string    `json:"addedBy,omitempty" gorm:"type:varchar(36)"`
	AddedAt     time.Time `json:"addedAt" gorm:"autoCreateTime"`
}

func (RoleMember) TableName() string {
	return "role_members"
}

// PermissionRule 授权规则（角色 + 系统用户 + 主机组）
type PermissionRule struct {
	ID   string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name string `json:"name" gorm:"type:varchar(200);not null"`

	// 授权对象
	RoleID string `json:"roleId" gorm:"type:varchar(36);not null;index"`

	// 资产范围
	HostGroupID *string `json:"hostGroupId,omitempty" gorm:"type:varchar(36);index"` // NULL = 所有主机
	HostIDs     string  `json:"hostIds,omitempty" gorm:"type:text"`                   // JSON数组，指定主机ID列表

	// 系统用户
	SystemUserID *string `json:"systemUserId,omitempty" gorm:"type:varchar(36);index"`

	// 时间限制
	ValidFrom *time.Time `json:"validFrom,omitempty" gorm:"type:timestamp;index"`
	ValidTo   *time.Time `json:"validTo,omitempty" gorm:"type:timestamp;index"`

	// 状态
	Enabled     bool   `json:"enabled" gorm:"default:true;index"`
	Priority    int    `json:"priority" gorm:"default:0"`
	Description string `json:"description" gorm:"type:text"`

	CreatedBy string    `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (PermissionRule) TableName() string {
	return "permission_rules"
}

// SystemUserWithPermissions 带权限信息的系统用户（用于前端显示）
type SystemUserWithPermissions struct {
	SystemUser
	CanUse bool `json:"canUse" gorm:"-"` // 当前用户是否可以使用
}

// RoleWithMembers 带成员信息的角色
type RoleWithMembers struct {
	Role
	MemberCount int    `json:"memberCount" gorm:"column:member_count"`
	Members     []User `json:"members,omitempty" gorm:"-"`
}

// PermissionRuleDetail 授权规则详情（带关联信息）
type PermissionRuleDetail struct {
	PermissionRule
	RoleName        string       `json:"roleName" gorm:"-"`
	HostGroupName   string       `json:"hostGroupName" gorm:"-"`   // 已废弃，保留兼容性
	SystemUserName  string       `json:"systemUserName" gorm:"-"`  // 已废弃，保留兼容性
	SystemUsers     []SystemUser `json:"systemUsers" gorm:"-"`     // 多个系统用户
	HostGroups      []HostGroup  `json:"hostGroups" gorm:"-"`      // 多个主机组
	SystemUserIDs   []string     `json:"systemUserIds" gorm:"-"`   // 系统用户ID列表
	HostGroupIDs    []string     `json:"hostGroupIds" gorm:"-"`    // 主机组ID列表
	SystemUserNames []string     `json:"systemUserNames" gorm:"-"` // 系统用户名称列表
	HostGroupNames  []string     `json:"hostGroupNames" gorm:"-"`  // 主机组名称列表
}

// PermissionExpirationLog 授权过期日志
type PermissionExpirationLog struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	RuleID        string     `json:"ruleId" gorm:"type:varchar(36);not null;index"`
	RuleName      string     `json:"ruleName" gorm:"type:varchar(200);not null"`
	RoleID        string     `json:"roleId" gorm:"type:varchar(36);not null;index"`
	RoleName      string     `json:"roleName" gorm:"type:varchar(100)"`
	Action        string     `json:"action" gorm:"type:varchar(50);not null;index"` // warning_sent, expired, disabled, renewed
	ValidTo       *time.Time `json:"validTo" gorm:"type:timestamp"`
	NewValidTo    *time.Time `json:"newValidTo" gorm:"type:timestamp"`
	Reason        string     `json:"reason" gorm:"type:text"`
	PerformedBy   string     `json:"performedBy" gorm:"type:varchar(36)"` // Admin user ID
	CreatedAt     time.Time  `json:"createdAt" gorm:"autoCreateTime;index"`
}

func (PermissionExpirationLog) TableName() string {
	return "permission_expiration_logs"
}

// PermissionRuleSystemUser 授权规则-系统用户关联表（多对多）
type PermissionRuleSystemUser struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	PermissionRuleID string    `json:"permissionRuleId" gorm:"type:varchar(36);not null;index;uniqueIndex:idx_rule_system_user"`
	SystemUserID     string    `json:"systemUserId" gorm:"type:varchar(36);not null;index;uniqueIndex:idx_rule_system_user"`
	CreatedAt        time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (PermissionRuleSystemUser) TableName() string {
	return "permission_rule_system_users"
}

// PermissionRuleHostGroup 授权规则-主机组关联表（多对多）
type PermissionRuleHostGroup struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	PermissionRuleID string    `json:"permissionRuleId" gorm:"type:varchar(36);not null;index;uniqueIndex:idx_rule_host_group"`
	HostGroupID      string    `json:"hostGroupId" gorm:"type:varchar(36);not null;index;uniqueIndex:idx_rule_host_group"`
	CreatedAt        time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (PermissionRuleHostGroup) TableName() string {
	return "permission_rule_host_groups"
}

// PermissionRuleWithRelations 带关联数据的授权规则（用于API返回）
type PermissionRuleWithRelations struct {
	PermissionRule
	SystemUsers []SystemUser `json:"systemUsers" gorm:"-"`
	HostGroups  []HostGroup  `json:"hostGroups" gorm:"-"`
}
