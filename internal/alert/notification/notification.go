package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/repository"
)

// AlertNotifier 告警通知器
type AlertNotifier struct {
	strategyLogRepo     *repository.StrategyLogRepository
	templateRepo        *repository.AlertTemplateRepository
	channelRepo         *repository.AlertChannelRepository
	channelTemplateRepo *repository.ChannelTemplateRepository
	alertGroupRepo      *repository.AlertGroupRepository
	ruleSourceRepo      *repository.AlertRuleSourceRepository
	frontendURL         string // 前端URL，用于构建详情链接
}

// NewAlertNotifier 创建告警通知器
func NewAlertNotifier(
	strategyLogRepo *repository.StrategyLogRepository,
	templateRepo *repository.AlertTemplateRepository,
	channelRepo *repository.AlertChannelRepository,
	channelTemplateRepo *repository.ChannelTemplateRepository,
	alertGroupRepo *repository.AlertGroupRepository,
	ruleSourceRepo *repository.AlertRuleSourceRepository,
	frontendURL string,
) *AlertNotifier {
	return &AlertNotifier{
		strategyLogRepo:     strategyLogRepo,
		templateRepo:        templateRepo,
		channelRepo:         channelRepo,
		channelTemplateRepo: channelTemplateRepo,
		alertGroupRepo:      alertGroupRepo,
		ruleSourceRepo:      ruleSourceRepo,
		frontendURL:         frontendURL,
	}
}

// SendNotification 发送通知
// 注意：告警通知使用告警渠道（alert-channel）配置，不依赖系统设置中的 NotificationManager
func (n *AlertNotifier) SendNotification(event *model.AlertEvent, strategies []model.AlertStrategy, notificationManager *notification.NotificationManager) error {
	var tags map[string]string
	if err := json.Unmarshal(event.Tags, &tags); err != nil {
		return err
	}

	var annotations map[string]string
	if err := json.Unmarshal(event.Annotations, &annotations); err != nil {
		annotations = make(map[string]string)
	}

	// 为每个策略发送通知
	for _, strategy := range strategies {
		// 必须配置模板才能发送通知
		if strategy.TemplateID == 0 {
			log.Printf("[AlertNotifier] Strategy %d (name=%s) has no template configured (TemplateID=0), skipping notification", strategy.ID, strategy.StrategyName)
			// 记录策略日志（未发送）
			strategyContent, _ := json.Marshal(strategy)
			logEntry := &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        false,
				NotifyType:      1, // 告警
				ErrMessage:      "策略未配置告警模板，请前往 告警管理 → 告警策略 中为该策略关联一个告警模板",
			}
			if err := n.strategyLogRepo.Create(logEntry); err != nil {
				log.Printf("[AlertNotifier] Failed to create strategy log: %v", err)
			}
			continue
		}

		log.Printf("[AlertNotifier] Strategy %d (name=%s) has template ID=%d, using template to send notification", strategy.ID, strategy.StrategyName, strategy.TemplateID)
		// 使用模板发送通知（内部会记录日志）
		if err := n.sendNotificationWithTemplate(event, strategy, tags, annotations); err != nil {
			log.Printf("[AlertNotifier] Failed to send notification with template for strategy %d: %v", strategy.ID, err)
		}
	}

	return nil
}

// sendNotificationWithTemplate 使用模板发送通知
func (n *AlertNotifier) sendNotificationWithTemplate(event *model.AlertEvent, strategy model.AlertStrategy, tags, annotations map[string]string) error {
	log.Printf("[AlertNotifier] sendNotificationWithTemplate: event ID=%d, strategy ID=%d, template ID=%d", event.ID, strategy.ID, strategy.TemplateID)

	// 加载模板
	template, err := n.templateRepo.FindByID(strategy.TemplateID)
	if err != nil {
		log.Printf("[AlertNotifier] Failed to load template %d: %v", strategy.TemplateID, err)
		return fmt.Errorf("failed to load template: %w", err)
	}

	log.Printf("[AlertNotifier] Template loaded: ID=%d, Name=%s, Enable=%v", template.ID, template.TemplateName, template.Enable)

	if !template.Enable {
		log.Printf("[AlertNotifier] Template %d is disabled", template.ID)
		return fmt.Errorf("template %d is disabled", template.ID)
	}

	// 解析模板的 channels 配置
	var channelsConfig map[string]interface{}
	if len(template.Channels) == 0 {
		log.Printf("[AlertNotifier] Template %d has no channels JSON data", template.ID)
		channelsConfig = make(map[string]interface{})
	} else {
		log.Printf("[AlertNotifier] Template %d channels JSON length: %d bytes", template.ID, len(template.Channels))
		if err := json.Unmarshal(template.Channels, &channelsConfig); err != nil {
			log.Printf("[AlertNotifier] Failed to parse template %d channels JSON: %v", template.ID, err)
			return fmt.Errorf("failed to parse template channels: %w", err)
		}
		log.Printf("[AlertNotifier] Template %d channels config parsed: %+v", template.ID, channelsConfig)
	}

	// 收集所有渠道ID
	var channelIDs []uint
	for key, channelIDsInterface := range channelsConfig {
		log.Printf("[AlertNotifier] Processing channel key: %s, value: %+v", key, channelIDsInterface)
		if channelIDsArray, ok := channelIDsInterface.([]interface{}); ok {
			for _, idInterface := range channelIDsArray {
				if idFloat, ok := idInterface.(float64); ok {
					channelIDs = append(channelIDs, uint(idFloat))
					log.Printf("[AlertNotifier] Added channel ID: %d", uint(idFloat))
				}
			}
		}
	}

	log.Printf("[AlertNotifier] Total channel IDs collected from template %d: %d, IDs: %v", template.ID, len(channelIDs), channelIDs)

	if len(channelIDs) == 0 {
		// 记录策略日志（没有配置渠道）
		strategyContent, _ := json.Marshal(strategy)
		logEntry := &model.StrategyLog{
			AlertID:         event.ID,
			StrategyID:      strategy.ID,
			StrategyContent: strategyContent,
			IsNotify:        false,
			NotifyType:      1, // 告警
			ErrMessage:      fmt.Sprintf("告警模板 %d 未配置通知渠道，请前往 告警管理 → 告警模板 中为该模板配置渠道（飞书/钉钉/企业微信等）", template.ID),
		}
		if err := n.strategyLogRepo.Create(logEntry); err != nil {
			log.Printf("[AlertNotifier] Failed to create strategy log: %v", err)
		}
		return fmt.Errorf("template %d has no channels configured", template.ID)
	}

	// 加载渠道信息并发送通知
	var sendErrors []string
	successCount := 0
	for _, channelID := range channelIDs {
		channel, err := n.channelRepo.FindByID(channelID)
		if err != nil {
			continue
		}

		// 加载渠道模板内容
		channelTemplate, err := n.channelTemplateRepo.FindByTemplateIDAndChannelID(template.ID, channelID)
		if err != nil {
			continue
		}

		// 检查渠道模板是否存在
		if channelTemplate == nil {
			continue
		}

		// 检查模板是否完成配置
		if !channelTemplate.Finished {
			continue
		}

		// 构建消息内容（使用渠道模板内容）
		title, content := n.buildMessage(event, template, channel, channelTemplate.Content, tags, annotations, false)

		// 根据渠道类型创建通知器并发送
		if err := n.sendToChannel(channel, title, content); err != nil {
			// 记录错误，但继续处理其他渠道
			sendErrors = append(sendErrors, fmt.Sprintf("渠道 %d (%s): %v", channelID, channel.ChannelType, err))
		} else {
			successCount++
		}
	}

	// 记录通知日志（无论成功或失败）
	strategyContent, _ := json.Marshal(strategy)
	var logEntry *model.StrategyLog

	log.Printf("[AlertNotifier] Creating strategy log: alertID=%d, strategyID=%d, successCount=%d, errorCount=%d", event.ID, strategy.ID, successCount, len(sendErrors))

	if successCount > 0 {
		// 至少有一个渠道成功
		if len(sendErrors) > 0 {
			// 部分成功
			logEntry = &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        true,
				NotifyType:      1, // 告警
				ErrMessage:      fmt.Sprintf("部分渠道发送失败: %s", strings.Join(sendErrors, "; ")),
			}
			log.Printf("[AlertNotifier] Strategy log (partial success): alertID=%d, strategyID=%d", event.ID, strategy.ID)
		} else {
			// 全部成功
			logEntry = &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        true,
				NotifyType:      1, // 告警
			}
			log.Printf("[AlertNotifier] Strategy log (all success): alertID=%d, strategyID=%d", event.ID, strategy.ID)
		}
	} else {
		// 所有渠道都失败
		logEntry = &model.StrategyLog{
			AlertID:         event.ID,
			StrategyID:      strategy.ID,
			StrategyContent: strategyContent,
			IsNotify:        false,
			NotifyType:      1, // 告警
			ErrMessage:      fmt.Sprintf("所有渠道发送失败: %s", strings.Join(sendErrors, "; ")),
		}
		log.Printf("[AlertNotifier] Strategy log (all failed): alertID=%d, strategyID=%d", event.ID, strategy.ID)
	}

	if err := n.strategyLogRepo.Create(logEntry); err != nil {
		log.Printf("[AlertNotifier] Failed to create strategy log: alertID=%d, strategyID=%d, error=%v", event.ID, strategy.ID, err)
	} else {
		log.Printf("[AlertNotifier] Successfully created strategy log: alertID=%d, strategyID=%d, logID=%d", event.ID, strategy.ID, logEntry.ID)
	}

	// 如果有任何发送失败，返回错误
	if len(sendErrors) > 0 {
		// 如果所有渠道都失败，返回错误
		if successCount == 0 {
			return fmt.Errorf("所有渠道发送失败: %s", strings.Join(sendErrors, "; "))
		}
		// 如果部分成功，不返回错误（继续处理）
	}

	return nil
}

// buildMessage 构建消息内容（支持模板变量替换）
func (n *AlertNotifier) buildMessage(event *model.AlertEvent, template *model.AlertTemplate, channel *model.AlertChannel, templateContent string, tags, annotations map[string]string, isRecovery bool) (string, string) {
	// 获取数据源名称
	sourceName := "Prometheus" // 默认值
	if n.ruleSourceRepo != nil && event.SourceID > 0 {
		if source, err := n.ruleSourceRepo.FindByID(event.SourceID); err == nil && source != nil {
			sourceName = source.SourceName
		}
	}

	// 构建详情链接
	alertPath := fmt.Sprintf("/monitors/alert-event/%d", event.ID)
	alertURL := alertPath
	if n.frontendURL != "" {
		// 如果配置了前端URL，构建完整URL
		baseURL := strings.TrimSuffix(n.frontendURL, "/")
		alertURL = fmt.Sprintf("%s%s", baseURL, alertPath)
	}

	// 准备变量映射
	variables := map[string]string{
		"alert_title":        event.AlertTitle,
		"alert_level":        fmt.Sprintf("%d", event.Level),
		"alert_level_name":   fmt.Sprintf("P%d", event.Level),
		"description":        event.Description,
		"trigger_time":       formatTime(event.TriggerTime),
		"first_trigger_time": formatTime(event.FirstTriggerTime),
		"recover_time":       formatTime(event.RecoverTime),
		"alert_status":       getAlertStatus(event.IsRecovered),
		"alert_id":           fmt.Sprintf("%d", event.ID),
		"source_ip":          event.SourceIP,
		"department_id":      event.DepartmentID,
		"source_name":        sourceName,
		"alert_url":          alertURL,
	}

	// 添加标签和注解到变量
	for k, v := range tags {
		variables[fmt.Sprintf("tag_%s", k)] = v
	}
	for k, v := range annotations {
		variables[fmt.Sprintf("annotation_%s", k)] = v
	}

	// 构建标题
	title := event.AlertTitle
	if title == "" {
		title = "告警通知"
	}
	if isRecovery {
		title = fmt.Sprintf("告警恢复: %s", title)
	}

	// 使用渠道模板内容，如果没有则返回空（需要在前端模板中配置）
	content := templateContent
	if content == "" {
		content = "" // 不再使用默认格式，需要在模板中配置
	}

	// 替换变量
	title = replaceVariables(title, variables)
	content = replaceVariables(content, variables)

	return title, content
}

// buildDefaultContent 构建默认内容模板（已废弃，不再使用默认格式）
// 默认格式已移除，需要在模板前端配置中设置示例格式
// 如果渠道模板内容为空，将返回空字符串
func (n *AlertNotifier) buildDefaultContent(event *model.AlertEvent, variables map[string]string, isRecovery bool) string {
	// 不再提供默认格式，需要在模板中配置
	return ""
}

// replaceVariables 替换模板变量
func replaceVariables(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}
	return result
}

// SendPlainMessage 向指定告警渠道发送纯文本/Markdown 消息（不依赖告警模板，用于 AI 巡检报告等）
func (n *AlertNotifier) SendPlainMessage(channelIDs []uint, title, content string) error {
	if len(channelIDs) == 0 {
		return nil
	}
	var errs []string
	for _, channelID := range channelIDs {
		channel, err := n.channelRepo.FindByID(channelID)
		if err != nil || channel == nil {
			errs = append(errs, fmt.Sprintf("渠道 %d: 未找到", channelID))
			continue
		}
		if err := n.sendToChannel(channel, title, content); err != nil {
			errs = append(errs, fmt.Sprintf("渠道 %d (%s): %v", channelID, channel.ChannelType, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// sendToChannel 根据渠道类型发送通知
func (n *AlertNotifier) sendToChannel(channel *model.AlertChannel, title, content string) error {
	// 解析渠道配置（channel_sign 存储了 webhook URL 和密钥等信息）
	var channelConfig map[string]interface{}
	if err := json.Unmarshal([]byte(channel.ChannelSign), &channelConfig); err != nil {
		// 如果不是 JSON，可能是直接的 webhook URL
		channelConfig = map[string]interface{}{
			"webhook": channel.ChannelSign,
		}
	}

	webhookURL, _ := channelConfig["webhook"].(string)
	secret, _ := channelConfig["secret"].(string)

	if len(webhookURL) == 0 {
		return fmt.Errorf("webhook URL is empty for channel %d", channel.ID)
	}

	switch channel.ChannelType {
	case "feishu":
		notifier := notification.NewFeishuNotifier(webhookURL, secret)
		if err := notifier.SendAlert(title, content); err != nil {
			return fmt.Errorf("飞书通知发送失败: %w", err)
		}
		return nil
	case "dingtalk":
		notifier := notification.NewDingTalkNotifier(webhookURL, secret)
		if err := notifier.SendAlert(title, content); err != nil {
			return fmt.Errorf("钉钉通知发送失败: %w", err)
		}
		return nil
	case "wechat":
		notifier := notification.NewWeChatNotifier(webhookURL)
		if err := notifier.SendAlert(title, content); err != nil {
			return fmt.Errorf("企业微信通知发送失败: %w", err)
		}
		return nil
	case "email", "sms", "webhook":
		// TODO: 实现其他渠道的通知器
		return fmt.Errorf("channel type %s not implemented", channel.ChannelType)
	default:
		return fmt.Errorf("unknown channel type: %s", channel.ChannelType)
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatTime 格式化时间
func formatTime(t *time.Time) string {
	if t == nil {
		return "未知"
	}
	return t.Format("2006-01-02 15:04:05")
}

// getAlertStatus 获取告警状态
func getAlertStatus(isRecovered bool) string {
	if isRecovered {
		return "resolved"
	}
	return "firing"
}

// formatTagsFromVariables 从变量中格式化标签
func formatTagsFromVariables(variables map[string]string) string {
	var parts []string
	for k, v := range variables {
		if strings.HasPrefix(k, "tag_") {
			tagKey := strings.TrimPrefix(k, "tag_")
			parts = append(parts, fmt.Sprintf("%s=%s", tagKey, v))
		}
	}
	if len(parts) == 0 {
		return "无"
	}
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

// formatAnnotationsFromVariables 从变量中格式化注解
func formatAnnotationsFromVariables(variables map[string]string) string {
	var parts []string
	for k, v := range variables {
		if strings.HasPrefix(k, "annotation_") {
			annKey := strings.TrimPrefix(k, "annotation_")
			parts = append(parts, fmt.Sprintf("%s=%s", annKey, v))
		}
	}
	if len(parts) == 0 {
		return "无"
	}
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

// SendRecoveryNotification 发送恢复通知
// 注意：告警通知使用告警渠道（alert-channel）配置，不依赖系统设置中的 NotificationManager
func (n *AlertNotifier) SendRecoveryNotification(event *model.AlertEvent, strategies []model.AlertStrategy, notificationManager *notification.NotificationManager) error {

	var tags map[string]string
	if err := json.Unmarshal(event.Tags, &tags); err != nil {
		return err
	}

	var annotations map[string]string
	if err := json.Unmarshal(event.Annotations, &annotations); err != nil {
		annotations = make(map[string]string)
	}

	// 为每个策略发送恢复通知
	for _, strategy := range strategies {
		// 必须配置模板才能发送通知
		if strategy.TemplateID == 0 {
			// 记录策略日志（未发送）
			strategyContent, _ := json.Marshal(strategy)
			logEntry := &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        false,
				NotifyType:      2, // 恢复
				ErrMessage:      "策略未配置告警模板",
			}
			if err := n.strategyLogRepo.Create(logEntry); err != nil {
				log.Printf("[Alert] Failed to create strategy log: %v", err)
			}
			continue
		}

		// 使用模板发送恢复通知（内部会记录日志）
		_ = n.sendRecoveryNotificationWithTemplate(event, strategy, tags, annotations)
	}

	return nil
}

// sendRecoveryNotificationWithTemplate 使用模板发送恢复通知
func (n *AlertNotifier) sendRecoveryNotificationWithTemplate(event *model.AlertEvent, strategy model.AlertStrategy, tags, annotations map[string]string) error {
	// 加载模板
	template, err := n.templateRepo.FindByID(strategy.TemplateID)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	if !template.Enable {
		return fmt.Errorf("template %d is disabled", template.ID)
	}

	// 解析模板的 channels 配置
	var channelsConfig map[string]interface{}
	if len(template.Channels) == 0 {
		channelsConfig = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(template.Channels, &channelsConfig); err != nil {
			return fmt.Errorf("failed to parse template channels: %w", err)
		}
	}

	// 收集所有渠道ID
	var channelIDs []uint
	for _, channelIDsInterface := range channelsConfig {
		if channelIDsArray, ok := channelIDsInterface.([]interface{}); ok {
			for _, idInterface := range channelIDsArray {
				if idFloat, ok := idInterface.(float64); ok {
					channelIDs = append(channelIDs, uint(idFloat))
				}
			}
		}
	}

	if len(channelIDs) == 0 {
		// 记录日志：模板未配置渠道
		strategyContent, _ := json.Marshal(strategy)
		logEntry := &model.StrategyLog{
			AlertID:         event.ID,
			StrategyID:      strategy.ID,
			StrategyContent: strategyContent,
			IsNotify:        false,
			NotifyType:      2, // 恢复
			ErrMessage:      fmt.Sprintf("告警模板 %d 未配置通知渠道，请前往 告警管理 → 告警模板 中为该模板配置渠道（飞书/钉钉/企业微信等）", template.ID),
		}
		if err := n.strategyLogRepo.Create(logEntry); err != nil {
			log.Printf("[Alert] Failed to create strategy log: %v", err)
		}
		return fmt.Errorf("template %d has no channels configured", template.ID)
	}

	// 加载渠道信息并发送通知
	var sendErrors []string
	successCount := 0
	for _, channelID := range channelIDs {
		channel, err := n.channelRepo.FindByID(channelID)
		if err != nil {
			continue
		}

		// 加载渠道模板内容
		channelTemplate, err := n.channelTemplateRepo.FindByTemplateIDAndChannelID(template.ID, channelID)
		if err != nil {
			continue
		}

		// 检查渠道模板是否存在
		if channelTemplate == nil {
			continue
		}

		// 检查模板是否完成配置
		if !channelTemplate.Finished {
			continue
		}

		// 构建消息内容（恢复通知，使用渠道模板内容）
		title, content := n.buildMessage(event, template, channel, channelTemplate.Content, tags, annotations, true)

		// 根据渠道类型创建通知器并发送
		if err := n.sendToChannel(channel, title, content); err != nil {
			// 记录错误，但继续处理其他渠道
			sendErrors = append(sendErrors, fmt.Sprintf("渠道 %d (%s): %v", channelID, channel.ChannelType, err))
		} else {
			successCount++
		}
	}

	// 记录通知日志（无论成功或失败）
	strategyContent, _ := json.Marshal(strategy)
	var logEntry *model.StrategyLog

	if successCount > 0 {
		// 至少有一个渠道成功
		if len(sendErrors) > 0 {
			// 部分成功
			logEntry = &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        true,
				NotifyType:      2, // 恢复
				ErrMessage:      fmt.Sprintf("部分渠道发送失败: %s", strings.Join(sendErrors, "; ")),
			}
		} else {
			// 全部成功
			logEntry = &model.StrategyLog{
				AlertID:         event.ID,
				StrategyID:      strategy.ID,
				StrategyContent: strategyContent,
				IsNotify:        true,
				NotifyType:      2, // 恢复
			}
		}
	} else {
		// 所有渠道都失败
		logEntry = &model.StrategyLog{
			AlertID:         event.ID,
			StrategyID:      strategy.ID,
			StrategyContent: strategyContent,
			IsNotify:        false,
			NotifyType:      2, // 恢复
			ErrMessage:      fmt.Sprintf("所有渠道发送失败: %s", strings.Join(sendErrors, "; ")),
		}
	}

	if err := n.strategyLogRepo.Create(logEntry); err != nil {
		log.Printf("[Alert] Failed to create strategy log: %v", err)
	}

	// 如果有任何发送失败，返回错误
	if len(sendErrors) > 0 {
		// 如果所有渠道都失败，返回错误
		if successCount == 0 {
			return fmt.Errorf("所有渠道发送失败: %s", strings.Join(sendErrors, "; "))
		}
		// 如果部分成功，不返回错误（继续处理）
	}

	return nil
}

// formatTags 格式化标签
func formatTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "无"
	}

	var parts []string
	for k, v := range tags {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("{%s}", fmt.Sprintf("%v", parts))
}

// formatAnnotations 格式化注解
func formatAnnotations(annotations map[string]string) string {
	if len(annotations) == 0 {
		return "无"
	}

	var parts []string
	for k, v := range annotations {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("{%s}", fmt.Sprintf("%v", parts))
}
