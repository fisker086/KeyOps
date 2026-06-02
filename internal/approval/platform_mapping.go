package approval

import "github.com/fisker086/keyops/internal/model"

// PlatformMapping 审批平台完整信息映射
// 新增审批平台时只需在此处添加一条记录
type PlatformMapping struct {
	Type           model.ApprovalPlatform `json:"type"`
	DisplayName    string                 `json:"display_name"`
	DefaultBaseURL string                 `json:"default_base_url"`
}

// SupportedPlatforms 所有支持的审批平台完整映射
// 设计用于审批配置、设置页面、校验等所有需要平台信息的场景
var SupportedPlatforms = []PlatformMapping{
	{
		Type:           model.ApprovalPlatformFeishu,
		DisplayName:    "飞书",
		DefaultBaseURL: "https://open.feishu.cn/open-apis",
	},
	{
		Type:           model.ApprovalPlatformLark,
		DisplayName:    "Lark",
		DefaultBaseURL: "https://open.larksuite.com/open-apis",
	},
	{
		Type:           model.ApprovalPlatformDingTalk,
		DisplayName:    "钉钉",
		DefaultBaseURL: "https://oapi.dingtalk.com",
	},
	{
		Type:           model.ApprovalPlatformWeChat,
		DisplayName:    "企业微信",
		DefaultBaseURL: "https://qyapi.weixin.qq.com",
	},
	{
		Type:           model.ApprovalPlatformCustom,
		DisplayName:    "自定义",
		DefaultBaseURL: "",
	},
}

// GetPlatformMapping 获取平台映射信息
func GetPlatformMapping(pt model.ApprovalPlatform) *PlatformMapping {
	for _, p := range SupportedPlatforms {
		if p.Type == pt {
			return &p
		}
	}
	return nil
}

// IsSupportedPlatform 判断是否支持的审批平台
func IsSupportedPlatform(pt model.ApprovalPlatform) bool {
	return GetPlatformMapping(pt) != nil
}

// ListPlatformTypes 列出所有支持的平台类型
func ListPlatformTypes() []model.ApprovalPlatform {
	types := make([]model.ApprovalPlatform, 0, len(SupportedPlatforms))
	for _, p := range SupportedPlatforms {
		types = append(types, p.Type)
	}
	return types
}
