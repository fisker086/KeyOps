package temporal

import (
	"time"

	"go.temporal.io/sdk/workflow"
	"go.temporal.io/sdk/temporal"
)

// DeployProdInput 生产发布 Workflow 输入
type DeployProdInput struct {
	RunID         string
	ApplicationID string
	Environment   string
}

// DeployProdWorkflow 生产发布编排：执行部署 Activity，后续可扩展（如等待结果、失败回滚等）
func DeployProdWorkflow(ctx workflow.Context, input DeployProdInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(ctx, ActivityNameExecuteDeploy, input).Get(ctx, nil)
}
