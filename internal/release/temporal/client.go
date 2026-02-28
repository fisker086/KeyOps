package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
)

// Client 用于在生产审批通过后启动 DeployProdWorkflow（可选）
type Client struct {
	c        client.Client
	taskQueue string
}

// NewClient 创建 Temporal 客户端（需已运行的 Temporal Server）
func NewClient(hostPort string, taskQueue string) (*Client, error) {
	if hostPort == "" || taskQueue == "" {
		return nil, fmt.Errorf("temporal host and task_queue required")
	}
	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return nil, err
	}
	return &Client{c: c, taskQueue: taskQueue}, nil
}

// StartDeployProd 异步启动生产发布 Workflow
func (c *Client) StartDeployProd(ctx context.Context, runID, applicationID, environment string) error {
	_, err := c.c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "deploy-prod-" + runID,
		TaskQueue: c.taskQueue,
	}, DeployProdWorkflow, DeployProdInput{
		RunID:         runID,
		ApplicationID: applicationID,
		Environment:   environment,
	})
	return err
}

// Close 关闭客户端
func (c *Client) Close() {
	if c.c != nil {
		c.c.Close()
	}
}
