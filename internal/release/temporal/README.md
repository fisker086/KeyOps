# 生产发布 Temporal 编排

生产环境发布（及回滚）可通过 Temporal 编排，实现可观测、重试与后续扩展（如失败自动回滚）。设计详见 [DESIGN.md](../DESIGN.md)。

## 流程简述

1. 用户申请 prod 发布（或回滚）→ 创建 Approval。
2. 审批通过后：若已配置 Temporal，则**启动 Temporal Workflow**（WorkflowID 与 approval/release_run 关联）；否则直接调用 `ExecuteDeployment`。
3. Workflow 内 Activity：执行部署（调 Jenkins/ArgoCD，带 strategy 参数）；后续可扩展：等待部署结果、健康检查、失败时触发回滚等。
4. 未配置 Temporal 时，审批通过后仍直接调用 `doExecuteRun`，行为与当前一致。

## 启用方式

### 1. 部署 Temporal Server

例如使用 Docker 一键启动（含 Web UI、Cassandra/PostgreSQL 等）：

```bash
docker run -d --name temporal \
  -p 7233:7233 \
  -e DB=postgresql \
  -e POSTGRES_USER=temporal \
  -e POSTGRES_PWD=temporal \
  -e POSTGRES_SEEDS=host.docker.internal \
  temporalio/auto-setup
```

或查看 [Temporal 官方文档](https://docs.temporal.io/self-hosted-guide) 自建集群。

### 2. 配置

在应用配置中增加 Temporal 地址与 TaskQueue。后续可在 `config.yaml` 中增加 `release.temporal` 节，例如：

```yaml
# config.yaml 示例（需在 bootstrap 中解析并传入）
release:
  temporal:
    host_port: "localhost:7233"
    task_queue: "release-deploy"
```

当前若尚无统一 config 节，可在初始化代码中写死或从环境变量读取，例如：

- `TEMPORAL_HOST_PORT`（默认 `localhost:7233`）
- `TEMPORAL_TASK_QUEUE`（默认 `release-deploy`）

### 3. 注入客户端

在 `app/services.go` 或主程序初始化处：

1. 使用上述配置创建 `temporal.Client`（调用 `temporal.NewClient(hostPort, taskQueue)`）。
2. 调用 `releaseService.SetDeployProdStarter(temporalClient)`。

审批通过后，Release 服务将改为调用 `starter.StartDeployProd(...)` 启动 Workflow，而非直接执行部署。

### 4. 运行 Worker

Worker 需在**单独进程**或**同一进程**中运行，注册 `DeployProdWorkflow` 与 `Activities.ExecuteDeploy`，TaskQueue 与客户端一致。

**方式 A：同一进程内启动 Worker**

在 `cmd/server` 或 bootstrap 中，在启动 HTTP 服务的同时启动 Worker。Worker 需使用 `go.temporal.io/sdk/client` 直接 Dial（与 `temporal.NewClient` 使用相同 hostPort 与 taskQueue）：

```go
// 伪代码示例
import "go.temporal.io/sdk/client"
import "go.temporal.io/sdk/worker"
import "go.temporal.io/sdk/activity"

temporalClient, _ := temporal.NewClient(hostPort, taskQueue)
releaseService.SetDeployProdStarter(temporalClient)
// Worker 使用 SDK 的 client 创建（同一 hostPort、taskQueue）
c, _ := client.Dial(client.Options{HostPort: hostPort})
w, _ := worker.New(c, taskQueue, worker.Options{})
w.RegisterWorkflow(temporal.DeployProdWorkflow)
w.RegisterActivityWithOptions(&temporal.Activities{ReleaseService: releaseSvc}, activity.RegisterOptions{
    Name: temporal.ActivityNameExecuteDeploy,
})
go w.Run(worker.InterruptCh())
```

**方式 B：独立 Worker 进程（推荐生产）**

单独的可执行文件（如 `cmd/release-worker`），使用同一 Temporal 地址与 TaskQueue，并注入与主应用相同的 ReleaseService（读同一 DB、Jenkins 配置等）：

```go
// cmd/release-worker/main.go 示例
c, _ := client.Dial(client.Options{HostPort: os.Getenv("TEMPORAL_HOST_PORT")})
taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
w, _ := worker.New(c, taskQueue, worker.Options{})
w.RegisterWorkflow(temporal.DeployProdWorkflow)
w.RegisterActivityWithOptions(&temporal.Activities{ReleaseService: releaseSvc}, activity.RegisterOptions{
    Name: temporal.ActivityNameExecuteDeploy,
})
w.Run(worker.InterruptCh())
```

### 5. WorkflowID 约定

- 当前实现：`deploy-prod-{runID}`，与 `release_run.id` 一一对应，便于在 Temporal Web UI 中按 run 查询。
- 可选：在 Workflow 输入或 Search Attributes 中写入 `approval_id`，便于按审批单追踪。

## 可选扩展（Phase 4+）

- **健康检查 Activity**：部署完成后轮询或回调，确认实例健康再完成 Workflow。
- **失败自动回滚**：在 Workflow 中捕获 `ExecuteDeploy` 失败，再调用一次部署（回滚到上一版本）或发送告警。
- **超时与重试**：已在 Workflow 中配置 `StartToCloseTimeout: 10*time.Minute` 与 `RetryPolicy.MaximumAttempts: 3`，可按需调整。

未配置 Temporal 时，审批通过后仍直接调用 `ExecuteDeployment`，行为与当前一致。
