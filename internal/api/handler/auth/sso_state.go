package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

const defaultSSONextPath = "/"

type ssoStatePayload struct {
	Nonce string `json:"nonce"`
	Next  string `json:"next"`
}

// normalizeSSONextPath 仅允许站内相对路径，防止开放重定向（对齐 hashcheck oidc_login.normalize_next_path）
func normalizeSSONextPath(next string) string {
	if next == "" {
		return defaultSSONextPath
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return defaultSSONextPath
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return defaultSSONextPath
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return defaultSSONextPath
	}
	return next
}

func encodeSSOState(next string) (string, error) {
	n := normalizeSSONextPath(next)
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	payload := ssoStatePayload{
		Nonce: hex.EncodeToString(buf),
		Next:  n,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

var legacyUUIDState = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// decodeSSONextFromState 从 OAuth state 解析登录成功后的站内路径；无法解析时返回默认 "/"。
// 兼容旧版 initiate 返回的纯 UUID state（无 next 信息）。须先尝试 base64 JSON，避免与 base64url 中的 "-" 误判。
func decodeSSONextFromState(state string) string {
	if state == "" {
		return defaultSSONextPath
	}
	// 使用 RawURLEncoding 编码的输出通常无 padding；勿随意补「=」，否则会解码失败
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err == nil {
		var p ssoStatePayload
		if json.Unmarshal(raw, &p) == nil {
			return normalizeSSONextPath(p.Next)
		}
	}
	if legacyUUIDState.MatchString(state) {
		return defaultSSONextPath
	}
	return defaultSSONextPath
}

// redirectLoginWithSSOToken 重定向到登录页并附带 token；next 为登录成功后的站内路径。
// 必须经 /login 落地，否则根路径会被 PrivateRoute 重定向到 /login 而丢失 query 中的 sso_token。
func redirectLoginWithSSOToken(postLoginPath, token string) string {
	dest := normalizeSSONextPath(postLoginPath)
	u := &url.URL{Path: "/login"}
	q := url.Values{}
	q.Set("sso_token", token)
	q.Set("next", dest)
	u.RawQuery = q.Encode()
	return u.String()
}
