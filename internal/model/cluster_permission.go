package model

import (
	"time"
	"gorm.io/datatypes"
)

// ClusterPermission K8s集群权限表
type ClusterPermission struct {
	ID             string         `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ClusterID      string         `json:"clusterId" gorm:"type:varchar(36);not null;index"`
	UserID         *string        `json:"userId,omitempty" gorm:"type:varchar(36);index"` // 用户ID（如果为用户级权限）
	RoleID         *string        `json:"roleId,omitempty" gorm:"type:varchar(36);index"` // 角色ID（如果为角色级权限）
	PermissionType string         `json:"permissionType" gorm:"type:varchar(20);default:'read';index"` // 权限类型: read, write, admin
	AllowedNamespaces datatypes.JSON `json:"allowedNamespaces,omitempty" gorm:"type:text"` // 允许的命名空间（JSON数组，NULL表示所有命名空间）
	CreatedBy      string         `json:"createdBy,omitempty" gorm:"type:varchar(36)"`
	CreatedAt      time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ClusterPermission) TableName() string {
	return "cluster_permissions"
}

