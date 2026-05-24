package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fisker086/keyops/internal/mcp"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sToolContext struct {
	ClusterRepo repository.K8sClusterRepository
}

func createClientset(cluster *model.K8sCluster) (kubernetes.Interface, error) {
	switch cluster.AuthType {
	case "kubeconfig":
		config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Kubeconfig))
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %v", err)
		}
		return kubernetes.NewForConfig(config)
	default:
		config := &rest.Config{
			Host:        cluster.APIServer,
			BearerToken: cluster.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
		}
		return kubernetes.NewForConfig(config)
	}
}

func RegisterTools(registry *mcp.Registry, ctx *K8sToolContext) {
	registry.Register(
		mcp.ToolDefinition{
			Name:        "k8s_list_namespaces",
			Description: "List all namespaces in the Kubernetes cluster",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
					"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListNamespaces(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "k8s_list_pods",
			Description: "List pods in a namespace",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cluster_id":     map[string]string{"type": "string", "description": "Cluster ID"},
					"cluster_name":   map[string]string{"type": "string", "description": "Cluster name"},
					"namespace":      map[string]string{"type": "string", "description": "Namespace (default: default)"},
					"label_selector": map[string]string{"type": "string", "description": "Label selector filter"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListPods(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "k8s_list_deployments",
			Description: "List deployments in a namespace",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
					"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
					"namespace":    map[string]string{"type": "string", "description": "Namespace (default: default)"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListDeployments(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "k8s_get_pod_logs",
			Description: "Get logs from a pod",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
					"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
					"namespace":    map[string]string{"type": "string", "description": "Namespace"},
					"pod_name":     map[string]string{"type": "string", "description": "Pod name"},
					"container":    map[string]string{"type": "string", "description": "Container name (optional)"},
					"tail_lines":   map[string]string{"type": "string", "description": "Number of recent log lines (default: 100)"},
				},
				"required": []string{"namespace", "pod_name"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetPodLogs(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "k8s_describe_node",
			Description: "Describe a Kubernetes node",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
					"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
					"node_name":    map[string]string{"type": "string", "description": "Node name"},
				},
				"required": []string{"node_name"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleDescribeNode(args, ctx)
		},
	)
}

type clusterResolver interface {
	FindByID(id string) (*model.K8sCluster, error)
	FindByName(name string) (*model.K8sCluster, error)
}

func resolveCluster(args map[string]any, repo repository.K8sClusterRepository) (*model.K8sCluster, error) {
	clusterID, _ := args["cluster_id"].(string)
	clusterName, _ := args["cluster_name"].(string)

	var cluster *model.K8sCluster
	var err error

	if clusterID != "" {
		cluster, err = repo.FindByID(clusterID)
	} else if clusterName != "" {
		cluster, err = repo.FindByName(clusterName)
	} else {
		return nil, fmt.Errorf("cluster_id or cluster_name is required")
	}

	if err != nil {
		return nil, fmt.Errorf("cluster query error: %v", err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found")
	}
	return cluster, nil
}

func handleListNamespaces(args json.RawMessage, ctx *K8sToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	clientset, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	var items []map[string]any
	for _, ns := range nsList.Items {
		items = append(items, map[string]any{
			"name":   ns.Name,
			"status": ns.Status.Phase,
			"age":    ns.CreationTimestamp.Format("2006-01-02"),
		})
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListPods(args json.RawMessage, ctx *K8sToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	clientset, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	namespace := "default"
	if n, ok := params["namespace"].(string); ok && n != "" {
		namespace = n
	}

	opts := metav1.ListOptions{}
	if ls, ok := params["label_selector"].(string); ok && ls != "" {
		opts.LabelSelector = ls
	}

	podList, err := clientset.CoreV1().Pods(namespace).List(context.Background(), opts)
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	var items []map[string]any
	for _, pod := range podList.Items {
		items = append(items, map[string]any{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"status":    pod.Status.Phase,
			"node":      pod.Spec.NodeName,
			"ip":        pod.Status.PodIP,
			"age":       pod.CreationTimestamp.Format("2006-01-02"),
		})
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListDeployments(args json.RawMessage, ctx *K8sToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	clientset, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	namespace := "default"
	if n, ok := params["namespace"].(string); ok && n != "" {
		namespace = n
	}

	deployList, err := clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	var items []map[string]any
	for _, d := range deployList.Items {
		items = append(items, map[string]any{
			"name":             d.Name,
			"namespace":        d.Namespace,
			"replicas":         d.Status.Replicas,
			"available":        d.Status.AvailableReplicas,
			"strategy":         string(d.Spec.Strategy.Type),
			"age":              d.CreationTimestamp.Format("2006-01-02"),
		})
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetPodLogs(args json.RawMessage, ctx *K8sToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	clientset, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	namespace, _ := params["namespace"].(string)
	podName, _ := params["pod_name"].(string)
	container, _ := params["container"].(string)
	tailLines := int64(100)
	if tl, ok := params["tail_lines"].(string); ok && tl != "" {
		fmt.Sscanf(tl, "%d", &tailLines)
	}

	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}

	logs := clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	data, err := logs.DoRaw(context.Background())
	if err != nil {
		return errorResult("get logs error: " + err.Error())
	}

	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleDescribeNode(args json.RawMessage, ctx *K8sToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	clientset, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	nodeName, _ := params["node_name"].(string)
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return errorResult("get node error: " + err.Error())
	}

	var conditions []string
	for _, c := range node.Status.Conditions {
		conditions = append(conditions, fmt.Sprintf("%s=%s", c.Type, c.Status))
	}

	info := map[string]any{
		"name":       node.Name,
		"status":     conditions,
		"version":    node.Status.NodeInfo.KubeletVersion,
		"os":         node.Status.NodeInfo.OSImage,
		"arch":       node.Status.NodeInfo.Architecture,
		"cpu_cores":  quantityString(node.Status.Capacity["cpu"]),
		"memory":     quantityString(node.Status.Capacity["memory"]),
		"pod_cidr":   node.Spec.PodCIDR,
		"age":        node.CreationTimestamp.Format("2006-01-02"),
		"labels":     node.Labels,
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.ToolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func rawJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func quantityString(q resource.Quantity) string {
	return q.String()
}
