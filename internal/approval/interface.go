package approval

import (
	"context"

	"github.com/fisker086/keyops/internal/model"
)

// Provider 第三方审批平台接口
type Provider interface {
	// GetName 获取平台名称
	GetName() string

	// CreateApproval 创建审批单
	CreateApproval(ctx context.Context, approval *model.Approval) (externalID string, err error)

	// GetApprovalStatus 获取审批单状态
	GetApprovalStatus(ctx context.Context, externalID string) (*ApprovalStatusResponse, error)

	// CancelApproval 取消审批单
	CancelApproval(ctx context.Context, externalID string) error

	// HandleCallback 处理审批回调
	HandleCallback(ctx context.Context, data interface{}) (*CallbackResult, error)

	// ValidateConfig 验证配置
	ValidateConfig(config map[string]interface{}) error

	// CreateApprovalWithFormData 使用预构建的表单数据创建审批单
	// approvalCode: 审批代码（如果为空则使用配置中的审批代码）
	// formData: 预构建的表单数据（JSON数组字符串）
	// approval: 审批基本信息
	CreateApprovalWithFormData(ctx context.Context, approvalCode string, formData string, approval *model.Approval) (externalID string, err error)

	// GetApprovalFormDetail 获取审批表单详情（通过审批代码）
	// 返回表单字段列表，包含 id, type, name 等信息
	GetApprovalFormDetail(ctx context.Context, approvalCode string) ([]map[string]interface{}, error)
}

// ApprovalStatusResponse 审批状态响应
type ApprovalStatusResponse struct {
	Status       model.ApprovalStatus `json:"status"`
	ApproverName string               `json:"approver_name"`
	ApprovedAt   string               `json:"approved_at"`
	Comment      string               `json:"comment"`
}

// CallbackResult 回调处理结果
type CallbackResult struct {
	ApprovalID   string               `json:"approval_id"`
	Status       model.ApprovalStatus `json:"status"`
	ApproverName string               `json:"approver_name"`
	Comment      string               `json:"comment"`
}

// Factory 审批平台工厂
type Factory struct {
	providers map[model.ApprovalPlatform]Provider
}

// NewFactory 创建工厂
func NewFactory() *Factory {
	return &Factory{
		providers: make(map[model.ApprovalPlatform]Provider),
	}
}

// Register 注册审批平台
func (f *Factory) Register(platform model.ApprovalPlatform, provider Provider) {
	f.providers[platform] = provider
}

// GetProvider 获取审批平台
func (f *Factory) GetProvider(platform model.ApprovalPlatform) (Provider, bool) {
	provider, ok := f.providers[platform]
	return provider, ok
}

// ListProviders 列出所有注册的平台
func (f *Factory) ListProviders() []string {
	platforms := make([]string, 0, len(f.providers))
	for platform := range f.providers {
		platforms = append(platforms, string(platform))
	}
	return platforms
}
