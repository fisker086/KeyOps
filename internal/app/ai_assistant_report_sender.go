package app

import (
	"context"
	"fmt"
	"os"

	alertnotification "github.com/fisker086/keyops/internal/alert/notification"
	"github.com/fisker086/keyops/internal/aiassistant"
	aitemporal "github.com/fisker086/keyops/internal/aiassistant/temporal"
)

type aiAssistantReportSender struct {
	notifier *alertnotification.AlertNotifier
}

// NewAIAssistantReportSender 创建巡检报告发送器，将报告发往告警渠道（飞书/钉钉/企业微信）
func NewAIAssistantReportSender(notifier *alertnotification.AlertNotifier) aiassistant.InspectionReportSender {
	if notifier == nil {
		return nil
	}
	return &aiAssistantReportSender{notifier: notifier}
}

func (s *aiAssistantReportSender) SendInspectionReport(channelIDs []uint, scheduleName, sessionID, status, summary string) error {
	title := "AI 巡检报告: " + scheduleName
	content := fmt.Sprintf("**会话**: %s\n**状态**: %s\n\n%s", sessionID, status, summary)
	return s.notifier.SendPlainMessage(channelIDs, title, content)
}

// temporalScheduleStarter 使用 Temporal 启动定时任务工作流（原子：巡检 + 发报告）
type temporalScheduleStarter struct {
	c *aitemporal.Client
}

func (t *temporalScheduleStarter) StartScheduleRun(ctx context.Context, scheduleID string, idempotencyKey string) error {
	if t.c == nil {
		return nil
	}
	return t.c.StartScheduleRun(ctx, scheduleID, idempotencyKey)
}

// NewTemporalScheduleStarter 当 TEMPORAL_HOST 与 TEMPORAL_TASK_QUEUE 已设置时创建并返回 starter，否则返回 nil
func NewTemporalScheduleStarter() aiassistant.ScheduleWorkflowStarter {
	host := os.Getenv("TEMPORAL_HOST")
	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if host == "" || taskQueue == "" {
		return nil
	}
	c, err := aitemporal.NewClient(host, taskQueue)
	if err != nil {
		return nil
	}
	return &temporalScheduleStarter{c: c}
}
