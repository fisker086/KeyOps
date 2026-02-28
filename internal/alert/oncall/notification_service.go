package oncall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/logger"

	"gorm.io/gorm"
)

// OnCallNotificationService 值班通知服务
type OnCallNotificationService struct {
	db            *gorm.DB
	shiftRepo     *repository.OnCallShiftRepository
	scheduleRepo  *repository.OnCallScheduleRepository
	stopChan      chan struct{}
	isRunning     bool
	checkInterval time.Duration
}

// NewOnCallNotificationService 创建值班通知服务
func NewOnCallNotificationService(
	db *gorm.DB,
	shiftRepo *repository.OnCallShiftRepository,
	scheduleRepo *repository.OnCallScheduleRepository,
) *OnCallNotificationService {
	return &OnCallNotificationService{
		db:            db,
		shiftRepo:    shiftRepo,
		scheduleRepo: scheduleRepo,
		stopChan:     make(chan struct{}),
		checkInterval: 1 * time.Minute, // 默认每分钟检查一次
	}
}

// Start 启动值班通知服务
func (s *OnCallNotificationService) Start(ctx context.Context) error {
	if s.isRunning {
		return fmt.Errorf("on-call notification service is already running")
	}

	s.isRunning = true
	logger.Infof("On-call notification service started, check interval: %v", s.checkInterval)

	// 启动定时检查
	go s.runPeriodicCheck(ctx)

	return nil
}

// Stop 停止值班通知服务
func (s *OnCallNotificationService) Stop() {
	if !s.isRunning {
		return
	}

	close(s.stopChan)
	s.isRunning = false
	logger.Infof("On-call notification service stopped")
}

// runPeriodicCheck 运行定期检查
func (s *OnCallNotificationService) runPeriodicCheck(ctx context.Context) {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// 延迟执行首次检查
	time.Sleep(5 * time.Second)
	s.performCheck(ctx)

	for {
		select {
		case <-ticker.C:
			s.performCheck(ctx)
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performCheck 执行检查
func (s *OnCallNotificationService) performCheck(ctx context.Context) {
	logger.Debugf("Starting on-call shift notification check...")

	// 检查即将开始或刚刚开始的班次（检查最近1分钟到未来5分钟的时间窗口）
	now := time.Now()
	checkStartTime := now.Add(-1 * time.Minute) // 检查1分钟前开始的班次（避免漏掉）
	checkEndTime := now.Add(5 * time.Minute)    // 检查未来5分钟内开始的班次

	// 查找在这个时间窗口内开始的班次
	var shifts []model.OnCallShift
	err := s.db.Where("start_time >= ? AND start_time <= ?", checkStartTime, checkEndTime).
		Where("status = ?", "active").
		Find(&shifts).Error

	if err != nil {
		logger.Errorf("Failed to query shifts: %v", err)
		return
	}

	logger.Debugf("Found %d shifts starting soon", len(shifts))

	// 为每个班次发送通知
	for _, shift := range shifts {
		// 检查班次开始时间是否在合理范围内（避免发送太早或太晚的通知）
		timeUntilStart := shift.StartTime.Sub(now)
		if timeUntilStart < -2*time.Minute || timeUntilStart > 5*time.Minute {
			// 如果班次开始时间不在合理范围内，跳过
			continue
		}

		// 检查是否已经发送过通知（通过检查更新时间）
		// 如果班次在检查时间窗口内，且更新时间很接近创建时间，说明可能还没发送过通知
		timeSinceUpdated := now.Sub(shift.UpdatedAt)
		if timeSinceUpdated > 2*time.Minute && shift.UpdatedAt.After(shift.CreatedAt.Add(1*time.Minute)) {
			// 如果更新时间超过2分钟，且更新时间明显晚于创建时间，可能已经处理过了
			// 这里使用一个简单的启发式方法：如果更新时间在创建时间后1分钟以上，且距离现在超过2分钟，跳过
			logger.Debugf("Shift %d may have been processed already (updated %v ago)", shift.ID, timeSinceUpdated)
			continue
		}

		// 发送通知
		if err := s.sendShiftStartNotification(ctx, &shift); err != nil {
			logger.Errorf("Failed to send notification for shift %d: %v", shift.ID, err)
		} else {
			// 更新班次的更新时间，标记为已处理（避免重复发送）
			s.db.Model(&shift).Update("updated_at", now)
			logger.Infof("Notification sent for shift %d (user: %s, start: %s)", 
				shift.ID, shift.UserID, shift.StartTime.Format("2006-01-02 15:04:05"))
		}
	}

	logger.Debugf("On-call shift notification check completed")
}

// sendShiftStartNotification 发送班次开始通知
func (s *OnCallNotificationService) sendShiftStartNotification(ctx context.Context, shift *model.OnCallShift) error {
	// 获取排班信息
	schedule, err := s.scheduleRepo.FindByID(shift.ScheduleID)
	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}

	// 检查是否配置了通知webhook
	if schedule.NotificationWebhook == "" {
		logger.Debugf("Schedule %d has no notification webhook configured, skipping notification", schedule.ID)
		return nil
	}

	// 获取用户信息
	var user model.User
	if err := s.db.Where("id = ?", shift.UserID).First(&user).Error; err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// 构建通知消息
	title := "📢 值班开始提醒"
	content := fmt.Sprintf(
		"**排班名称**：%s\n"+
			"**值班人员**：%s (%s)\n"+
			"**开始时间**：%s\n"+
			"**结束时间**：%s\n"+
			"**班次类型**：%s\n\n"+
			"值班已开始，请关注告警信息！",
		schedule.ScheduleName,
		user.FullName,
		user.Username,
		shift.StartTime.Format("2006-01-02 15:04:05"),
		shift.EndTime.Format("2006-01-02 15:04:05"),
		shift.ShiftType,
	)

	// 发送企业微信消息（支持企业微信、飞书、钉钉等）
	return s.sendWebhookNotification(schedule.NotificationWebhook, title, content)
}

// sendWebhookNotification 发送webhook通知（支持企业微信、飞书、钉钉等）
func (s *OnCallNotificationService) sendWebhookNotification(webhookURL, title, content string) error {
	// 判断webhook类型（通过URL判断）
	// 企业微信: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx
	// 飞书: https://open.feishu.cn/open-apis/bot/v2/hook/xxx
	// 钉钉: https://oapi.dingtalk.com/robot/send?access_token=xxx

	message := map[string]interface{}{}

	// 根据URL判断类型
	if contains(webhookURL, "qyapi.weixin.qq.com") {
		// 企业微信
		message = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"content": fmt.Sprintf("## %s\n\n%s", title, content),
			},
		}
	} else if contains(webhookURL, "open.feishu.cn") {
		// 飞书
		message = map[string]interface{}{
			"msg_type": "interactive",
			"card": map[string]interface{}{
				"config": map[string]interface{}{
					"wide_screen_mode": true,
				},
				"elements": []map[string]interface{}{
					{
						"tag": "div",
						"text": map[string]interface{}{
							"tag":     "lark_md",
							"content": fmt.Sprintf("**%s**\n\n%s", title, content),
						},
					},
				},
			},
		}
	} else if contains(webhookURL, "oapi.dingtalk.com") {
		// 钉钉
		message = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"title": title,
				"text":  content,
			},
		}
	} else {
		// 默认使用企业微信格式
		message = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"content": fmt.Sprintf("## %s\n\n%s", title, content),
			},
		}
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned non-200 status: %d", resp.StatusCode)
	}

	logger.Infof("On-call shift notification sent successfully to %s", webhookURL)
	return nil
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

