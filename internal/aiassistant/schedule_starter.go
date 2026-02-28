package aiassistant

import "context"

// ScheduleWorkflowStarter 定时任务工作流启动器（如 Temporal）：原子执行「巡检 + 发送报告到渠道」
// idempotencyKey 可选；非空时多实例下同一 key 只启一次（如 cron 时间槽），避免重复执行
type ScheduleWorkflowStarter interface {
	StartScheduleRun(ctx context.Context, scheduleID string, idempotencyKey string) error
}
