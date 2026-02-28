package certificate

import (
	"encoding/json"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"time"

	"github.com/fisker086/keyops/internal/alert/notification"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"gorm.io/gorm"
)

// CertificateAlertService 证书告警服务
type CertificateAlertService struct {
	certRepo            *repository.DomainCertificateRepository
	templateRepo        *repository.AlertTemplateRepository
	channelRepo         *repository.AlertChannelRepository
	alertNotifier       *notification.AlertNotifier
	db                  *gorm.DB
}

// NewCertificateAlertService 创建证书告警服务
func NewCertificateAlertService(
	certRepo *repository.DomainCertificateRepository,
	templateRepo *repository.AlertTemplateRepository,
	channelRepo *repository.AlertChannelRepository,
	alertNotifier *notification.AlertNotifier,
	db *gorm.DB,
) *CertificateAlertService {
	return &CertificateAlertService{
		certRepo:      certRepo,
		templateRepo:  templateRepo,
		channelRepo:   channelRepo,
		alertNotifier: alertNotifier,
		db:            db,
	}
}

// CheckAndSendAlerts 检查证书过期情况并发送告警
// 这个方法会被定时任务调用，每天执行一次
func (s *CertificateAlertService) CheckAndSendAlerts() error {
	logger.Info("[CertificateAlertService] Starting certificate expiration check...")

	// 获取所有启用了监控的证书
	_, certs, err := s.certRepo.List(1, 10000, "") // 获取所有证书
	if err != nil {
		return fmt.Errorf("failed to fetch certificates: %w", err)
	}

	now := time.Now()
	alertCount := 0

	for _, cert := range certs {
		// 跳过未启用监控的证书
		if !cert.IsMonitor {
			continue
		}

		// 跳过没有过期时间的证书
		if cert.ExpireTime == nil {
			continue
		}

		// 计算剩余天数
		expireTime := *cert.ExpireTime
		daysLeft := int(time.Until(expireTime).Hours() / 24)

		// 检查是否需要发送告警
		if daysLeft <= cert.AlertDays {
			// 检查今天是否已经发送过告警（防止重复发送）
			shouldSend := false
			if cert.LastAlertTime == nil {
				// 从未发送过告警，需要发送
				shouldSend = true
			} else {
				// 检查最后一次告警时间是否是今天
				lastAlertDate := cert.LastAlertTime.Format("2006-01-02")
				today := now.Format("2006-01-02")
				if lastAlertDate != today {
					// 今天还没发送过，需要发送
					shouldSend = true
				}
			}

			if shouldSend {
				// 发送告警
				if err := s.sendAlert(&cert, daysLeft); err != nil {
					logger.Infof("[CertificateAlertService] Failed to send alert for certificate %d (%s): %v", cert.ID, cert.Domain, err)
					continue
				}

				// 更新最后告警时间
				now := time.Now()
				cert.LastAlertTime = &now
				if err := s.certRepo.Update(&cert); err != nil {
					logger.Infof("[CertificateAlertService] Failed to update last_alert_time for certificate %d: %v", cert.ID, err)
				}

				alertCount++
				logger.Infof("[CertificateAlertService] Alert sent for certificate %d (%s), %d days remaining", cert.ID, cert.Domain, daysLeft)
			} else {
				logger.Infof("[CertificateAlertService] Alert already sent today for certificate %d (%s), skipping", cert.ID, cert.Domain)
			}
		}
	}

	logger.Infof("[CertificateAlertService] Certificate expiration check completed, sent %d alerts", alertCount)
	return nil
}

// sendAlert 发送证书过期告警
func (s *CertificateAlertService) sendAlert(cert *model.DomainCertificate, daysLeft int) error {
	// 如果没有配置告警模板，跳过
	if cert.AlertTemplateID == nil || *cert.AlertTemplateID == 0 {
		logger.Infof("[CertificateAlertService] Certificate %d has no alert template configured, skipping", cert.ID)
		return nil
	}

	// 加载告警模板
	template, err := s.templateRepo.FindByID(*cert.AlertTemplateID)
	if err != nil {
		logger.Infof("[CertificateAlertService] Failed to load template %d: %v", *cert.AlertTemplateID, err)
		return fmt.Errorf("failed to load template: %w", err)
	}

	if !template.Enable {
		logger.Infof("[CertificateAlertService] Template %d is disabled, skipping", template.ID)
		return nil
	}

	// 检查模板是否配置了渠道（模板中已经配置了渠道，不需要证书单独配置）
	var channelsConfig map[string]interface{}
	if len(template.Channels) == 0 {
		logger.Infof("[CertificateAlertService] Template %d has no channels configured, skipping", template.ID)
		return fmt.Errorf("template %d has no channels configured", template.ID)
	}
	
	if err := json.Unmarshal(template.Channels, &channelsConfig); err != nil {
		logger.Infof("[CertificateAlertService] Failed to parse template %d channels JSON: %v", template.ID, err)
		return fmt.Errorf("failed to parse template channels: %w", err)
	}
	
	// 检查是否有渠道配置
	hasChannels := false
	for _, channelIDsInterface := range channelsConfig {
		if channelIDsArray, ok := channelIDsInterface.([]interface{}); ok && len(channelIDsArray) > 0 {
			hasChannels = true
			break
		}
	}
	
	if !hasChannels {
		logger.Infof("[CertificateAlertService] Template %d has no channels configured, skipping", template.ID)
		return fmt.Errorf("template %d has no channels configured", template.ID)
	}

	// 构建告警内容
	status := "即将过期"
	if daysLeft < 0 {
		status = "已过期"
	}

	now := time.Now()
	expireTimeStr := ""
	if cert.ExpireTime != nil {
		expireTimeStr = cert.ExpireTime.Format("2006-01-02 15:04:05")
	}

	// 构建标签和注解（用于模板变量替换）
	// 简化变量：标签只保留必要的标识信息，详细信息放在注解中
	tags := map[string]string{
		"alertname": "certificate_expiration",
		"domain":    cert.Domain,
	}

	// 注解包含详细的证书信息（统一使用 annotation_ 前缀）
	annotations := map[string]string{
		"domain":      cert.Domain,
		"port":        fmt.Sprintf("%d", cert.Port),
		"expire_time": expireTimeStr,
		"days_left":   fmt.Sprintf("%d", daysLeft),
		"status":      status,
	}

	tagsJSON, _ := json.Marshal(tags)
	annotationsJSON, _ := json.Marshal(annotations)

	// 创建临时告警事件用于发送通知（AlertNotifier 需要 AlertEvent）
	alertEvent := &model.AlertEvent{
		AlertTitle:       fmt.Sprintf("证书过期告警 - %s", cert.Domain),
		Description:      fmt.Sprintf("证书 %s:%d 剩余 %d 天过期，请及时更新", cert.Domain, cert.Port, daysLeft),
		Level:            1, // 告警等级
		FirstTriggerTime: &now,
		TriggerTime:      &now,
		Tags:             tagsJSON,
		Annotations:      annotationsJSON,
		FingerPrint:      fmt.Sprintf("certificate_%d_%s_%d", cert.ID, cert.Domain, cert.Port),
		IsRecovered:      false,
		Progress:         1, // 未认领
	}

	// 构建策略（使用证书配置的模板）
	strategy := model.AlertStrategy{
		ID:         0, // 临时策略ID（证书告警不使用策略）
		TemplateID: *cert.AlertTemplateID,
	}

	// 使用 AlertNotifier 发送通知
	// 注意：AlertNotifier.sendNotificationWithTemplate 会从模板中读取渠道配置
	// 模板中已经配置了渠道，直接使用模板的渠道配置即可
	if s.alertNotifier != nil {
		// 调用 AlertNotifier 的发送方法
		// 注意：这里传入 nil 作为 NotificationManager，因为 AlertNotifier 不依赖它
		// AlertNotifier 会自动从模板中读取渠道配置并发送通知
		// 变量会通过 tags 和 annotations 传递，模板中可以使用 {{tag_domain}}, {{annotation_domain}} 等变量
		if err := s.alertNotifier.SendNotification(alertEvent, []model.AlertStrategy{strategy}, nil); err != nil {
			logger.Infof("[CertificateAlertService] Failed to send alert via AlertNotifier: %v", err)
			return err
		}
		
		logger.Infof("[CertificateAlertService] Alert sent successfully for certificate %d (%s:%d), %d days remaining",
			cert.ID, cert.Domain, cert.Port, daysLeft)
	} else {
		logger.Infof("[CertificateAlertService] AlertNotifier is nil, cannot send alert")
		return fmt.Errorf("alertNotifier is nil")
	}

	return nil
}
