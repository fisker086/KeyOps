package config

import (
	"os"
	"strings"
)

// SecurityAuthMethodOverride 非空时，应覆盖数据库 settings 中的 authMethod。
// 优先级：环境变量 AUTH_METHOD > config.yaml 的 security.auth_method。
// 用于紧急恢复（例如数据库里误配成 sso 导致无法登录）；两者皆空则完全以数据库为准。
func SecurityAuthMethodOverride() string {
	if GlobalConfig == nil {
		return ""
	}
	if v := strings.TrimSpace(os.Getenv("AUTH_METHOD")); v != "" {
		return normalizeAuthMethod(v)
	}
	if v := strings.TrimSpace(GlobalConfig.Security.AuthMethod); v != "" {
		return normalizeAuthMethod(v)
	}
	return ""
}

func normalizeAuthMethod(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "password", "ldap", "sso":
		return s
	default:
		return ""
	}
}

// NormalizeAuthMethod 将配置或环境变量中的认证方式规范为 password / ldap / sso，非法值返回空字符串。
func NormalizeAuthMethod(s string) string {
	return normalizeAuthMethod(s)
}
