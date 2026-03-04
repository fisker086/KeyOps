# AI 运维助手 Skills

采用 [SKILL.md](https://agentskills.io/) 标准格式，每个技能独立目录，含 `name`、`description`。Skills 不存库，由目标环境配置推导，根据提示词/任务动态注入可用工具。

---

## 一、Skills 是怎么用的

### 1. 整体流程

```
用户创建任务（选环境、专家、模型）
        │
        ▼
getEnvironment(env_id) 从 DB 读取环境（含 prom_url, graf_url, k8s_cluster_id）
        │
        ▼
NewEngine(env, runners, llmConfig) 创建引擎
        │
        ├── env.PromURL != ""  → 创建 PrometheusClient
        ├── env.GrafURL != ""  → 创建 GrafanaClient
        └── env.K8sClusterID   → 使用 runners["k8s"]
        │
        ▼
Engine.Run(task, role, systemPrompt) 执行
        │
        ├── GetAvailableToolsPrompt(env, runners) 根据环境拼接「当前可用技能」的提示词
        ├── systemPrompt = systemPrompt + 可用技能提示词
        └── ReAct 循环：Thought → Action → Observation → ...
        │
        ▼
executeTool(name, input) 执行工具
        │
        ├── 内置工具（Prometheus/Grafana）→ e.prom / e.graf
        └── 外部工具（K8s）→ e.runners["k8s"].Run(env, name, input)
```

### 2. 关键代码位置

| 步骤 | 文件 | 函数/逻辑 |
|------|------|-----------|
| 环境配置 | 目标环境管理 | 配置 prom_url、graf_url、k8s_cluster_id |
| 技能推导 | `skills.go` | `GetEnabledSkills(env, runners)` → 供 UI 展示 |
| 提示词注入 | `skills.go` | `GetAvailableToolsPrompt(env, runners)` → 拼接工具说明 |
| 执行入口 | `engine.go` | `Run()` 中调用 `GetAvailableToolsPrompt` 注入 systemPrompt |
| 工具执行 | `engine.go` | `executeTool()` 根据工具名分发到 prom/graf/runners |

### 3. 技能选择逻辑

**Skills 不存库**，由环境字段推导：

| 环境字段 | 启用技能 | 对应 SKILL.md |
|----------|----------|---------------|
| `prom_url` 非空 | prometheus | `skills/prometheus/SKILL.md` |
| `graf_url` 非空 | grafana | `skills/grafana/SKILL.md` |
| `k8s_cluster_id` 非空 且 `runners["k8s"]` 存在 | k8s | `skills/k8s/SKILL.md` |

### 4. 提示词注入

`GetAvailableToolsPrompt` 会：

1. 根据 `env` 判断哪些技能已启用
2. 从 `builtin.PromptFragmentPrometheus`、`builtin.PromptFragmentGrafana`、`tools.GetPromptFragment("k8s")` 中取对应片段
3. 拼接成一段 Markdown，追加到 systemPrompt 末尾

例如环境配置了 Prometheus + K8s，则只注入这两个技能的提示词，不注入 Grafana，避免 LLM 调用未配置的工具。

### 5. 与专家提示词的关系（优先级：数据库覆盖）

- **Skills/工具提示词**：由 `GetAvailableToolsPrompt` 生成，或 k8s-install 的 SKILL 知识库
- **专家提示词**（数据库 system_prompt）：来自 DB 专家配置，定义角色、分析逻辑、约束

**拼接顺序**：Skills/工具在前，数据库在后 → **数据库定制（如安装路径、偏好）优先于技能默认值**，改数据库即可生效，无需改代码。

---

## 二、技能列表

| 目录 | name | 环境配置 | 工具数 |
|------|------|----------|--------|
| prometheus/ | prometheus | prom_url | 5 |
| grafana/ | grafana | graf_url, graf_token | 2 |
| k8s/ | k8s | k8s_cluster_id | 8 |
| k8s-install/ | k8s-install | 专家 `skill_id=k8s-install`（可结合环境预配置节点） | 0 |

**k8s-install**：K8s 集群安装指导技能，由数据库专家 `skill_id` 驱动注入（默认 `k8s-installer`）。不依赖环境工具，知识库通过 embed 注入，包含 containerd、CNI、kubeadm/二进制等官方下载地址与安装流程，分步骤输出命令，每步等待用户确认；若环境提供 `k8s_install_nodes`，可直接带入节点上下文。

---

## 三、SKILL.md 格式说明

每个 SKILL.md 的 YAML frontmatter：

| 字段 | 必填 | 说明 |
|------|------|------|
| name | 是 | 技能唯一标识，与目录名一致 |
| description | 是 | 技能用途与激活条件，用于发现与路由 |
| allowed-tools | 否 | 该技能可用的工具列表，空格分隔 |

正文为 Markdown，包含工具详解、使用建议、典型流程等。当前实现中，**提示词内容来自 `tools/builtin` 和 `tools/k8s` 的 Go 常量**，SKILL.md 主要作为文档与规范参考；若需从 SKILL.md 动态加载，需在 `skills.go` 中增加解析逻辑。

---

## 四、新增技能步骤

1. 在 `skills/` 下新建目录，如 `skills/mysql/SKILL.md`
2. 在 `tools/` 下定义工具集，`tools.Register` 注册
3. 在 `app` 层实现 `tools.Runner`，注入到 `Engine.runners`
4. 在 `skills.go` 的 `GetEnabledSkills`、`GetAvailableToolsPrompt` 中增加判断
5. 在 `Environment` 和 `ai_assistant_environments` 表增加配置字段

---

## 五、技能质量门禁（新增/变更必查）

为保证不同技能输出风格一致、失败时可降级、避免“看起来会用但一跑就崩”，每个 SKILL.md 新增或修改后都应通过以下清单。

### 1. 结构门禁（必须有）

- **参数兜底与默认值**：缺参时先补问还是用默认值，必须明确
- **失败重试与降级策略**：至少定义 1 次重试和 1 条降级路径
- **标准输出结构**：建议统一为「结论 / 证据 / 下一步」

### 2. 行为门禁（必须满足）

- 不编造工具结果；无数据要明确写“空结果/未检索到”
- 工具参数来源清晰（用户输入、默认值、上一步工具输出）
- 关键假设可追溯（例如时间窗口、命名空间、目标对象）
- 与其他技能联动路径明确（如 Grafana -> Prometheus，K8s + Prometheus）

### 3. 一致性门禁（发布前核对）

- SKILL 文档中的激活条件与代码逻辑一致（`skills.go` / `tools` 注册）
- 工具名、参数名、默认值与实际实现一致（避免文档漂移）
- 示例 JSON 可直接复用（键名、类型、大小写正确）

### 4. 可直接复用模板

建议每个技能至少包含以下三个小节：

```md
## 参数兜底与默认值（强约束）
- ...

## 失败重试与降级策略
1. ...

## 标准输出结构（建议统一）
1. 结论
2. 证据
3. 下一步
```

---

## 六、Skills 轻量评审流程（建议）

目标：在不增加太多流程成本的前提下，确保技能“可执行、可回溯、不漂移”。

### 0. 极速模式（巡检默认，2-3 分钟）

巡检类技能优先走极速模式，默认不走完整 5 步流程。

- 只检查 3 件事：
  1. 是否写清“参数兜底与默认值”
  2. 是否写清“失败重试与降级策略”
  3. 是否写清“标准输出结构（结论/证据/下一步）”
- 再快速扫一眼工具名是否与实现一致（避免拼写错误）
- 满足以上条件直接通过，不做长流程评审

> 建议口径：**“巡检类 skill 评审只查三板斧：兜底、降级、结构；3 分钟内结束。”**

### 0.1 按角色评审矩阵（推荐）

同一套评审流程按角色分层执行，避免“一刀切”：

| 角色 | 默认模式 | 额外必查项 | 目标 |
|------|----------|-----------|------|
| `inspector` | 极速模式（2-3 分钟） | 无 | 巡检及时、输出稳定 |
| `sre` | 轻量模式 | 证据链完整（结论可被指标/事件支撑） | 减少误判，便于追溯 |
| `k8s-expert` | 轻量模式 | 命名空间/对象粒度明确（避免泛化结论） | 排障定位更准确 |
| `k8s-installer` | 强约束模式（优先） | 版本/网络/拓扑先确认，分步骤输出并等待确认 | 安装命令正确、可落地 |

执行规则：

- 角色为 `inspector`：默认直接走极速模式
- 角色为 `sre` / `k8s-expert`：在极速模式基础上补“额外必查项”
- 角色为 `k8s-installer`：不走极速，按强约束流程评审（可慢不能错）

### 1. 何时触发

- 新增任意 `SKILL.md`
- 修改技能的工具名、参数、默认值、激活条件
- 修改涉及跨技能协同（如 Grafana -> Prometheus、K8s + Prometheus）的段落

### 2. 评审角色（最小配置）

- **作者**：提交技能变更并完成自检
- **评审者（1 人即可）**：优先由熟悉对应工具链的人担任

### 3. 5 步快速流程

1. **作者自检（3-5 分钟）**  
   按“第五章 技能质量门禁”逐项打勾，确保结构和行为约束都满足。

2. **一致性核对（3 分钟）**  
   对照 `skills.go`、`tools/*` 的真实工具与参数，确认文档无漂移。

3. **场景走查（5 分钟）**  
   至少用 2 个正例 + 1 个失败例进行桌面推演（无需真实执行）：
   - 正例：参数齐全，预期应直接可执行
   - 正例：参数不全，预期应触发兜底逻辑
   - 失败例：工具失败/空结果，预期应走降级路径

4. **评审结论（2 分钟）**  
   评审者给出“通过 / 需修改”，并指出必须修项（若有）。

5. **合并前确认（1 分钟）**  
   作者确认所有必须修项已处理，更新文档后再合并。

### 4. 通过标准（轻量）

- 必须项：
  - 已包含“参数兜底 / 失败降级 / 标准输出结构”三节
  - 激活条件、工具名、参数名与代码一致
  - 明确“无数据/失败时不编造结果”
- 建议项：
  - 有跨技能联动路径说明
  - 有可直接复制的参数示例 JSON

### 5. 评审记录模板（可复制）

```md
## Skills 轻量评审记录

- 变更技能：`<skill-name>`
- 评审者：`<name>`
- 结论：`通过 / 需修改`

### 检查项
- [ ] 参数兜底与默认值
- [ ] 失败重试与降级策略
- [ ] 标准输出结构（结论/证据/下一步）
- [ ] 激活条件与代码一致
- [ ] 工具名/参数名与实现一致
- [ ] 失败或空结果时不编造

### 必须修项（如有）
1. ...

### 备注
- ...
```
