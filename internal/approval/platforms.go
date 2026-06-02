package approval

import "github.com/fisker086/keyops/internal/model"

// PlatformInfo 审批平台定义（保持向后兼容）
type PlatformInfo struct {
	Type           model.ApprovalPlatform `json:"type"`
	DefaultBaseURL string                 `json:"default_base_url"`
}

// SupportedApprovalPlatforms 所有支持的审批平台（保持向后兼容）
// 新增平台请优先在 platform_mapping.go 中添加
var SupportedApprovalPlatforms []PlatformInfo

func init() {
	for _, p := range SupportedPlatforms {
		SupportedApprovalPlatforms = append(SupportedApprovalPlatforms, PlatformInfo{
			Type:           p.Type,
			DefaultBaseURL: p.DefaultBaseURL,
		})
	}
}

// DefaultBaseURLForPlatform 获取平台默认 API 基础地址
func DefaultBaseURLForPlatform(pt model.ApprovalPlatform) string {
	m := GetPlatformMapping(pt)
	if m != nil {
		return m.DefaultBaseURL
	}
	return ""
}
