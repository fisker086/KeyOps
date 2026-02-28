package app

import (
	"fmt"

	"github.com/fisker086/keyops/internal/aiassistant"
	"github.com/fisker086/keyops/internal/aiassistant/tools"
	"github.com/fisker086/keyops/internal/service"
)

// k8sToolRunner 实现 tools.Runner，用于执行 K8s 工具集（见 aiassistant/tools/k8s）。
type k8sToolRunner struct {
	k8sService    *service.K8sService
	k8sClusterSvc *service.K8sClusterService
}

// NewK8sToolRunner 创建 K8s 工具集执行器；若任一 service 为 nil 则返回 nil。
func NewK8sToolRunner(k8sService *service.K8sService, k8sClusterSvc *service.K8sClusterService) tools.Runner {
	if k8sService == nil || k8sClusterSvc == nil {
		return nil
	}
	return &k8sToolRunner{k8sService: k8sService, k8sClusterSvc: k8sClusterSvc}
}

func (r *k8sToolRunner) Run(env interface{}, toolName string, input map[string]interface{}) (interface{}, error) {
	e, ok := env.(*aiassistant.Environment)
	if !ok || e == nil {
		return nil, fmt.Errorf("invalid environment")
	}
	clusterID := e.K8sClusterID
	if clusterID == "" {
		return nil, fmt.Errorf("当前目标环境未关联 K8s 集群，请在「环境管理」中为该环境选择关联的 K8s 集群")
	}
	getNS := func() string {
		if input == nil {
			return ""
		}
		if v, _ := input["namespace"].(string); v != "" {
			return v
		}
		return ""
	}
	getStr := func(key string) string {
		if input == nil {
			return ""
		}
		v, _ := input[key].(string)
		return v
	}

	switch toolName {
	case "k8s_cluster_summary":
		return r.k8sClusterSvc.GetClusterSummary(clusterID)
	case "k8s_list_nodes":
		return r.k8sService.GetNodeList(clusterID, "", 0, 0)
	case "k8s_list_namespaces":
		return r.k8sService.GetNamespaceList(clusterID, "")
	case "k8s_list_pods":
		return r.k8sService.GetPodList(clusterID, "", 0, 0, getNS())
	case "k8s_list_deployments":
		return r.k8sService.GetDeploymentList(clusterID, "", 0, 0, getNS())
	case "k8s_list_daemonsets":
		return r.k8sService.GetDaemonSetList(clusterID, "", 0, 0, getNS())
	case "k8s_list_statefulsets":
		return r.k8sService.GetStatefulSetList(clusterID, "", 0, 0, getNS())
	case "k8s_list_events":
		ns := getNS()
		objName := getStr("object_name")
		objKind := getStr("object_kind")
		return r.k8sService.GetEventList(clusterID, "", 0, 0, ns, objName, objKind)
	default:
		return nil, fmt.Errorf("未知的 K8s 工具: %s", toolName)
	}
}
