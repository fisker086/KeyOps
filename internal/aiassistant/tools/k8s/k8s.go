// Package k8s 提供 AI 助手「K8s 工具集」的定义：工具名列表与提示词片段。
// 执行器（Runner）由 app 层实现并注入，见 internal/app/ai_assistant_k8s_runner.go。
package k8s

import "github.com/fisker086/keyops/internal/aiassistant/tools"

const ID = "k8s"

var (
	Names = []string{
		"k8s_cluster_summary",
		"k8s_list_nodes",
		"k8s_list_namespaces",
		"k8s_list_pods",
		"k8s_list_deployments",
		"k8s_list_daemonsets",
		"k8s_list_statefulsets",
		"k8s_list_events",
	}
	PromptFragment = `
## K8s 工具箱（仅当目标环境关联了 K8s 集群时可用）：

8. k8s_cluster_summary - 获取当前关联 K8s 集群的摘要（节点数、Pod 数、工作负载数等）。参数：{}
9. k8s_list_nodes - 列出集群节点及状态。参数：{}
10. k8s_list_namespaces - 列出所有命名空间。参数：{}
11. k8s_list_pods - 列出指定命名空间下的 Pod。参数：{"namespace": "可选，默认 default"}
12. k8s_list_deployments - 列出指定命名空间下的 Deployment。参数：{"namespace": "可选"}
13. k8s_list_daemonsets - 列出指定命名空间下的 DaemonSet。参数：{"namespace": "可选"}
14. k8s_list_statefulsets - 列出指定命名空间下的 StatefulSet。参数：{"namespace": "可选"}
15. k8s_list_events - 列出命名空间下的事件（可按对象过滤）。参数：{"namespace": "可选", "object_name": "可选", "object_kind": "可选"}
`
)

func init() {
	tools.Register(&tools.ToolSet{
		ID:             ID,
		Names:          Names,
		PromptFragment: PromptFragment,
	})
}
