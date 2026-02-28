package model

import (
	"time"
)

// ChannelTemplate 渠道模板表
type ChannelTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TemplateID *uint    `gorm:"index" json:"template_id,omitempty"`
	ChannelID  *uint    `gorm:"index" json:"channel_id,omitempty"`
	Content    string   `gorm:"type:text" json:"content"`
	Finished   bool     `gorm:"default:false" json:"finished"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ChannelTemplate) TableName() string {
	return "channel_templates"
}

