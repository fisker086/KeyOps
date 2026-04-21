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

const (
	defaultWeComGetTokenURL    = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	weComAuthGetUserInfoPath   = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	weComUserGetPath          = "https://qyapi.weixin.qq.com/cgi-bin/user/get"
)

func (s *AuthService) exchangeWeComCorpToken(clientID, clientSecret, tokenURL string) (string, error) {
	if tokenURL == "" {
		tokenURL = defaultWeComGetTokenURL
	}
	u, err := url.Parse(tokenURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Get("corpid") == "" {
		q.Set("corpid", clientID)
	}
	if q.Get("corpsecret") == "" {
		q.Set("corpsecret", clientSecret)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn  int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("解析企业微信 gettoken 失败: %w", err)
	}
	if out.ErrCode != 0 {
		return "", fmt.Errorf("企业微信 gettoken 错误 [%d]: %s", out.ErrCode, out.ErrMsg)
	}
	if out.AccessToken == "" {
		return "", errors.New("企业微信 gettoken 未返回 access_token")
	}
	return out.AccessToken, nil
}

func (s *AuthService) getWeComUserInfo(corpAccessToken, oauthCode, agentID string) (*SSOUserInfo, error) {
	if oauthCode == "" {
		return nil, errors.New("缺少 OAuth code")
	}
	u, err := url.Parse(weComAuthGetUserInfoPath)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("access_token", corpAccessToken)
	q.Set("code", oauthCode)
	if agentID != "" {
		q.Set("agentid", agentID)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var step1 struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"UserId"`
		UserID2 string `json:"userid"`
	}
	if err := json.Unmarshal(raw, &step1); err != nil {
		return nil, fmt.Errorf("解析企业微信 auth/getuserinfo 失败: %w", err)
	}
	if step1.ErrCode != 0 {
		return nil, fmt.Errorf("企业微信 auth/getuserinfo [%d]: %s", step1.ErrCode, step1.ErrMsg)
	}
	uid := firstNonEmpty(step1.UserID, step1.UserID2)
	if uid == "" {
		return nil, errors.New("企业微信未返回 UserId")
	}

	u2, _ := url.Parse(weComUserGetPath)
	q2 := u2.Query()
	q2.Set("access_token", corpAccessToken)
	q2.Set("userid", uid)
	u2.RawQuery = q2.Encode()

	resp2, err := client.Get(u2.String())
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	raw2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, err
	}
	var detail struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
		Name    string `json:"name"`
		Mobile  string `json:"mobile"`
		Email   string `json:"email"`
		Avatar  string `json:"avatar"`
	}
	if err := json.Unmarshal(raw2, &detail); err != nil {
		return nil, fmt.Errorf("解析企业微信 user/get 失败: %w", err)
	}
	if detail.ErrCode != 0 {
		return nil, fmt.Errorf("企业微信 user/get [%d]: %s", detail.ErrCode, detail.ErrMsg)
	}

	ui := &SSOUserInfo{
		Sub:       detail.UserID,
		Name:      detail.Name,
		Email:     detail.Email,
		Mobile:   detail.Mobile,
		AvatarURL: detail.Avatar,
	}
	if ui.Email != "" {
		ui.Username = strings.Split(ui.Email, "@")[0]
	} else {
		ui.Username = "wecom_" + detail.UserID
	}
	return ui, nil
}