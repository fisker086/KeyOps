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

### 5. 与专家提示词的关系

- **专家提示词**（systemPromptOverride）：来自 DB 专家配置，定义角色、分析逻辑、约束
- **Skills 提示词**：由 `GetAvailableToolsPrompt` 生成，定义当前可用工具列表

最终 systemPrompt = 专家提示词 + Skills 提示词。

---

## 二、技能列表

| 目录 | name | 环境配置 | 工具数 |
|------|------|----------|--------|
| prometheus/ | prometheus | prom_url | 5 |
| grafana/ | grafana | graf_url, graf_token | 2 |
| k8s/ | k8s | k8s_cluster_id | 8 |

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
