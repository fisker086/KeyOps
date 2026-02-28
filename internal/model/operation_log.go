package model

import (
	"time"
)

// OperationLog 操作日志模型
type OperationLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string    `gorm:"type:varchar(100);not null" json:"username"`
	IP        string    `gorm:"type:varchar(50);not null" json:"ip"`
	Method    string    `gorm:"type:varchar(10);not null" json:"method"`
	Path      string    `gorm:"type:varchar(255);not null" json:"path"`
	Desc      string    `gorm:"type:varchar(255)" json:"desc"`
	Status    int       `gorm:"not null" json:"status"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	TimeCost  int64     `gorm:"type:bigint" json:"time_cost"`
	UserAgent string    `gorm:"type:varchar(500)" json:"user_agent"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (OperationLog) TableName() string {
	return "operation_logs"
}

