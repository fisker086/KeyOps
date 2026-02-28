package model

import (
	"time"
)

// ApprovalStatus 审批状态
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"  // 待审批
	ApprovalStatusApproved ApprovalStatus = "approved" // 已批准
	ApprovalStatusRejected ApprovalStatus = "rejected" // 已拒绝
	ApprovalStatusCanceled ApprovalStatus = "canceled" // 已取消
	ApprovalStatusExpired  ApprovalStatus = "expired"  // 已过期
)

// ApprovalType 审批类型
type ApprovalType string

const (
	ApprovalTypeHostAccess      ApprovalType = "host_access"       // 主机访问权限
	ApprovalTypeHostGroupAccess ApprovalType = "host_group_access" // 主机组访问权限
	ApprovalTypeDeployment      ApprovalType = "deployment"        // 发布审批
)

// ApprovalPlatform 审批平台（仅支持第三方平台）
type ApprovalPlatform string

const (
	ApprovalPlatformInternal ApprovalPlatform = "internal" // 内部审批系统
	ApprovalPlatformFeishu   ApprovalPlatform = "feishu"   // 飞书
	ApprovalPlatformDingTalk ApprovalPlatform = "dingtalk" // 钉钉
	ApprovalPlatformWeChat   ApprovalPlatform = "wechat"   // 企业微信
	ApprovalPlatformCustom   ApprovalPlatform = "custom"   // 自定义
)

// Approval 审批工单模型
type Approval struct {
	ID          string           `json:"id" gorm:"primaryKey"`
	Title       string           `json:"title" gorm:"not null"`                  // 标题
	Description string           `json:"description"`                            // 描述
	Type        ApprovalType     `json:"type" gorm:"not null"`                   // 类型
	Status      ApprovalStatus   `json:"status" gorm:"default:pending;not null"` // 状态
	Platform    ApprovalPlatform `json:"platform" gorm:"default:internal"`       // 审批平台

	// 申请人信息
	ApplicantID    string `json:"applicant_id" gorm:"not null"` // 申请人ID
	ApplicantName  string `json:"applicant_name"`               // 申请人名称
	ApplicantEmail string `json:"applicant_email"`              // 申请人邮箱

	// 审批人信息
	ApproverIDs     StringArray `json:"approver_ids" gorm:"type:text"`   // 审批人ID列表
	ApproverNames   StringArray `json:"approver_names" gorm:"type:text"` // 审批人名称列表
	CurrentApprover string      `json:"current_approver"`                // 当前审批人

	// 资源信息
	ResourceType  string      `json:"resource_type"`                   // 资源类型 (host/host_group)
	ResourceIDs   StringArray `json:"resource_ids" gorm:"type:text"`   // 资源ID列表
	ResourceNames StringArray `json:"resource_names" gorm:"type:text"` // 资源名称列表

	// 权限信息
	Permissions StringArray `json:"permissions" gorm:"type:text"` // 权限列表
	Duration    int         `json:"duration"`                     // 权限时长(小时)
	ExpiresAt   *time.Time  `json:"expires_at"`                   // 过期时间

	// 审批详情
	Reason       string `json:"reason"`        // 申请理由
	ApprovalNote string `json:"approval_note"` // 审批备注
	RejectReason string `json:"reject_reason"` // 拒绝原因
	Priority     string `json:"priority" gorm:"type:varchar(20);default:normal"` // 优先级: low/normal/high/urgent

	// 第三方平台信息
	ExternalID   string `json:"external_id"`                    // 第三方平台审批单ID
	ExternalURL  string `json:"external_url"`                   // 第三方平台审批单链接
	ExternalData string `json:"external_data" gorm:"type:text"` // 第三方平台额外数据(JSON)
	
	// 发布相关字段（当Type为deployment时使用）
	DeployConfig string `json:"deploy_config" gorm:"type:text"` // 发布配置(JSON格式，存储Jenkins等发布方式的配置)
	DeploymentID string `json:"deployment_id"`                  // 关联的部署记录ID（发布成功后创建）
	Deployed     bool   `json:"deployed" gorm:"default:false"`  // 是否已发布

	// 时间信息
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ApprovedAt *time.Time `json:"approved_at"` // 批准时间
	RejectedAt *time.Time `json:"rejected_at"` // 拒绝时间

	// 关联
	Applicant *User `json:"applicant,omitempty" gorm:"foreignKey:ApplicantID"`
}

// TableName 指定表名
func (Approval) TableName() string {
	return "approvals"
}

// ApprovalComment 审批评论/历史记录
type ApprovalComment struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	ApprovalID string    `json:"approval_id" gorm:"not null;index"`
	UserID     string    `json:"user_id" gorm:"not null"`
	UserName   string    `json:"user_name"`
	Action     string    `json:"action"` // submit/approve/reject/comment/cancel
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`

	// 关联
	Approval *Approval `json:"approval,omitempty" gorm:"foreignKey:ApprovalID"`
}

// TableName 指定表名
func (ApprovalComment) TableName() string {
	return "approval_comments"
}

// ApprovalConfig 第三方审批平台配置
type ApprovalConfig struct {
	ID      string `json:"id" gorm:"primaryKey"`
	Name    string `json:"name" gorm:"not null"`         // 配置名称
	Type    string `json:"type" gorm:"not null"`         // 平台类型: feishu, dingtalk, wechat
	Enabled bool   `json:"enabled" gorm:"default:false"` // 是否启用

	// 应用凭证
	AppID     string `json:"app_id" gorm:"column:app_id;not null"`         // 应用ID或AppKey
	AppSecret string `json:"app_secret" gorm:"column:app_secret;not null"` // 应用密钥

	// 平台特定字段
	ApprovalCode string `json:"approval_code" gorm:"column:approval_code"` // 飞书审批定义code
	ProcessCode  string `json:"process_code" gorm:"column:process_code"`   // 钉钉流程code
	TemplateID   string `json:"template_id" gorm:"column:template_id"`     // 企业微信模板ID

	// 表单字段映射
	FormFields string `json:"form_fields" gorm:"column:form_fields;type:text"` // 表单字段映射JSON

	// 审批人配置
	ApproverUserIDs string `json:"approver_user_ids" gorm:"column:approver_user_ids;type:text"` // 审批人用户ID列表(JSON)

	// API配置
	APIBaseURL    string `json:"api_base_url" gorm:"column:api_base_url"`       // API基础URL，用户自定义填写
	APIPath       string `json:"api_path" gorm:"column:api_path"`               // API调用路径（创建审批）
	APIPathGet    string `json:"api_path_get" gorm:"column:api_path_get"`       // 获取审批API路径
	APIPathCancel string `json:"api_path_cancel" gorm:"column:api_path_cancel"` // 取消审批API路径

	// 回调配置
	CallbackURL string `json:"callback_url" gorm:"column:callback_url"` // 回调URL

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (ApprovalConfig) TableName() string {
	return "approval_configs"
}
