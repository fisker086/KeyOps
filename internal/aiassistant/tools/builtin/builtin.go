// Package builtin 提供 AI 助手「内置工具集」的定义：Prometheus + Grafana + 分析工具。
// 执行逻辑仍在 engine.go 的 executeTool 中（使用 e.prom / e.graf），不通过 Runner，故不调用 tools.Register。
package builtin

// ID 内置工具集标识（未注册到 registry，仅用于文档与提示词聚合）
const ID = "builtin"

// Names 内置工具名列表（执行由 engine 直接处理）
var Names = []string{
	"list_all_dashboards",
	"get_dashboard_metadata",
	"find_metrics_by_keyword",
	"get_metric_dimension",
	"execute_promql_query",
	"detect_anomaly",
	"check_correlation",
}

// PromptFragmentPrometheus Prometheus 技能：指标查询与分析
const PromptFragmentPrometheus = `
## Prometheus 技能（指标查询与分析）：
1. find_metrics_by_keyword - 在 Prometheus 中搜索包含关键字的指标名。参数：{"keyword": "string"}
2. get_metric_dimension - 获取指定指标名的标签维度。参数：{"metric_name": "string"}
3. execute_promql_query - 执行 PromQL 范围查询。参数：query(string), duration(string, 默认"1h"), step(string), start_time/end_time(可选)
4. detect_anomaly - 对 Prometheus 查询结果做异常检测(3-sigma)。参数：{"result_list": array}
5. check_correlation - 计算两个指标序列的相关性。参数：{"result_a": array, "result_b": array}
`

// PromptFragmentGrafana Grafana 技能：仪表盘
const PromptFragmentGrafana = `
## Grafana 技能（仪表盘）：
1. list_all_dashboards - 获取 Grafana 中所有仪表盘列表。参数：{}
2. get_dashboard_metadata - 获取指定 UID 仪表盘中的 PromQL 查询。参数：{"uid": "string"}
`

// PromptFragment 内置工具箱的提示词片段，供 commonConstraints 等使用（全量，兼容旧逻辑）
const PromptFragment = PromptFragmentPrometheus + PromptFragmentGrafana
