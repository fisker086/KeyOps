package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultOIDCTokenURL = "https://your-idp.example.com/oauth/token"
const defaultOIDCUserInfoURL = "https://your-idp.example.com/oauth/userinfo"

func (s *AuthService) exchangeOIDCToken(code, clientID, clientSecret, tokenURL, redirectURL string) (string, error) {
	if tokenURL == "" {
		tokenURL = defaultOIDCTokenURL
	}
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURL)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取token失败 (HTTP %d)", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType  string `json:"token_type"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析token响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("获取token失败: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", errors.New("响应中未包含access_token")
	}

	return tokenResp.AccessToken, nil
}

func (s *AuthService) getOIDCUserInfo(accessToken, userInfoURL string) (*SSOUserInfo, error) {
	if userInfoURL == "" {
		userInfoURL = defaultOIDCUserInfoURL
	}
	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户信息失败 (HTTP %d)", resp.StatusCode)
	}

	var userInfo SSOUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	if userInfo.Sub == "" && userInfo.Email == "" {
		return nil, errors.New("用户信息中缺少必要字段（sub或email）")
	}

	return &userInfo, nil
}