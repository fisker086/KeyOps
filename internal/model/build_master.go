package model

import "time"

// BuildMasterType 发版类型：0 常规发版，1 紧急发版
const (
	BuildMasterTypeNormal  = 0
	BuildMasterTypeUrgent  = 1
)

// BuildMasterStatus 发版状态：0 创建 1 填写截止 2 审核 3 发版 4 完成
const (
	BuildMasterStatusCreated     = 0
	BuildMasterStatusFilling     = 1
	BuildMasterStatusApproving   = 2
	BuildMasterStatusReleasing   = 3
	BuildMasterStatusCompleted   = 4
)

// BuildMasterList
type BuildMasterList struct {
	ID           string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	PublishDate  string    `gorm:"type:date;not null;index:idx_build_master_date_type" json:"publish_date"` // 发版日期 YYYY-MM-DD
	Type         int       `gorm:"not null;index:idx_build_master_date_type" json:"type"`                 // 0 常规 1 紧急
	Status       int       `gorm:"not null;default:0" json:"status"`                                       // 0-4
	OrderNum     int       `gorm:"not null;default:1" json:"order"`                                         // 第几弹
	OrderDescribe string   `gorm:"type:varchar(32)" json:"order_describe,omitempty"`                       // 自定义弹名
	OwnerID      string    `gorm:"type:varchar(36);index" json:"owner_id"`
	OwnerName    string    `gorm:"type:varchar(100)" json:"owner_name,omitempty"`
	Hurried      int       `gorm:"default:0" json:"hurried"` // 催一下次数 0/1/2/3
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BuildMasterList) TableName() string {
	return "build_master_lists"
}

// BuildMasterOperationLog
type BuildMasterOperationLog struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ListID       string    `gorm:"type:varchar(36);not null;index:idx_bm_log_list" json:"list_id"`
	OperatorID   string    `gorm:"type:varchar(36)" json:"operator_id"`
	OperatorName string    `gorm:"type:varchar(100)" json:"operator_name"`
	Method       string    `gorm:"type:varchar(32);not null" json:"method"` // create | update
	Body         string    `gorm:"type:text" json:"body"`                   // JSON: [{"name":"status","old":"0","new":"1"},...]
	CreatedAt    time.Time `json:"created_at"`
}

func (BuildMasterOperationLog) TableName() string {
	return "build_master_operation_logs"
}
