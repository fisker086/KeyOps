package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/bastion/types"
)

// ApiClient API 客户端 - 用于实时上报审计数据
type ApiClient struct {
	baseURL    string
	httpClient *http.Client
	proxyID    string
}

// Config API 客户端配置
type Config struct {
	BaseURL string
	Timeout time.Duration
	ProxyID string
}

// NewApiClient 创建 API 客户端
func NewApiClient(config Config) *ApiClient {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	return &ApiClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		proxyID: config.ProxyID,
	}
}

// ReportSession 实时上报会话信息
func (c *ApiClient) ReportSession(session *types.SessionRecord) error {
	url := c.baseURL + "/api/proxy/sessions"

	payload := map[string]interface{}{
		"proxy_id": c.proxyID,
		"session":  session,
	}

	return c.post(url, payload)
}

// CloseSession closes a session
func (c *ApiClient) CloseSession(sessionID, recording string) error {
	url := fmt.Sprintf("%s/api/proxy/sessions/%s/close", c.baseURL, sessionID)

	payload := map[string]interface{}{
		"proxy_id":  c.proxyID,
		"recording": recording,
		"end_time":  time.Now(),
	}

	return c.post(url, payload)
}

// ReportCommand 实时上报命令
func (c *ApiClient) ReportCommand(cmd *types.CommandRecord) error {
	url := c.baseURL + "/api/proxy/commands"

	payload := map[string]interface{}{
		"proxy_id": c.proxyID,
		"command":  cmd,
	}

	return c.post(url, payload)
}

// FetchBlacklist 获取危险命令黑名单（返回完整规则信息）
func (c *ApiClient) FetchBlacklist() ([]types.BlacklistRule, error) {
	url := c.baseURL + "/api/proxy/blacklist"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Proxy-ID", c.proxyID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api returned status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Rules []types.BlacklistRule `json:"rules"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("api error: %s", response.Message)
	}

	log.Printf("[ApiClient] Fetched %d blacklist rules", len(response.Data.Rules))
	return response.Data.Rules, nil
}

// Heartbeat 发送心跳
func (c *ApiClient) Heartbeat() error {
	url := c.baseURL + "/api/proxy/heartbeat"

	payload := map[string]interface{}{
		"proxy_id":  c.proxyID,
		"status":    "online",
		"timestamp": time.Now().Unix(),
	}

	return c.post(url, payload)
}

// ValidateSessionToken 验证会话令牌
func (c *ApiClient) ValidateSessionToken(token string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/proxy/validate-token?token=%s", c.baseURL, token)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Proxy-ID", c.proxyID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// post 发送 POST 请求
func (c *ApiClient) post(url string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Proxy-ID", c.proxyID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api returned status %d: %s", resp.StatusCode, string(body))
	}

	// 检查响应是否包含错误
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if errorMsg, ok := result["error"].(string); ok && errorMsg != "" {
			return fmt.Errorf("api error: %s", errorMsg)
		}
	}

	log.Printf("[ApiClient] Request successful: %s", url)
	return nil
}
