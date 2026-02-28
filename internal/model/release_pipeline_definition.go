package model

import "time"

// ReleasePipelineDefinition 发布流水线定义（用户可编辑的节点与连线，支持多条）
type ReleasePipelineDefinition struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string    `gorm:"type:varchar(255);default:''" json:"name"` // 展示名称，列表与应用-发布选择用
	Content   string    `gorm:"type:json;not null" json:"content"`         // JSON: { "nodes": [], "edges": [] }
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	UpdatedBy string    `gorm:"type:varchar(36)" json:"updated_by"`
}

// TableName 指定表名
func (ReleasePipelineDefinition) TableName() string {
	return "release_pipeline_definitions"
}

const ReleasePipelineDefinitionIDDefault = "default"
