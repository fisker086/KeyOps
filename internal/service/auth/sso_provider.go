package auth

import (
	"strings"
)

// 内置 provider 取值（与前端下拉 value 一致）
const (
	SSOProviderOIDC     = "oidc"
	SSOProviderFeishu   = "feishu"
	SSOProviderLark     = "lark"
	SSOProviderDingTalk = "dingtalk"
	SSOProviderWeCom    = "wecom"
)

// normalizeSSOProvider 将界面/旧数据中的名称规范为内置 key
func normalizeSSOProvider(p string) string {
	p = strings.TrimSpace(strings.ToLower(p))
	switch p {
	case "", "generic", "oauth2", "oauth", "标准", "通用", "oidc":
		return SSOProviderOIDC
	case "feishu", "飞书":
		return SSOProviderFeishu
	case "lark":
		return SSOProviderLark
	case "dingtalk", "钉钉", "ding":
		return SSOProviderDingTalk
	case "wecom", "wework", "workweixin", "企业微信", "wxwork":
		return SSOProviderWeCom
	default:
		if strings.Contains(p, "feishu") || strings.Contains(p, "lark") {
			if strings.Contains(p, "lark") && !strings.Contains(p, "feishu") {
				return SSOProviderLark
			}
			return SSOProviderFeishu
		}
		if strings.Contains(p, "ding") {
			return SSOProviderDingTalk
		}
		if strings.Contains(p, "wecom") || strings.Contains(p, "weixin") || strings.Contains(p, "wework") {
			return SSOProviderWeCom
		}
		return SSOProviderOIDC
	}
}

func isFeishuFamily(p string) bool {
	switch normalizeSSOProvider(p) {
	case SSOProviderFeishu, SSOProviderLark:
		return true
	default:
		return false
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}