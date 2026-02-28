# 流水线定义与按图执行

前端「流水线」页保存的图（nodes + edges）需可被后端**按图执行**，不能是随意画着玩的节点。此处约定节点类型与执行语义，后端实现「执行流水线」时严格按此解释。

## 当前后端状态

**后端目前没有按图执行的流程。** 仅提供：
- `GET /api/release/pipeline`：读取保存的流水线定义（nodes + edges）
- `PUT /api/release/pipeline`：保存流水线定义

实际发布执行仍为 `release_service.ExecuteRun` 的**固定逻辑**（选环境 → prod 走审批、非 prod 直接 Jenkins）。要「完全根据用户画的流程执行」，需在后端增加**流水线解释器**：从保存的图中按下面约定解析并依次执行节点（或转为 Temporal/Argo 再执行）。

## Argo Workflows 的 DAG 定义方式

[Argo Workflows](https://argo-workflows.readthedocs.io/en/latest/walk-through/dag/) 用 **DAG（有向无环图）** 定义工作流：
- **templates**：每个模板对应一类可执行步骤（类似我们的节点类型）。
- **dag.tasks**：每个 task 有 `name`、`template`、`dependencies: [A, B]`（依赖的 task 名），即边用「依赖」表示（task D 依赖 B、C → 执行顺序为 B、C 完成后执行 D）。

我们的拖拉定义与之一致：**节点 = 可执行步骤，边 = 依赖/顺序**。存储格式（nodes + edges）可等价转换为 Argo 风格（templates + tasks with dependencies），便于后续对接 Argo 或自研执行引擎时复用同一套定义。

## 节点类型（node.type）

| type | 含义 | 执行语义 |
|------|------|----------|
| `trigger` | 触发 | 流程入口，无入边。执行时：表示一次发布流程开始（如 Webhook 或用户点击执行），上下文带入 run_id 或由本节点前序逻辑创建。 |
| `create_run` | 创建记录 | 若上下文中尚无 run_id：创建 release_run（pending）；若有则沿用。执行后沿出边继续。 |
| `branch_by_env` | 按环境分支 | 根据**当前执行环境**（dev/test/qa/staging/prod）选择出边。约定：**sourceHandle `a`** = 非 prod（dev/test/qa/staging），**sourceHandle `b`** = prod。执行时只走一条出边。 |
| `deploy` | 部署 | 调用现有部署逻辑：按「应用 + 当前环境」选绑定，触发 Jenkins/ArgoCD。节点 `data.env` 可写死环境（如 `prod`），不写则用执行上下文中的环境。执行后沿出边继续。 |
| `approval` | 审批 | 创建发布审批单（Approval type=deployment）；**不阻塞**执行线程，审批通过后由回调/工单流程继续执行「该节点的后继子图」（见下）。执行引擎记录「当前停在 approval 节点，approval_id = xxx」。 |
| `end` | 结束 | 流程终止，无出边。 |

## 边的约定

- **普通边**：顺序执行，从 source 执行完后到 target。
- **branch_by_env 的出边**：必须用 `sourceHandle` 区分：
  - `a`：当前环境 ∈ { dev, test, qa, staging } 时走这条边。
  - `b`：当前环境 === prod 时走这条边。
- 边上可带 `data.env` 覆盖下一段执行的环境（可选）。

## 执行流程（后端如何按图执行）

1. **入口**：从类型为 `trigger` 的节点开始（若有多个 trigger，可约定取第一个或按 run 来源选一个）。
2. **遍历**：按有向边依次执行节点：
   - `trigger` → 不做实质动作，进入下一节点。
   - `create_run` → 确保有 run（创建或复用），写入上下文。
   - `branch_by_env` → 根据上下文环境选一条出边（a 或 b），只沿该边进入下一节点。
   - `deploy` → 调现有 `ExecuteRun`/Jenkins 逻辑，完成后沿出边继续。
   - `approval` → 创建审批单，当前执行在此挂起；审批通过后由回调根据 approval 关联的 run_id/流程状态，从该节点的**出边**继续执行后继节点（如 deploy）。
   - `end` → 结束，不再有后继。
3. **上下文**：执行过程中维护「当前 run_id、当前 environment、当前 approval_id（若在等审批）」等，供各节点使用。

## 与现有实现的对应

- 当前逻辑（DESIGN.md）：Webhook → 落库 run → 用户选环境 → dev/test/qa/staging 直接 Jenkins，prod 创建审批 → 审批通过 → Temporal 或直接 doExecuteRun → Jenkins。
- 对应到上述节点：**trigger** → **create_run** → **branch_by_env**（a 边→非 prod 部署，b 边→**approval**→**deploy**）→ **end**。  
因此前端默认图应只包含这 6 类节点，且连线与 handle 约定与本文一致，后端即可在后续迭代中实现「按保存的图执行」，完全根据用户定义的流程执行。

## 存储格式（当前：nodes + edges）

- **节点**：`{ id, type, position, data }`，其中 `data` 可含 `label`（展示用）、`env`（仅 deploy 等需要时）。`type` 必须为上述 6 种之一。
- **边**：`{ id, source, target, sourceHandle?, targetHandle? }`，branch_by_env 的出边必须带 `sourceHandle`（`a` 或 `b`）。

前端拖拉编辑即对此图进行增删改；保存/读取均用该格式。

## 与 Argo 风格 DAG 的对应（可选，供后端按图执行或对接 Argo）

将 nodes+edges 转为「类 Argo」的 DAG 表示，便于执行引擎或 Argo 使用：

- **templates**：按节点类型定义模板（每种 type 一个 template，如 `trigger`、`create_run`、`deploy` 等），模板名可与 `type` 一致。
- **dag.tasks**：每个节点对应一个 task，`name` = 节点 `id`，`template` = 节点 `type`；**dependencies** = 所有「指向该节点的边」的 source 节点 id 列表（即前驱）。  
  例：边 A→B、A→C、B→D、C→D 对应 task D 的 `dependencies: [B, C]`。
- **branch_by_env** 等分支：在 Argo 中可用 [Enhanced Depends](https://argo-workflows.readthedocs.io/en/latest/enhanced-depends-logic/) 或条件 task；我们当前用 `sourceHandle` 区分出边，执行时按上下文环境选一条出边即可。

后端若实现「按图执行」，可：  
1）读取保存的 nodes+edges；  
2）从 `trigger` 起拓扑排序或按依赖遍历；  
3）每步根据节点 type 调用现有能力（create_run、创建审批、doExecuteRun 等）；  
4）或先转成上述 DAG 再交给 Temporal/Argo 执行。
