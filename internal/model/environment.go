package model

import "time"

// Environment 平台环境管理（组织管理）
type Environment struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)" example:"env-dev"`
	Code        string    `json:"code" gorm:"type:varchar(50);uniqueIndex;not null" example:"dev"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null" example:"开发"`
	IsActive    bool      `json:"isActive" gorm:"column:is_active;type:boolean;default:true;index" example:"true"`
	SortOrder   int       `json:"sortOrder" gorm:"column:sort_order;type:int;default:0;index" example:"1"`
	Description string    `json:"description" gorm:"type:varchar(500)" example:"开发环境"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (Environment) TableName() string {
	return "environments"
}

type CreateEnvironmentRequest struct {
	Code        string `json:"code" binding:"required" example:"dev"`
	Name        string `json:"name" binding:"required" example:"开发"`
	IsActive    bool   `json:"isActive" example:"true"`
	SortOrder   int    `json:"sortOrder" example:"1"`
	Description string `json:"description" example:"开发环境"`
}

type UpdateEnvironmentRequest struct {
	Code        string `json:"code" example:"test"`
	Name        string `json:"name" example:"测试"`
	IsActive    *bool  `json:"isActive" example:"true"`
	SortOrder   *int   `json:"sortOrder" example:"2"`
	Description string `json:"description" example:"测试环境"`
}
