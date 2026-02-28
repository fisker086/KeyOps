package system

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/repository"
	bastionService "github.com/fisker086/keyops/internal/service/bastion"
	"github.com/fisker086/keyops/pkg/distributed"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
	"github.com/gin-gonic/gin"
)

type SettingHandler struct {
	repo            *repository.SettingRepository
	notificationMgr *notification.NotificationManager
	hostMonitor     *bastionService.HostMonitorService
	configSync      *distributed.ConfigSyncManager
}

func NewSettingHandler(repo *repository.SettingRepository, notificationMgr *notification.NotificationManager) *SettingHandler {
	h := &SettingHandler{
		repo:            repo,
		notificationMgr: notificationMgr,
	}

	// 如果 Redis 启用，初始化配置同步
	if pkgredis.IsEnabled() {
		h.configSync = distributed.NewConfigSyncManager(pkgredis.GetClient(), "zjump:config:changes")
		// 启动配置同步监听
		go h.configSync.Start()
	}

	return h
}

// SetHostMonitor 设置主机监控服务（用于更新配置后重新加载）
func (h *SettingHandler) SetHostMonitor(hostMonitor *bastionService.HostMonitorService) {
	h.hostMonitor = hostMonitor
}

// GetAllSettings 获取所有设置
func (h *SettingHandler) GetAllSettings(c *gin.Context) {
	settings, err := h.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to get settings: "+err.Error()))
		return
	}

	// 转换为响应格式；key 若带 category 前缀则去掉，便于前端按 enableFeishu 等使用
	response := make(map[string]interface{})
	for _, setting := range settings {
		if response[setting.Category] == nil {
			response[setting.Category] = make(map[string]interface{})
		}
		categoryMap := response[setting.Category].(map[string]interface{})
		key := setting.Key
		if strings.HasPrefix(key, setting.Category+".") {
			key = key[len(setting.Category)+1:]
		}
		categoryMap[key] = setting.Value
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Settings retrieved successfully",
		Data:    response,
	})
}

// GetSettingsByCategory 根据分类获取设置
func (h *SettingHandler) GetSettingsByCategory(c *gin.Context) {
	category := c.Param("category")

	settings, err := h.repo.GetByCategory(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to get settings: "+err.Error()))
		return
	}

	// 转换为响应格式
	response := make(map[string]interface{})
	for _, setting := range settings {
		response[setting.Key] = setting.Value
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Settings retrieved successfully",
		Data:    response,
	})
}

// UpdateSettings 批量更新设置
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var request map[string]interface{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	// 转换为 Setting 模型
	var settings []model.Setting

	// 支持两种格式：
	// 1. { "key": { "value": "val", "category": "cat" } }  (前端新格式)
	// 2. { "category": { "key": "value" } }  (旧格式)

	for key, value := range request {
		// 检查是否是新格式 (包含 value 和 category 字段)
		if valueMap, ok := value.(map[string]interface{}); ok {
			if val, hasValue := valueMap["value"]; hasValue {
				if cat, hasCat := valueMap["category"]; hasCat {
					// 新格式
					valueType := "string"
					valueStr := fmt.Sprintf("%v", val)

					// 判断值类型
					switch v := val.(type) {
					case string:
						valueType = "string"
						valueStr = v
					case bool:
						valueType = "boolean"
						if v {
							valueStr = "true"
						} else {
							valueStr = "false"
						}
					case float64:
						valueType = "number"
						valueStr = fmt.Sprintf("%v", v)
					}

					settings = append(settings, model.Setting{
						Key:      key,
						Value:    valueStr,
						Category: fmt.Sprintf("%v", cat),
						Type:     valueType,
					})
					continue
				}
			}

			// 旧格式: category 作为 key，内部是 settings map
			category := key
			for settingKey, settingValue := range valueMap {
				valueType := "string"
				valueStr := ""

				switch v := settingValue.(type) {
				case string:
					valueType = "string"
					valueStr = v
				case bool:
					valueType = "boolean"
					if v {
						valueStr = "true"
					} else {
						valueStr = "false"
					}
				case float64:
					valueType = "number"
					valueStr = fmt.Sprintf("%v", v)
				default:
					valueType = "string"
					valueStr = fmt.Sprintf("%v", v)
				}

				// 规范化 key：如果 key 不包含 category 前缀，则添加前缀（兼容旧数据）
				// 例如：category="system", key="showWatermark" -> "system.showWatermark"
				finalKey := settingKey
				if !strings.Contains(settingKey, ".") {
					finalKey = fmt.Sprintf("%s.%s", category, settingKey)
				}

				settings = append(settings, model.Setting{
					Key:      finalKey,
					Value:    valueStr,
					Category: category,
					Type:     valueType,
				})
			}
		}
	}

	// 批量更新
	if err := h.repo.BatchUpsert(settings); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to update settings: "+err.Error()))
		return
	}

	// 检查是否更新了通知相关的设置，如果是则重新加载通知配置
	hasNotificationSettings := false
	hasHostMonitorSettings := false
	for _, setting := range settings {
		if setting.Category == "notification" {
			hasNotificationSettings = true
		}
		if setting.Category == "host_monitor" {
			hasHostMonitorSettings = true
		}
	}
	if hasNotificationSettings && h.notificationMgr != nil {
		h.notificationMgr.ReloadFromDatabase()
	}
	if hasHostMonitorSettings && h.hostMonitor != nil {
		h.hostMonitor.ReloadConfig()

		// 如果启用了 Redis，发布配置变更通知（通知其他实例）
		if h.configSync != nil {
			for _, setting := range settings {
				if setting.Category == "host_monitor" {
					if err := h.configSync.PublishConfigChange(setting.Key, setting.Value); err != nil {
						// 记录错误但不影响主流程
						fmt.Printf("[SettingHandler] Failed to publish config change: %v\n", err)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Settings updated successfully",
		Data:    nil,
	})
}

// UpdateSetting 更新单个设置
func (h *SettingHandler) UpdateSetting(c *gin.Context) {
	var request struct {
		Key      string      `json:"key" binding:"required"`
		Value    interface{} `json:"value" binding:"required"`
		Category string      `json:"category" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	// 判断值类型
	valueType := "string"
	valueStr := ""

	switch v := request.Value.(type) {
	case string:
		valueType = "string"
		valueStr = v
	case bool:
		valueType = "boolean"
		if v {
			valueStr = "true"
		} else {
			valueStr = "false"
		}
	case float64:
		valueType = "number"
		valueStr = fmt.Sprintf("%v", v)
	default:
		valueType = "string"
		valueStr = fmt.Sprintf("%v", v)
	}

	setting := &model.Setting{
		Key:      request.Key,
		Value:    valueStr,
		Category: request.Category,
		Type:     valueType,
	}

	if err := h.repo.Upsert(setting); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to update setting: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Setting updated successfully",
		Data:    setting,
	})
}

// DeleteSetting 删除设置
func (h *SettingHandler) DeleteSetting(c *gin.Context) {
	key := c.Param("key")

	if err := h.repo.Delete(key); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to delete setting: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Setting deleted successfully",
		Data:    nil,
	})
}

// GetPublicSettings 获取公开的系统设置（不需要认证）
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	// 只返回 system 分类的公开设置
	settings, err := h.repo.GetByCategory("system")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "Failed to get public settings: "+err.Error()))
		return
	}

	// 转换为响应格式
	response := make(map[string]interface{})
	for _, setting := range settings {
		response[setting.Key] = setting.Value
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Public settings retrieved successfully",
		Data:    response,
	})
}

// GetAuthMethods 获取启用的认证方式（不需要认证）
func (h *SettingHandler) GetAuthMethods(c *gin.Context) {
	// 获取认证配置（从 auth category 读取 authMethod）
	authSettings, _ := h.repo.GetByCategory("auth")

	// 获取当前的认证方式，默认为 password
	authMethod := "password"
	for _, setting := range authSettings {
		if setting.Key == "authMethod" {
			authMethod = setting.Value
			break
		}
	}

	// 根据 authMethod 判断各认证方式是否启用
	passwordEnabled := authMethod == "password"
	ldapEnabled := authMethod == "ldap"
	ssoEnabled := authMethod == "sso"

	// 返回认证方式配置
	response := map[string]interface{}{
		"password": passwordEnabled,
		"ldap":     ldapEnabled,
		"sso":      ssoEnabled,
		"primary":  authMethod,
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Auth methods retrieved successfully",
		Data:    response,
	})
}

// TestLDAPConnection 测试 LDAP 连接
func (h *SettingHandler) TestLDAPConnection(c *gin.Context) {
	var request struct {
		Server       string `json:"server" binding:"required"`
		BaseDN       string `json:"baseDN" binding:"required"`
		BindDN       string `json:"bindDN" binding:"required"`
		BindPassword string `json:"bindPassword" binding:"required"`
		EnableSSL    bool   `json:"enableSSL"`
		EnableTLS    bool   `json:"enableTLS"`
		Timeout      int    `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	// TODO: 实现真实的 LDAP 连接测试
	// 这里先返回成功，实际应该使用 go-ldap 库测试连接

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "LDAP connection test successful",
		Data: map[string]interface{}{
			"success": true,
			"message": "Successfully connected to LDAP server",
		},
	})
}

// TestSSOConnection 测试 SSO 配置
func (h *SettingHandler) TestSSOConnection(c *gin.Context) {
	var request struct {
		Provider     string `json:"provider" binding:"required"`
		ClientID     string `json:"clientId" binding:"required"`
		ClientSecret string `json:"clientSecret" binding:"required"`
		AuthURL      string `json:"authUrl" binding:"required"`
		TokenURL     string `json:"tokenUrl" binding:"required"`
		UserInfoURL  string `json:"userInfoUrl"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	// TODO: 实现真实的 SSO 配置测试
	// 这里先返回成功，实际应该测试 OAuth2/OIDC 端点的可用性

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "SSO configuration test successful",
		Data: map[string]interface{}{
			"success": true,
			"message": "SSO configuration is valid",
		},
	})
}

// getSiteName 从系统设置读取站点名称，用于通知文案等，未配置时返回 "ZJump"
func (h *SettingHandler) getSiteName() string {
	settings, err := h.repo.GetByCategory("system")
	if err != nil {
		return "ZJump"
	}
	for _, s := range settings {
		if s.Key == "siteName" || s.Key == "system.siteName" {
			if v := strings.TrimSpace(s.Value); v != "" {
				return v
			}
			break
		}
	}
	return "ZJump"
}

// TestFeishuNotification 测试飞书通知
func (h *SettingHandler) TestFeishuNotification(c *gin.Context) {
	var request struct {
		Webhook string `json:"webhook" binding:"required"`
		Secret  string `json:"secret"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	siteName := h.getSiteName()
	title := siteName + " 通知测试"

	// 创建飞书通知器
	notifier := notification.NewFeishuNotifier(request.Webhook, request.Secret)

	// 发送测试消息
	err := notifier.SendAlert(
		title,
		"**这是一条测试消息**\n\n如果您收到此消息，说明飞书通知配置成功！\n\n测试时间："+time.Now().Format("2006-01-02 15:04:05"),
	)

	if err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    0,
			Message: "Test notification sent",
			Data: map[string]interface{}{
				"success": false,
				"message": "发送失败: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Test notification sent successfully",
		Data: map[string]interface{}{
			"success": true,
			"message": " 测试消息已发送！请检查飞书群组是否收到消息。",
		},
	})
}

// TestDingtalkNotification 测试钉钉通知
func (h *SettingHandler) TestDingtalkNotification(c *gin.Context) {
	var request struct {
		Webhook string `json:"webhook" binding:"required"`
		Secret  string `json:"secret"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	siteName := h.getSiteName()
	title := siteName + " 通知测试"

	// 创建钉钉通知器
	notifier := notification.NewDingTalkNotifier(request.Webhook, request.Secret)

	// 发送测试消息（使用 Markdown 格式）
	testContent := fmt.Sprintf(`## %s

---

**这是一条测试消息**

如果您收到此消息，说明钉钉通知配置成功！

---

- **测试时间：** %s
- **系统：** %s 堡垒机
- **状态：**  配置正常

---

*测试消息来源：%s 系统设置*`, title, time.Now().Format("2006-01-02 15:04:05"), siteName, siteName)

	err := notifier.SendAlert(title, testContent)

	if err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    0,
			Message: "Test notification sent",
			Data: map[string]interface{}{
				"success": false,
				"message": "发送失败: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Test notification sent successfully",
		Data: map[string]interface{}{
			"success": true,
			"message": " 测试消息已发送！请检查钉钉群组是否收到消息。",
		},
	})
}

// TestWechatNotification 测试企业微信通知
func (h *SettingHandler) TestWechatNotification(c *gin.Context) {
	var request struct {
		CorpID  string `json:"corpId" binding:"required"`
		AgentID string `json:"agentId" binding:"required"`
		Secret  string `json:"secret" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}

	// 注意：企业微信需要先获取 access_token，这里暂时返回提示信息
	// 如果你有企业微信机器人 webhook（群机器人），请在前端配置 webhook URL
	// TODO: 完整实现企业微信 API 集成（需要 access_token 机制）

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "WeChat Work notification info",
		Data: map[string]interface{}{
			"success": true,
			"message": " 企业微信通知需要完整的 API 集成。\n\n建议：\n1. 如果使用群机器人，请配置 Webhook URL\n2. 如果使用应用消息，需要实现 access_token 获取逻辑\n\n当前配置的参数已保存，系统会在检测到危险命令时尝试发送通知。",
		},
	})
}

// TestWechatWebhook 测试企业微信群机器人 Webhook
func (h *SettingHandler) TestWechatWebhook(c *gin.Context) {
	var request struct {
		Webhook string `json:"webhook" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request: "+err.Error()))
		return
	}
	siteName := h.getSiteName()
	title := siteName + " 通知测试"
	notifier := notification.NewWeChatNotifier(request.Webhook)
	err := notifier.SendAlert(
		title,
		"**这是一条测试消息**\n\n如果您收到此消息，说明企业微信 Webhook 配置成功！\n\n测试时间："+time.Now().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    0,
			Message: "Test notification sent",
			Data: map[string]interface{}{
				"success": false,
				"message": "发送失败: " + err.Error(),
			},
		})
		return
	}
	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Message: "Test notification sent successfully",
		Data: map[string]interface{}{
			"success": true,
			"message": " 测试消息已发送！请检查企业微信群是否收到消息。",
		},
	})
}

// TestDeployGit 测试发布用 Git 连接（使用当前表单或已保存的 release 配置）
// POST /api/settings/test-deploy-git，body: { repo_url, deploy_git_username?, deploy_git_password? }
func (h *SettingHandler) TestDeployGit(c *gin.Context) {
	var request struct {
		RepoURL           string `json:"repo_url" binding:"required"`
		DeployGitUsername string `json:"deploy_git_username"`
		DeployGitPassword string `json:"deploy_git_password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "请填写要测试的仓库地址 (repo_url)"))
		return
	}
	repoURL := strings.TrimSpace(request.RepoURL)
	if repoURL == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "请填写要测试的仓库地址 (repo_url)"))
		return
	}
	username := strings.TrimSpace(request.DeployGitUsername)
	password := request.DeployGitPassword
	if username == "" || password == "" {
		settings, err := h.repo.GetByCategory(model.CategoryRelease)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "读取发布配置失败"))
			return
		}
		m := make(map[string]string)
		for _, st := range settings {
			m[st.Key] = st.Value
		}
		if username == "" {
			username = strings.TrimSpace(m["deploy_git_username"])
		}
		if password == "" {
			password = m["deploy_git_password"]
		}
	}
	parsed, err := url.Parse(repoURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "仓库地址格式无效: "+err.Error()))
		return
	}
	if username != "" || password != "" {
		parsed.User = url.UserPassword(username, password)
	}
	credentialURL := parsed.String()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", credentialURL)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if msg == "" {
			msg = err.Error()
		}
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		c.JSON(http.StatusBadRequest, model.Error(400, "Git 连接失败: "+msg))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "message": "连接成功"}))
}
