package notification

import (
	"github.com/fisker086/keyops/pkg/logger"

	"github.com/fisker086/keyops/internal/repository"
	"gorm.io/gorm"
)

// InitFromDatabase 从数据库初始化通知管理器
func InitFromDatabase(db *gorm.DB) *NotificationManager {
	nm := NewNotificationManager()
	nm.db = db      // 保存数据库连接以便后续重新加载
	nm.loadConfig() // 加载配置
	return nm
}

// loadConfig 从数据库加载通知配置（内部方法）
func (nm *NotificationManager) loadConfig() {
	if nm.db == nil {
		logger.Infof("[Notification] Cannot load: database connection is nil")
		return
	}

	// 从数据库加载通知配置
	settingRepo := repository.NewSettingRepository(nm.db.(*gorm.DB))
	settings, err := settingRepo.GetByCategory("notification")
	if err != nil {
		logger.Infof("[Notification] Failed to load notification settings: %v", err)
		return
	}

	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	logger.Infof("[Notification] Loading notification settings from database...")

	// 获取写锁，准备更新配置
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 清空现有通知器
	nm.notifiers = make([]Notifier, 0)
	// 默认启用通知管理器（即使没有配置渠道，也会记录日志）
	nm.enabled = true

	var notifiersAdded int

	// 配置飞书通知
	if settingsMap["enableFeishu"] == "true" {
		webhookURL := settingsMap["feishuWebhook"]
		secret := settingsMap["feishuSecret"]
		if webhookURL != "" {
			feishu := NewFeishuNotifier(webhookURL, secret)
			nm.AddNotifier(feishu)
			notifiersAdded++
			truncatedURL := webhookURL
			if len(webhookURL) > 50 {
				truncatedURL = webhookURL[:50] + "..."
			}
			logger.Infof("[Notification]  Feishu notifier enabled (webhook: %s)", truncatedURL)
		} else {
			logger.Infof("[Notification]  Feishu enabled but webhook URL is empty")
		}
	}

	// 配置钉钉通知
	if settingsMap["enableDingTalk"] == "true" {
		webhookURL := settingsMap["dingTalkWebhook"]
		secret := settingsMap["dingTalkSecret"]
		if webhookURL != "" {
			dingtalk := NewDingTalkNotifier(webhookURL, secret)
			nm.AddNotifier(dingtalk)
			notifiersAdded++
			truncatedURL := webhookURL
			if len(webhookURL) > 50 {
				truncatedURL = webhookURL[:50] + "..."
			}
			logger.Infof("[Notification]  DingTalk notifier enabled (webhook: %s)", truncatedURL)
		} else {
			logger.Infof("[Notification]  DingTalk enabled but webhook URL is empty")
		}
	}

	// 配置企业微信通知
	if settingsMap["enableWeChat"] == "true" {
		webhookURL := settingsMap["wechatWebhookURL"]
		if webhookURL != "" {
			wechat := NewWeChatNotifier(webhookURL)
			nm.AddNotifier(wechat)
			notifiersAdded++
			truncatedURL := webhookURL
			if len(webhookURL) > 50 {
				truncatedURL = webhookURL[:50] + "..."
			}
			logger.Infof("[Notification]  WeChat notifier enabled (webhook: %s)", truncatedURL)
		} else {
			logger.Infof("[Notification]  WeChat enabled but webhook URL is empty")
		}
	}

	// 通知管理器默认启用，如果有通知器则记录成功日志
	if notifiersAdded > 0 {
		logger.Infof("[Notification]  Notification system enabled with %d notifier(s)", notifiersAdded)
	} else {
		logger.Infof("[Notification]  Notification system enabled (no channels configured yet, notifications will be logged but not sent)")
	}
}

// ReloadFromDatabase 重新从数据库加载通知配置（公开方法，用于动态更新配置）
func (nm *NotificationManager) ReloadFromDatabase() {
	logger.Infof("[Notification] Reloading notification configuration from database...")
	nm.loadConfig()
}
