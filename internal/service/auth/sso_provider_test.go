package auth

import "testing"

func TestNormalizeSSOProvider(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// 标准值
		{"", SSOProviderOIDC},
		{"OIDC", SSOProviderOIDC},
		{"oidc", SSOProviderOIDC},
		{"generic", SSOProviderOIDC},
		{"oauth2", SSOProviderOIDC},
		{"标准", SSOProviderOIDC},
		{"通用", SSOProviderOIDC},
		// 飞书
		{"feishu", SSOProviderFeishu},
		{"飞书", SSOProviderFeishu},
		{"FEISHU", SSOProviderFeishu},
		// Lark
		{"lark", SSOProviderLark},
		{"LARK", SSOProviderLark},
		// 钉钉
		{"dingtalk", SSOProviderDingTalk},
		{"钉钉", SSOProviderDingTalk},
		{"ding", SSOProviderDingTalk},
		{"DingTalk", SSOProviderDingTalk},
		// 企业微信
		{"wecom", SSOProviderWeCom},
		{"wework", SSOProviderWeCom},
		{"workweixin", SSOProviderWeCom},
		{"企业微信", SSOProviderWeCom},
		{"wxwork", SSOProviderWeCom},
		{"WeCom", SSOProviderWeCom},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeSSOProvider(tc.in); got != tc.want {
				t.Errorf("normalizeSSOProvider(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsFeishuFamily(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"feishu", true},
		{"lark", true},
		{"oidc", false},
		{"dingtalk", false},
		{"wecom", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := isFeishuFamily(tc.in); got != tc.want {
				t.Errorf("isFeishuFamily(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"", "", ""},
		{"a", "", "a"},
		{"", "b", "b"},
		{"a", "b", "a"},
	}
	for _, tc := range tests {
		t.Run(tc.a+"|"+tc.b, func(t *testing.T) {
			if got := firstNonEmpty(tc.a, tc.b); got != tc.want {
				t.Errorf("firstNonEmpty(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
