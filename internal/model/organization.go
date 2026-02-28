package model

import (
	"time"
)

// Organization 组织架构模型
type Organization struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UnitCode    string     `json:"unitCode" gorm:"type:varchar(100);uniqueIndex;not null"` // 组织标识符
	UnitName    string     `json:"unitName" gorm:"type:varchar(255);not null"`             // 组织名称
	UnitType    string     `json:"unitType" gorm:"type:varchar(50);not null;index"`        // 组织类型：BizGroup、LineOfBiz、Site、Department
	UnitOwner   string     `json:"unitOwner" gorm:"type:varchar(255)"`                      // 组织负责人
	IsActive    bool       `json:"isActive" gorm:"type:boolean;default:true;index"`        // 是否启用
	ParentID    *string    `json:"parentId" gorm:"type:varchar(36);index"`                  // 父级组织ID（自引用，NULL表示顶级组织）
	SortOrder   int        `json:"sortOrder" gorm:"type:int;default:0;index"`              // 排序顺序
	Description string     `json:"description" gorm:"type:text"`                          // 组织描述
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	Children    []Organization `json:"children,omitempty" gorm:"-"`                         // 子组织（不存储）
}

func (Organization) TableName() string {
	return "organizations"
}

// CreateOrganizationRequest 创建组织请求
type CreateOrganizationRequest struct {
	UnitCode    string  `json:"unitCode" binding:"required"`
	UnitName    string  `json:"unitName" binding:"required"`
	UnitType    string  `json:"unitType" binding:"required"`
	UnitOwner   string  `json:"unitOwner"`
	IsActive    bool    `json:"isActive"`
	ParentID    *string `json:"parentId"`
	SortOrder   int     `json:"sortOrder"`
	Description string  `json:"description"`
}

// UpdateOrganizationRequest 更新组织请求
type UpdateOrganizationRequest struct {
	UnitName    string  `json:"unitName" binding:"required"`
	UnitType    string  `json:"unitType" binding:"required"`
	UnitOwner   string  `json:"unitOwner"`
	IsActive    bool    `json:"isActive"`
	ParentID    *string `json:"parentId"`
	SortOrder   int     `json:"sortOrder"`
	Description string  `json:"description"`
}

