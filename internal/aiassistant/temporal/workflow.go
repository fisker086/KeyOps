package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	WorkflowIDPrefix     = "ai-assistant-schedule-"
	ActivityNameRunAndSend = "RunInspectionAndSendReport"
)

// ScheduleRunInput 定时任务执行输入
type ScheduleRunInput struct {
	ScheduleID string
}

// RunInspectionAndSendWorkflow 原子执行：巡检 + 发送报告到告警渠道（Temporal 保证执行与重试）
func RunInspectionAndSendWorkflow(ctx workflow.Context, input ScheduleRunInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 35 * time.Minute, // 巡检可能较久
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(ctx, ActivityNameRunAndSend, input).Get(ctx, nil)
}
