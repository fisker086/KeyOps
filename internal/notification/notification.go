package notification

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fisker086/keyops/pkg/logger"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Notifier 通知接口
type Notifier interface {
	SendAlert(title, content string) error
}

// FeishuNotifier 飞书通知
type FeishuNotifier struct {
	WebhookURL string
	Secret     string
}

// DingTalkNotifier 钉钉通知
type DingTalkNotifier struct {
	WebhookURL string
	Secret     string
}

// WeChatNotifier 企业微信通知
type WeChatNotifier struct {
	WebhookURL string
}

// NewFeishuNotifier 创建飞书通知器
func NewFeishuNotifier(webhookURL, secret string) *FeishuNotifier {
	return &FeishuNotifier{
		WebhookURL: webhookURL,
		Secret:     secret,
	}
}

// NewDingTalkNotifier 创建钉钉通知器
func NewDingTalkNotifier(webhookURL, secret string) *DingTalkNotifier {
	return &DingTalkNotifier{
		WebhookURL: webhookURL,
		Secret:     secret,
	}
}

// NewWeChatNotifier 创建企业微信通知器
func NewWeChatNotifier(webhookURL string) *WeChatNotifier {
	return &WeChatNotifier{
		WebhookURL: webhookURL,
	}
}

// normalizeMarkdownForChannels 将通用 Markdown 转为飞书/钉钉/企业微信均可正确渲染的格式：
// - 统一换行为 \n（避免 \r\n 导致行首不被识别）
// - 三级及以上标题先统一为 ##，再统一将行首 ## 标题改为 **粗体**（三端 webhook 对 ## 支持不稳定，粗体更可靠）
// - 行首 * 列表改为 - 列表（飞书仅支持 -，钉钉/企微通用）
func normalizeMarkdownForChannels(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = regexp.MustCompile(`(?m)^#{3,6}\s*`).ReplaceAllString(s, "## ")
	s = regexp.MustCompile(`(?m)^(\s*)\*\s+`).ReplaceAllString(s, "$1- ")
	// 行首 "## 标题" -> "**标题**"，飞书/钉钉/企微机器人对 ** 支持稳定，避免 ## 显示为字面量
	s = regexp.MustCompile(`(?m)^## (.+)$`).ReplaceAllString(s, "**$1**")
	return strings.TrimSpace(s)
}

// normalizeMarkdownForFeishu 飞书 lark_md 专用，与钉钉/企微共用同一套通用规范化
func normalizeMarkdownForFeishu(s string) string {
	return normalizeMarkdownForChannels(s)
}

// buildFeishuCardElements 组装飞书卡片 elements
func buildFeishuCardElements(content string) []map[string]interface{} {
	content = normalizeMarkdownForFeishu(content)
	elements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"content": content,
				"tag":     "lark_md",
			},
		},
		{"tag": "hr"},
		{
			"tag": "note",
			"elements": []map[string]interface{}{
				{
					"tag":     "plain_text",
					"content": fmt.Sprintf("告警时间: %s", time.Now().Format("2006-01-02 15:04:05")),
				},
			},
		},
	}
	return elements
}

// SendAlert 发送飞书告警（通用方法）
func (n *FeishuNotifier) SendAlert(title, content string) error {
	timestamp := time.Now().Unix()
	sign := n.genSign(timestamp)
	elements := buildFeishuCardElements(content)

	message := map[string]interface{}{
		"timestamp": fmt.Sprintf("%d", timestamp),
		"sign":      sign,
		"msg_type":  "interactive",
		"card": map[string]interface{}{
			"config": map[string]interface{}{
				"wide_screen_mode": true,
				"enable_forward":   true,
				"update_multi":     true,
			},
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": title,
				},
				"template": "red",
			},
			"elements": elements,
		},
	}

	return n.sendRequest(message)
}

// SendDangerousCommandAlert 发送危险命令告警（飞书专用富文本卡片）
func (n *FeishuNotifier) SendDangerousCommandAlert(username, hostIP, command, reason string) error {
	timestamp := time.Now().Unix()
	sign := n.genSign(timestamp)

	// 确定危险等级
	riskLevel := " 高危"
	riskColor := "red"
	if len(command) < 20 {
		riskLevel = "🟠 中危"
		riskColor = "orange"
	}

	message := map[string]interface{}{
		"timestamp": fmt.Sprintf("%d", timestamp),
		"sign":      sign,
		"msg_type":  "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": "🚨 危险命令拦截告警",
				},
				"template": riskColor,
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]interface{}{
						"content": fmt.Sprintf("**检测到用户尝试执行危险命令，已被系统拦截！**"),
						"tag":     "lark_md",
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**👤 操作用户**\n%s", username),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**🖥️ 目标主机**\n%s", hostIP),
							},
						},
					},
				},
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("** 危险等级**\n%s", riskLevel),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**🕐 告警时间**\n%s", time.Now().Format("2006-01-02 15:04:05")),
							},
						},
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"content": fmt.Sprintf("**💻 拦截命令**\n```\n%s\n```", command),
						"tag":     "lark_md",
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"content": fmt.Sprintf("**📋 拦截原因**\n%s", reason),
						"tag":     "lark_md",
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "note",
					"elements": []map[string]interface{}{
						{
							"tag":     "plain_text",
							"content": "🛡️ 系统已自动拦截该命令，请及时核查用户操作意图",
						},
					},
				},
			},
		},
	}

	return n.sendRequest(message)
}

// genSign 生成飞书签名
func (n *FeishuNotifier) genSign(timestamp int64) string {
	if n.Secret == "" {
		return ""
	}

	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + n.Secret
	var data []byte
	h := hmac.New(sha256.New, []byte(stringToSign))
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// sendRequest 发送HTTP请求
func (n *FeishuNotifier) sendRequest(message map[string]interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("feishu returned non-200 status: %d", resp.StatusCode)
		if len(respBody) > 0 {
			errorMsg += fmt.Sprintf(", response: %s", string(respBody))
		}
		return fmt.Errorf(errorMsg)
	}

	// 检查响应体中的错误码（飞书即使返回 200，也可能在响应体中包含错误）
	if len(respBody) > 0 {
		var feishuResp map[string]interface{}
		if err := json.Unmarshal(respBody, &feishuResp); err == nil {
			if code, ok := feishuResp["code"].(float64); ok {
				if code != 0 {
					msg := "unknown error"
					if msgVal, ok := feishuResp["msg"].(string); ok {
						msg = msgVal
					}
					return fmt.Errorf("feishu returned error code: %.0f, msg: %s", code, msg)
				}
			}
		}
	}

	return nil
}

// SendAlert 发送钉钉告警（通用方法）
func (n *DingTalkNotifier) SendAlert(title, content string) error {
	content = normalizeMarkdownForChannels(content)
	timestamp := time.Now().UnixNano() / 1e6
	sign := n.genSign(timestamp)

	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": title,
			"text":  content,
		},
		"at": map[string]interface{}{
			"isAtAll": false,
		},
	}

	if n.Secret != "" {
		message["timestamp"] = timestamp
		message["sign"] = sign
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	url := n.WebhookURL
	if n.Secret != "" {
		url = fmt.Sprintf("%s&timestamp=%d&sign=%s", url, timestamp, sign)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned non-200 status: %d", resp.StatusCode)
	}

	logger.Infof("[Notification] DingTalk alert sent successfully")
	return nil
}

// SendDangerousCommandAlert 发送危险命令告警（钉钉专用Markdown格式）
func (n *DingTalkNotifier) SendDangerousCommandAlert(username, hostIP, command, reason string) error {
	timestamp := time.Now().UnixNano() / 1e6
	sign := n.genSign(timestamp)

	// 确定危险等级
	riskLevel := " 高危"
	if len(command) < 20 {
		riskLevel = "🟠 中危"
	}

	// 构建美观的Markdown内容
	content := fmt.Sprintf(`## 🚨 危险命令拦截告警

---

> **检测到用户尝试执行危险命令，已被系统拦截！**

---

### 📋 告警详情

- **👤 操作用户：** %s
- **🖥️ 目标主机：** %s
- ** 危险等级：** %s
- **🕐 告警时间：** %s

---

### 💻 拦截命令

`+"```"+`
%s
`+"```"+`

###  拦截原因

> %s

---

### 🛡️ 安全建议

-  系统已自动拦截该命令
-  请及时核查用户操作意图
- 📞 如有疑问请联系安全团队

---

*告警来源：ZJump 堡垒机系统*`,
		username,
		hostIP,
		riskLevel,
		time.Now().Format("2006-01-02 15:04:05"),
		command,
		reason,
	)

	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": "🚨 危险命令拦截告警",
			"text":  content,
		},
		"at": map[string]interface{}{
			"isAtAll": false,
		},
	}

	if n.Secret != "" {
		message["timestamp"] = timestamp
		message["sign"] = sign
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	url := n.WebhookURL
	if n.Secret != "" {
		url = fmt.Sprintf("%s&timestamp=%d&sign=%s", url, timestamp, sign)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

// genSign 生成钉钉签名
func (n *DingTalkNotifier) genSign(timestamp int64) string {
	if n.Secret == "" {
		return ""
	}

	stringToSign := fmt.Sprintf("%d\n%s", timestamp, n.Secret)
	h := hmac.New(sha256.New, []byte(n.Secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// SendAlert 发送企业微信告警（通用方法）
func (n *WeChatNotifier) SendAlert(title, content string) error {
	content = normalizeMarkdownForChannels(content)
	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("## %s\n\n%s", title, content),
		},
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat returned non-200 status: %d", resp.StatusCode)
	}

	logger.Infof("[Notification] WeChat alert sent successfully")
	return nil
}

// SendDangerousCommandAlert 发送危险命令告警（企业微信专用Markdown格式）
func (n *WeChatNotifier) SendDangerousCommandAlert(username, hostIP, command, reason string) error {
	// 确定危险等级
	riskLevel := " 高危"
	if len(command) < 20 {
		riskLevel = "🟠 中危"
	}

	// 构建美观的Markdown内容
	content := fmt.Sprintf(`## 🚨 危险命令拦截告警

---

> <font color="warning">**检测到用户尝试执行危险命令，已被系统拦截！**</font>

---

### 📋 告警详情

> **👤 操作用户：**%s
> **🖥️ 目标主机：**%s
> ** 危险等级：**%s
> **🕐 告警时间：**%s

---

### 💻 拦截命令

`+"```"+`
%s
`+"```"+`

###  拦截原因

> %s

---

### 🛡️ 安全建议

- <font color="info"> 系统已自动拦截该命令</font>
- <font color="warning"> 请及时核查用户操作意图</font>
- <font color="comment">📞 如有疑问请联系安全团队</font>

---

*告警来源：ZJump 堡垒机系统*`,
		username,
		hostIP,
		riskLevel,
		time.Now().Format("2006-01-02 15:04:05"),
		command,
		reason,
	)

	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": content,
		},
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

// NotificationManager 通知管理器
type NotificationManager struct {
	notifiers []Notifier
	enabled   bool
	db        interface{}  // 数据库连接，用于重新加载配置
	mu        sync.RWMutex // 读写锁，保护并发访问
}

// NewNotificationManager 创建通知管理器
// 默认启用通知管理器（即使没有配置渠道，也会记录日志）
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		notifiers: make([]Notifier, 0),
		enabled:   true, // 默认启用
	}
}

// SetEnabled 设置是否启用通知
func (m *NotificationManager) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// IsEnabled 检查是否启用
// 注意：即使没有配置通知渠道，通知管理器也默认启用（会记录日志但不会发送通知）
func (m *NotificationManager) IsEnabled() bool {
	return m.enabled
}

// AddNotifier 添加通知器
func (m *NotificationManager) AddNotifier(notifier Notifier) {
	m.notifiers = append(m.notifiers, notifier)
}

// GetNotifiersCount 获取通知器数量（线程安全）
func (m *NotificationManager) GetNotifiersCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.notifiers)
}

// SendDangerousCommandAlert 发送危险命令告警
func (m *NotificationManager) SendDangerousCommandAlert(username, hostIP, command, reason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled || len(m.notifiers) == 0 {
		return
	}

	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			switch v := n.(type) {
			case *FeishuNotifier:
				if err := v.SendDangerousCommandAlert(username, hostIP, command, reason); err != nil {
					logger.Errorf("[Notification] Failed to send Feishu alert: %v", err)
				}
			case *DingTalkNotifier:
				if err := v.SendDangerousCommandAlert(username, hostIP, command, reason); err != nil {
					logger.Errorf("[Notification] Failed to send DingTalk alert: %v", err)
				}
			case *WeChatNotifier:
				if err := v.SendDangerousCommandAlert(username, hostIP, command, reason); err != nil {
					logger.Errorf("[Notification] Failed to send WeChat alert: %v", err)
				}
			default:
				title := " 危险命令告警"
				content := fmt.Sprintf(`**用户**: %s
**目标主机**: %s
**命令**: %s
**原因**: %s
**时间**: %s

请立即检查！`, username, hostIP, command, reason, time.Now().Format("2006-01-02 15:04:05"))
				if err := n.SendAlert(title, content); err != nil {
					logger.Errorf("[Notification] Failed to send alert: %v", err)
				}
			}
		}(notifier)
	}
}

// SendSessionAlert 发送会话告警
func (m *NotificationManager) SendSessionAlert(username, hostIP, action string) {
	if !m.IsEnabled() {
		return
	}

	title := "ℹ️ 会话通知"
	content := fmt.Sprintf(`**用户**: %s
**目标主机**: %s
**操作**: %s
**时间**: %s`, username, hostIP, action, time.Now().Format("2006-01-02 15:04:05"))

	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.SendAlert(title, content); err != nil {
				logger.Errorf("[Notification] Failed to send alert: %v", err)
			}
		}(notifier)
	}
}

// SendAlert 发送通用告警
func (m *NotificationManager) SendAlert(title, content string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled || len(m.notifiers) == 0 {
		return
	}

	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.SendAlert(title, content); err != nil {
				logger.Errorf("[Notification] Failed to send alert: %v", err)
			}
		}(notifier)
	}
}
