package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// BuildMasterType 发版类型：0 常规发版，1 紧急发版
const (
	BuildMasterTypeNormal = 0
	BuildMasterTypeUrgent = 1
)

// BuildMasterStatus 发版状态：0 创建 1 填写截止 2 审核 3 发版 4 完成
const (
	BuildMasterStatusCreated   = 0
	BuildMasterStatusFilling   = 1
	BuildMasterStatusApproving = 2
	BuildMasterStatusReleasing = 3
	BuildMasterStatusCompleted = 4
)

// BuildMasterDeployStatus 部署状态
const (
	BuildMasterDeployPending   = "pending"
	BuildMasterDeployDeploying = "deploying"
	BuildMasterDeploySuccess   = "success"
	BuildMasterDeployFailed    = "failed"
	BuildMasterDeployRollback  = "rollback"
)

// BuildMasterList
type BuildMasterList struct {
	ID               string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	PublishDate      string    `gorm:"type:date;not null;index:idx_build_master_date_type_site" json:"publish_date"`           // 发版日期 YYYY-MM-DD
	Site             string    `gorm:"type:varchar(50);not null;default:'';index:idx_build_master_date_type_site" json:"site"` // 应用站点，与 applications.site 一致
	Type             int       `gorm:"not null;index:idx_build_master_date_type_site" json:"type"`                             // 0 常规 1 紧急
	Status           int       `gorm:"not null;default:0" json:"status"`                                                       // 0-4
	OrderNum         int       `gorm:"not null;default:1" json:"order"`                                                        // 第几弹
	OrderDescribe    string    `gorm:"type:varchar(32)" json:"order_describe,omitempty"`                                       // 自定义弹名
	OwnerID          string    `gorm:"type:varchar(36);index" json:"owner_id"`
	OwnerName        string    `gorm:"type:varchar(100)" json:"owner_name,omitempty"`
	Hurried          int       `gorm:"default:0" json:"hurried"`                                   // 催一下次数 0/1/2/3
	ApprovalConfigID string    `gorm:"type:varchar(36)" json:"approval_config_id,omitempty"`       // 第三方审批配置ID
	ApprovalPlatform string    `gorm:"type:varchar(20)" json:"approval_platform,omitempty"`        // feishu/dingtalk/wechat
	ApprovalInstance string    `gorm:"type:varchar(64)" json:"approval_instance,omitempty"`        // 第三方审批实例 code
	ReleaseRunID     string    `gorm:"type:varchar(36)" json:"release_run_id,omitempty"`           // 关联的部署记录ID
	DeployStatus     string    `gorm:"type:varchar(20);default:''" json:"deploy_status,omitempty"` // 部署状态: pending/deploying/success/failed/rollback
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BuildMasterList) TableName() string {
	return "build_master_lists"
}

func normalizePublishDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func (l *BuildMasterList) BeforeSave(_ *gorm.DB) error {
	l.PublishDate = normalizePublishDate(l.PublishDate)
	return nil
}

func (l *BuildMasterList) AfterFind(_ *gorm.DB) error {
	l.PublishDate = normalizePublishDate(l.PublishDate)
	return nil
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

// BuildMasterItemStatus 条目状态
const (
	BuildMasterItemStatusUndone    = 0 // 未完成
	BuildMasterItemStatusDone      = 1 // 已完成
	BuildMasterItemStatusCancelled = 2 // 已取消
	BuildMasterItemStatusRollback  = 3 // 已回滚
)

// BuildMasterItem 发版工作项分类（如：前端、后端）
type BuildMasterItem struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ListID    string    `gorm:"type:varchar(36);not null;index:idx_bm_item_list" json:"list_id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	OrderNum  int       `gorm:"not null;default:0" json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BuildMasterItem) TableName() string {
	return "build_master_items"
}

// BuildMasterItemDetail 发版条目详情
type BuildMasterItemDetail struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ListID   string `gorm:"type:varchar(36);not null;index:idx_bm_detail_list" json:"list_id"`
	ItemID   uint   `gorm:"not null;index:idx_bm_detail_item" json:"item_id"`
	AppName  string `gorm:"type:varchar(100)" json:"app_name"`   // 操作项
	Operate  string `gorm:"type:varchar(100)" json:"operate"`    // 子类型
	SubType  string `gorm:"type:varchar(50)" json:"sub_type"`    // 操作类型（新增/修改）
	Tag      string `gorm:"type:varchar(100)" json:"tag"`        // 版本
	Content  string `gorm:"type:text" json:"content,omitempty"`  // 操作内容
	Note     string `gorm:"type:text" json:"note,omitempty"`     // 注意事项
	Rollback string `gorm:"type:text" json:"rollback,omitempty"` // 回滚步骤
	Record   string `gorm:"type:text" json:"record,omitempty"`   // 执行记录
	Status   int    `gorm:"not null;default:0" json:"status"`    // 0=未完成 1=已完成 2=已取消 3=已回滚

	OrderNum  int       `gorm:"not null;default:0" json:"order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BuildMasterItemDetail) TableName() string {
	return "build_master_item_details"
}

// BuildMasterApprovalResult 审批结果
const (
	BuildMasterApprovalPending  = "pending"
	BuildMasterApprovalApproved = "approved"
	BuildMasterApprovalRejected = "rejected"
)

// BuildMasterApproval 审批记录
type BuildMasterApproval struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ListID       string    `gorm:"type:varchar(36);not null;index:idx_bm_approval_list" json:"list_id"`
	ApproverID   string    `gorm:"type:varchar(36)" json:"approver_id"`
	ApproverName string    `gorm:"type:varchar(100)" json:"approver_name"`
	Result       string    `gorm:"type:varchar(20);not null;default:pending" json:"result"`
	Comment      string    `gorm:"type:text" json:"comment,omitempty"`
	OrderNum     int       `gorm:"not null;default:0" json:"order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (BuildMasterApproval) TableName() string {
	return "build_master_approvals"
}

// BuildMasterCheckItem 工作项模板（复用历史 checkList_checkitem 表）
type BuildMasterCheckItem struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name     string `gorm:"type:varchar(100);not null" json:"name"`
	NeedApps bool   `gorm:"column:need_apps;default:false" json:"need_apps"`
}

func (BuildMasterCheckItem) TableName() string {
	return "checkList_checkitem"
}
