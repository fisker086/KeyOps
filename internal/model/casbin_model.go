package model

import (
	"time"
)

// CasbinModel Casbin模型配置
type CasbinModel struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Section   string    `json:"section" gorm:"type:varchar(50);not null;index"` // 配置段：request_definition, policy_definition, role_definition, policy_effect, matchers
	Key       string    `json:"key" gorm:"type:varchar(50);not null"`           // 配置键：r, p, g, e, m
	Value     string    `json:"value" gorm:"type:text;not null"`                // 配置值
	Sort      int       `json:"sort" gorm:"default:0;index"`                    // 排序（同一section内的顺序）
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (CasbinModel) TableName() string {
	return "casbin_models"
}
