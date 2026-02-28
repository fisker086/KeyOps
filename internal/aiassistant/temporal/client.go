package temporal

import (
	"context"
	"fmt"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// Client 用于触发 AI 助手定时任务工作流（原子：巡检 + 发报告）
type Client struct {
	c        client.Client
	taskQueue string
}

// NewClient 创建 Temporal 客户端
func NewClient(hostPort, taskQueue string) (*Client, error) {
	if hostPort == "" || taskQueue == "" {
		return nil, fmt.Errorf("temporal host and task_queue required")
	}
	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return nil, err
	}
	return &Client{c: c, taskQueue: taskQueue}, nil
}

// StartScheduleRun 异步启动定时任务工作流（巡检 + 发送报告到渠道）
// idempotencyKey 非空时用作 WorkflowID 后缀并 RejectDuplicate，多实例同一时间槽只会有一个 run
func (c *Client) StartScheduleRun(ctx context.Context, scheduleID string, idempotencyKey string) error {
	var wfID string
	opts := client.StartWorkflowOptions{
		TaskQueue: c.taskQueue,
	}
	if idempotencyKey != "" {
		wfID = fmt.Sprintf("%s%s-%s", WorkflowIDPrefix, scheduleID, idempotencyKey)
		opts.ID = wfID
		opts.WorkflowIDReusePolicy = enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
	} else {
		wfID = fmt.Sprintf("%s%s-%d", WorkflowIDPrefix, scheduleID, time.Now().UnixNano())
		opts.ID = wfID
	}
	_, err := c.c.ExecuteWorkflow(ctx, opts, RunInspectionAndSendWorkflow, ScheduleRunInput{ScheduleID: scheduleID})
	return err
}

// Close 关闭客户端
func (c *Client) Close() {
	if c.c != nil {
		c.c.Close()
	}
}
