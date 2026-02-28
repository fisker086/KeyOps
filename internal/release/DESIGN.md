# 发布代码流程设计（CodeStar / Temporal / React Flow 演进）

## 当前已实现（Phase 1）

- **Webhook → 记录**：Git push 触发 `POST /api/release/webhook`，落库 `release_runs`。
- **执行**：用户对某条记录选择**环境**后执行：
  - **dev / test / qa / staging**：直接触发 Jenkins（按应用绑定 + 环境）。
  - **prod**：不直接执行，创建**发布审批单**（Approval type=deployment）；审批通过后自动执行（或通过 Temporal 编排执行）。

## 环境与绑定

- 环境枚举：`dev` / `test` / `qa` / `staging` / `prod`（与「应用-发布」绑定表 `environment` 一致）。
- 每个环境对应一套 Jenkins/ArgoCD 绑定；执行时按「应用 + 环境」选绑定。

## 生产发布：回滚（Phase 2）

- **除首次发布外，生产必须支持回滚**。系统记录每次 prod 成功发布的 run，提供「回滚到上一版本」能力。
- **回滚**：针对某应用，创建一条新的 release_run，commit 取「上一版 prod 成功 run」的 repo/branch/commit_sha，标记为回滚（`rollback_from_run_id`），同样走 prod 工单审批，审批通过后执行部署。
- 接口：`GET /api/release/applications/:id/last-prod-run` 查询当前 prod 最新成功 run；`POST /api/release/rollback` 提交回滚申请（创建 run + 创建审批单）。

## 发布策略：蓝绿 / 金丝雀 / 多泳道（Phase 3）

- 在「应用-发布」绑定上支持**发布策略**（或单次执行时指定）：
  - **rolling**：滚动发布（默认）。
  - **blue_green**：蓝绿发布。
  - **canary**：金丝雀（可配流量比例等）。
  - **multi_lane**：多泳道发布。
- 绑定表增加：`deploy_strategy`、`strategy_options`（JSON，如 canary 比例、泳道 ID）。执行 Jenkins 时通过参数传入（如 `DEPLOY_STRATEGY`、`CANARY_WEIGHT`、`LANE_ID`），由 Jenkins/ArgoCD 流水线实现具体逻辑。

## Temporal 生产工单编排（Phase 4）

- **生产环境**发布与回滚采用 **Temporal Workflow** 编排，便于重试、超时、可观测和后续扩展（如审批通过 → 部署 → 健康检查 → 自动回滚）。
- **流程**：
  1. 用户申请 prod 发布（或回滚）→ 创建 Approval。
  2. 审批通过后：**启动 Temporal Workflow**（WorkflowID 与 approval/release_run 关联）。
  3. Workflow 内 Activity：执行部署（调 Jenkins/ArgoCD，带 strategy 参数）；可选 Activity：等待部署结果、失败时触发回滚等。
  4. 若未配置 Temporal，则沿用当前「审批通过 → 直接调用 doExecuteRun」。
- 部署方式：配置中启用 Temporal 时，Approval 通过后由 Release 服务启动 Workflow；Worker 在独立进程或同一进程中注册并执行 Activity。

## 流程示意（含回滚与 Temporal）

```
[Git Push] → Webhook → release_runs (pending)
                              ↓
用户点击「执行」→ 选择环境
    ├─ dev/test/qa/staging → 直接触发 Jenkins（带 strategy 参数）→ run.status = running
    └─ prod                → 创建 Approval(deployment, pending)
                                    ↓
                            审批人通过
                                    ↓
                    ┌───────────────┴───────────────┐
                    │ Temporal 已配置?               │
                    ├─ 是 → 启动 DeployProdWorkflow  │
                    │        → Activity: 执行部署    │
                    │        → （可选）失败则回滚    │
                    └─ 否 → 直接 doExecuteRun       │
                                    ↓
                            Jenkins/ArgoCD 执行（DEPLOY_STRATEGY 等参数）

回滚：
  用户点击「回滚」→ GET last-prod-run → POST /rollback → 创建 run(rollback_from_run_id) + Approval
      → 审批通过 → 同上（Temporal 或直接执行），部署上一版本 commit
```

## 数据与接口约定

- **ReleaseRun**：增加 `rollback_from_run_id`（回滚时指向被回滚的 run）、可选 `deploy_strategy`（单次覆盖绑定默认）。
- **ApplicationDeployBinding**：增加 `deploy_strategy`（rolling|blue_green|canary|multi_lane）、`strategy_options`（JSON）。
- **Approval.DeployConfig**（type=deployment 时）：  
  `{"release_run_id":"uuid","environment":"prod","application_id":"uuid"}`  
  审批通过后由后端解析并执行发布（或启动 Temporal Workflow）。
- **执行接口**：`POST /api/release/runs/:id/execute` body: `{ "environment": "..." }`。
- **回滚接口**：`GET /api/release/applications/:id/last-prod-run`；`POST /api/release/rollback` body: `{ "application_id": "uuid", "run_id?": "可选，不传则用 last success" }`。

## 后续演进（Phase 5+）

- **React Flow**：发布代码页流水线视图，节点 = 构建/测试/部署各环境，边 = 依赖。
- **流水线按图执行**：前端保存的流水线定义（nodes/edges）需可被后端按图执行。节点类型与执行语义见 [PIPELINE_EXECUTION.md](./PIPELINE_EXECUTION.md)，仅使用约定好的 6 类节点（trigger / create_run / branch_by_env / deploy / approval / end），后端即可完全根据用户定义的流程实现执行。
- **CodeStar 式**：从提交到各环境状态一条线展示，环境门控、审计与工单打通。
