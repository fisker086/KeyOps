package temporal

import (
	"context"

	"github.com/fisker086/keyops/internal/aiassistant"
)

// Activities 供 Temporal Worker 注册
type Activities struct {
	Handler *aiassistant.Handler
}

// RunInspectionAndSendReport 原子执行：创建会话 → 跑巡检 → 发送报告到配置的告警渠道
func (a *Activities) RunInspectionAndSendReport(ctx context.Context, input ScheduleRunInput) error {
	if a.Handler == nil {
		return nil
	}
	s, err := a.Handler.GetSchedule(input.ScheduleID)
	if err != nil || s == nil {
		return err
	}
	createdBy := s.ResponsibleUser
	if createdBy == "" {
		createdBy = "system"
	}
	sessionID, err := a.Handler.CreateSessionForSchedule(input.ScheduleID, createdBy)
	if err != nil {
		return err
	}
	a.Handler.RunAgentCore(sessionID)
	a.Handler.SendReportIfNeeded(sessionID)
	return nil
}
