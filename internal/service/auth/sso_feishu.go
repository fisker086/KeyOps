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

const defaultFeishuTokenURL = "https://open.feishu.cn/open-apis/authen/v1/authorize_access_token"
const defaultFeishuUserInfoURL = "https://open.feishu.cn/open-apis/authen/v1/user_info"

type FeishuTokenResponse struct {
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
	Data *FeishuTokenData `json:"data"`
}

type FeishuTokenData struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Scope            string `json:"scope"`
}

type FeishuUserInfoResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data *FeishuUserData `json:"data"`
}

type FeishuUserData struct {
	Sub         string `json:"sub"`
	Name        string `json:"name"`
	Picture     string `json:"picture"`
	OpenID      string `json:"open_id"`
	UnionID     string `json:"union_id"`
	EnName      string `json:"en_name"`
	TenantKey   string `json:"tenant_key"`
	AvatarURL   string `json:"avatar_url"`
	AvatarThumb string `json:"avatar_thumb"`
	AvatarBig   string `json:"avatar_big"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
}

func (s *AuthService) exchangeFeishuToken(code, appID, appSecret, tokenURL string) (string, error) {
	if tokenURL == "" {
		tokenURL = defaultFeishuTokenURL
	}
	requestBody := map[string]interface{}{
		"grant_type": "authorization_code",
		"code":       code,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("构造请求体失败: %w", err)
	}

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodeBasicAuth(appID, appSecret)))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var tokenResp FeishuTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析飞书token响应失败: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("飞书返回错误 (code: %d): %s", tokenResp.Code, tokenResp.Msg)
	}

	if tokenResp.Data == nil || tokenResp.Data.AccessToken == "" {
		return "", errors.New("飞书响应中未包含access_token")
	}

	return tokenResp.Data.AccessToken, nil
}

func (s *AuthService) getFeishuUserInfo(accessToken, userInfoURL string) (*SSOUserInfo, error) {
	if userInfoURL == "" {
		userInfoURL = defaultFeishuUserInfoURL
	}
	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创��请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var userInfoResp FeishuUserInfoResponse
	if err := json.Unmarshal(body, &userInfoResp); err != nil {
		return nil, fmt.Errorf("解析飞书用户信息失败: %w", err)
	}

	if userInfoResp.Code != 0 {
		return nil, fmt.Errorf("飞书返回错误 (code: %d): %s", userInfoResp.Code, userInfoResp.Msg)
	}

	if userInfoResp.Data == nil {
		return nil, errors.New("飞书响应中未包含用户数据")
	}

	userData := userInfoResp.Data
	userInfo := &SSOUserInfo{
		Sub:       userData.Sub,
		OpenID:    userData.OpenID,
		UnionID:   userData.UnionID,
		Email:     userData.Email,
		Name:      userData.Name,
		Mobile:   userData.Mobile,
		AvatarURL: userData.AvatarURL,
	}

	if userInfo.Email != "" {
		parts := strings.Split(userInfo.Email, "@")
		userInfo.Username = parts[0]
	} else if userInfo.OpenID != "" {
		userInfo.Username = "feishu_" + userInfo.OpenID
	} else {
		userInfo.Username = "sso_" + uuid.New().String()[:8]
	}

	return userInfo, nil
}