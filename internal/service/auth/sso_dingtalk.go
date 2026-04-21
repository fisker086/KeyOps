package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultDingTalkTokenURL = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
const defaultDingTalkUserInfoURL = "https://api.dingtalk.com/v1.0/contact/users/me"

func (s *AuthService) exchangeDingTalkToken(code, clientID, clientSecret, tokenURL string) (string, error) {
	if tokenURL == "" {
		tokenURL = defaultDingTalkTokenURL
	}
	body := map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"code":         code,
		"grantType":   "authorization_code",
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("钉钉换票失败 (HTTP %d)", resp.StatusCode)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", fmt.Errorf("解析钉钉 token 响应失败: %w", err)
	}
	var tok string
	switch v := generic["accessToken"].(type) {
	case string:
		tok = v
	}
	if tok == "" {
		if v, ok := generic["access_token"].(string); ok {
			tok = v
		}
	}
	if tok == "" {
		return "", errors.New("钉钉响应中无 accessToken")
	}
	return tok, nil
}

func (s *AuthService) getDingTalkUserInfo(userAccessToken, userInfoURL string) (*SSOUserInfo, error) {
	if userInfoURL == "" {
		userInfoURL = defaultDingTalkUserInfoURL
	}
	req, err := http.NewRequest(http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-acs-dingtalk-access-token", userAccessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("钉钉用户信息 HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Nick      string `json:"nick"`
		Name     string `json:"name"`
		UnionId  string `json:"unionId"`
		OpenId   string `json:"openId"`
		Email   string `json:"email"`
		Mobile  string `json:"mobile"`
		Avatar  string `json:"avatar"`
		AvatarUrl string `json:"avatarUrl"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析钉钉用户信息失败: %w", err)
	}
	name := firstNonEmpty(payload.Nick, payload.Name)
	ui := &SSOUserInfo{
		Sub:       firstNonEmpty(payload.UnionId, payload.OpenId),
		UnionID:   payload.UnionId,
		OpenID:    payload.OpenId,
		Email:     payload.Email,
		Name:      name,
		Mobile:   payload.Mobile,
		AvatarURL: firstNonEmpty(payload.Avatar, payload.AvatarUrl),
	}
	if ui.Email != "" {
		ui.Username = strings.Split(ui.Email, "@")[0]
	} else if ui.UnionID != "" {
		ui.Username = "dingtalk_" + ui.UnionID
	} else if ui.OpenID != "" {
		ui.Username = "dingtalk_" + ui.OpenID
	} else {
		ui.Username = "sso_" + uuid.New().String()[:8]
	}
	return ui, nil
}