# AI 运维助手 - 工具集目录规范

本目录用于统一管理 AI 助手的**工具集**（ToolSet）：工具名、提示词片段、以及「如何执行」的约定。  
内置工具（Prometheus、Grafana）仍在 `engine.go` 中直接执行；**需外部依赖**的工具（如 K8s、数据库）在此注册，由 app 层实现 Runner 并注入。

## 目录结构

```
internal/aiassistant/tools/
├── README.md           # 本规范
├── registry.go         # 统一注册表：Register / Lookup / GetPromptFragment / Runner 接口
├── builtin/            # 内置工具集（Prometheus + Grafana + 分析），仅定义，执行在 engine 内
│   └── builtin.go     # 工具名、提示词；不 Register，不经过 Runner
├── k8s/                # K8s 工具集（需外部 Runner）
│   └── k8s.go         # 工具名、提示词、init() 中 Register
├── (未来) mysql/
│   └── mysql.go
└── (未来) xxx/
    └── xxx.go
```

- **registry.go**：所有「需外部 Runner」的工具集共用的注册表与 `Runner` 接口，**不要**在此文件里写具体工具名。
- **builtin/**：Prometheus、Grafana、detect_anomaly、check_correlation 等**内置工具**的定义（名称 + 提示词），执行逻辑仍在 **engine.go** 的 `executeTool` 中，**不**注册到 registry、**不**使用 Runner。
- **每个需 Runner 的工具集一个子目录**（如 `k8s/`），包内 `init()` 调用 `tools.Register`；根目录 `tools/` 仅放 `registry.go` 和 `README.md`。

## 新增工具集接入步骤

### 1. 在 tools 下新增子目录与定义

在 `internal/aiassistant/tools/` 下新建目录，例如 `mysql/`：

**internal/aiassistant/tools/mysql/mysql.go：**

```go
package mysql

import "github.com/fisker/zjump-backend/internal/aiassistant/tools"

const ID = "mysql"

var (
	Names = []string{
		"mysql_slow_queries",
		"mysql_top_tables",
	}
	PromptFragment = `
## MySQL 工具箱（仅当目标环境关联了 MySQL 实例时可用）：
...
`
)

func init() {
	tools.Register(&tools.ToolSet{
		ID:             ID,
		Names:          Names,
		PromptFragment: PromptFragment,
	})
}
```

- **ID**：全局唯一，与「环境」里关联的配置对应（如环境扩展字段 `mysql_instance_id`）。
- **Names**：该工具集提供的全部 Action 工具名。
- **PromptFragment**：写入专家 system prompt 的段落，说明每个工具的作用与参数，格式与现有 K8s 一致即可。
- **init()**：必须调用 `tools.Register`，否则引擎不会识别该工具集。

### 2. 在 app 层实现 Runner

在 `internal/app/` 下新增文件（或扩展现有 runner 文件），实现 `tools.Runner`：

```go
// internal/app/ai_assistant_mysql_runner.go
package app

import (
	"github.com/fisker/zjump-backend/internal/aiassistant"
	"github.com/fisker/zjump-backend/internal/aiassistant/tools"
	"github.com/fisker/zjump-backend/internal/aiassistant/tools/mysql"
)

type mysqlToolRunner struct {
	// 注入 DB 或相关 service
}

func (r *mysqlToolRunner) Run(env interface{}, toolName string, input map[string]interface{}) (interface{}, error) {
	e, ok := env.(*aiassistant.Environment)
	if !ok || e == nil {
		return nil, fmt.Errorf("invalid environment")
	}
	// 从 e.ExtraConfig 或 e 的扩展字段读取实例 ID 等
	instanceID := ...
	switch toolName {
	case "mysql_slow_queries":
		return r.querySlowLogs(instanceID, input)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func NewMySQLToolRunner(...) tools.Runner { ... }
```

- **Runner.Run(env, toolName, input)**：`env` 即当前会话的 `*aiassistant.Environment`，实现方自行断言并读取所需配置（如 `K8sClusterID`、`ExtraConfig["mysql_instance_id"]` 等）。
- Runner 实现放在 **app 包**，以便注入 `service`、`repository` 等依赖；**不要**在 `aiassistant/tools/xxx` 下引用业务 service。

### 3. 注册 Runner 并注入 Handler，并在 Engine 中触发工具集 init

**3.1 在 internal/app/handlers.go 中**

- 创建该工具集的 Runner 实例（如 `NewMySQLToolRunner(services.MySQL)`）。
- 将 Runner 放入传给 AI 助手的 `runners map[string]tools.Runner`，key 为工具集 **ID**（如 `mysql.ID`）：

```go
runners := map[string]tools.Runner{
	k8s.ID: NewK8sToolRunner(services.K8s, services.K8sCluster),
}
if services.MySQL != nil {
	runners[mysql.ID] = NewMySQLToolRunner(services.MySQL)
}
aiAssistantHandler = aiassistant.NewHandler(..., runners)
```

**3.2 在 internal/aiassistant/engine.go 中**

- 增加对新区块包的 **空白 import**，以便该包的 `init()` 执行并把工具集注册到 `tools`：

```go
import (
	// ...
	_ "github.com/fisker/zjump-backend/internal/aiassistant/tools/k8s"   // 已有
	_ "github.com/fisker/zjump-backend/internal/aiassistant/tools/mysql"  // 新增工具集时添加
)
```

Engine 会根据 `tools.Lookup(toolName)` 得到工具集 ID，再从 `runners` 中取 Runner 执行；未配置的 Runner 会返回「工具未配置」类错误。

### 4. 配置专家提示词

- 若该工具集**只给特定专家**用：在 `internal/aiassistant/prompts.go` 中为该专家拼 prompt 时，加上 `tools.GetPromptFragment(mysql.ID)`。
- 若为**通用工具**：在 `commonConstraints` 或对应专家的约束里追加上述提示词片段。
- **init.sql** 中若存在该专家的种子数据，需同步更新 `system_prompt` 字段，把新工具集的说明段落加进去。

## 提示词约定

- 每个工具的说明建议包含：**工具名、一句话用途、参数 JSON 示例**（如 `参数：{"namespace": "可选"}`）。
- 注明**使用条件**（如「仅当目标环境关联了 K8s 集群时可用」），与 Runner 内对 `env` 的校验保持一致。
- 编号可与现有工具箱衔接（如 K8s 从 8 开始），避免与内置 1–7 冲突。

## 工具怎么写（执行逻辑）

- **内置工具**（仅依赖环境中的 Prometheus/Grafana URL）：定义放在 **tools/builtin/**（名称 + 提示词），执行仍在 **engine.go** 的 `executeTool` 里用 `e.prom` / `e.graf` 实现，**不**注册、**不**使用 Runner。
- **需集群、DB、外部服务等依赖的工具**：在 `tools/` 下新建子包并 `Register`，在 **app** 层实现 `tools.Runner`，通过 `env` 取环境配置（如集群 ID、实例 ID），在 Runner 内调用对应 service 完成请求并返回可 JSON 序列化的结果。

## 小结

| 内容           | 位置 |
|----------------|------|
| 内置工具名 + 提示词 | `internal/aiassistant/tools/builtin/builtin.go`（不 Register，执行在 engine） |
| 外部工具名 + 提示词 | `internal/aiassistant/tools/<id>/<id>.go`，init 中 `Register` |
| Runner 接口     | `internal/aiassistant/tools/registry.go` |
| Runner 实现    | `internal/app/ai_assistant_<id>_runner.go` |
| 注入 Runner    | `internal/app/handlers.go`，`map[string]tools.Runner` 传给 NewHandler |
| 专家提示词引用  | `prompts.go` 中 `builtin.PromptFragment` 或 `tools.GetPromptFragment(xxx.ID)`，种子数据见 `sql/init.sql` |

按上述规范接入后，新工具集会与 K8s 一样被引擎识别、路由到对应 Runner，并在专家提示词中展示工具箱说明。
