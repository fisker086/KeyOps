package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/mcp"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type K8sExtendedToolContext struct {
	ClusterRepo *repository.K8sClusterRepository
}

func RegisterK8sExtendedTools(registry *mcp.Registry, ctx *K8sExtendedToolContext) {
	// Standard resource listers
	registerLister(registry, ctx, "k8s_list_nodes", "List all nodes in the cluster", nodeLister)
	registerLister(registry, ctx, "k8s_list_services", "List services in a namespace", serviceLister)
	registerLister(registry, ctx, "k8s_list_daemonsets", "List daemonsets in a namespace", daemonsetLister)
	registerLister(registry, ctx, "k8s_list_statefulsets", "List statefulsets in a namespace", statefulsetLister)
	registerLister(registry, ctx, "k8s_list_jobs", "List jobs in a namespace", jobLister)
	registerLister(registry, ctx, "k8s_list_cronjobs", "List cronjobs in a namespace", cronjobLister)
	registerLister(registry, ctx, "k8s_list_ingresses", "List ingresses in a namespace", ingressLister)
	registerLister(registry, ctx, "k8s_list_events", "List events in a namespace", eventLister)
	registerLister(registry, ctx, "k8s_list_service_accounts", "List service accounts in a namespace", serviceAccountLister)

	// Generic operations
	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_apply_yaml",
		Description: "Create or update resources from YAML manifest",
		InputSchema: yamlInputSchema,
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleApplyYAML(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_get_resource",
		Description: "Get a resource by kind, namespace and name",
		InputSchema: resourceInputSchema,
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleGetResource(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_describe_resource",
		Description: "Describe a resource (verbose details)",
		InputSchema: resourceInputSchema,
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleDescribeResource(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_delete_resource",
		Description: "Delete a resource by kind, namespace and name",
		InputSchema: resourceInputSchema,
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleDeleteResource(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_patch_resource",
		Description: "Patch a resource with strategic merge or JSON merge patch",
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
				"kind":         map[string]string{"type": "string", "description": "Resource kind"},
				"api_version":  map[string]string{"type": "string", "description": "API version"},
				"namespace":    map[string]string{"type": "string", "description": "Namespace"},
				"name":         map[string]string{"type": "string", "description": "Resource name"},
				"patch":        map[string]string{"type": "string", "description": "Patch content (JSON)"},
				"patch_type":   map[string]string{"type": "string", "description": "Patch type: strategic, merge, or json (default: strategic)"},
			},
			"required": []string{"kind", "api_version", "name", "patch"},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult { return handlePatchResource(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_inspect_pod",
		Description: "Get comprehensive pod information (describe + logs + events)",
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
				"namespace":    map[string]string{"type": "string", "description": "Namespace"},
				"pod_name":     map[string]string{"type": "string", "description": "Pod name"},
				"tail_lines":   map[string]string{"type": "string", "description": "Number of recent log lines (default: 50)"},
			},
			"required": []string{"pod_name"},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleInspectPod(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_rollout_history",
		Description: "Show deployment rollout history",
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":      map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name":    map[string]string{"type": "string", "description": "Cluster name"},
				"namespace":       map[string]string{"type": "string", "description": "Namespace"},
				"deployment_name": map[string]string{"type": "string", "description": "Deployment name"},
			},
			"required": []string{"deployment_name"},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleRolloutHistory(args, ctx) })

	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_rollout_undo",
		Description: "Rollback a deployment to a previous revision",
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":      map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name":    map[string]string{"type": "string", "description": "Cluster name"},
				"namespace":       map[string]string{"type": "string", "description": "Namespace"},
				"deployment_name": map[string]string{"type": "string", "description": "Deployment name"},
				"revision":        map[string]string{"type": "string", "description": "Target revision number (empty = previous)"},
			},
			"required": []string{"deployment_name"},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleRolloutUndo(args, ctx) })

	// ServiceAccount operations
	registry.Register(mcp.ToolDefinition{
		Name:        "k8s_create_service_account",
		Description: "Create a service account in a namespace",
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
				"namespace":    map[string]string{"type": "string", "description": "Namespace (default: default)"},
				"name":         map[string]string{"type": "string", "description": "ServiceAccount name"},
			},
			"required": []string{"name"},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult { return handleCreateServiceAccount(args, ctx) })

	// Istio resources (via dynamic client)
	registerIstioListTool(registry, ctx, "k8s_list_istio_virtual_services", "List Istio VirtualServices in a namespace",
		"virtualservices", "networking.istio.io", "v1beta1")
	registerIstioListTool(registry, ctx, "k8s_list_istio_destination_rules", "List Istio DestinationRules in a namespace",
		"destinationrules", "networking.istio.io", "v1beta1")
	registerIstioListTool(registry, ctx, "k8s_list_istio_gateways", "List Istio Gateways in a namespace",
		"gateways", "networking.istio.io", "v1beta1")
}

// --- helpers ---

type listerFunc func(clientset kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error)

func registerLister(reg *mcp.Registry, ctx *K8sExtendedToolContext, name, desc string, fn listerFunc) {
	hasNS := true
	switch name {
	case "k8s_list_nodes", "k8s_list_service_accounts":
		hasNS = false
	}

	props := map[string]any{
		"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
		"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
	}
	if hasNS {
		props["namespace"] = map[string]string{"type": "string", "description": "Namespace (default: default)"}
	}

	reg.Register(mcp.ToolDefinition{
		Name:        name,
		Description: desc,
		InputSchema: rawJSON(map[string]any{
			"type":       "object",
			"properties": props,
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult {
		return handleListStandard(args, ctx, fn)
	})
}

func registerIstioListTool(reg *mcp.Registry, ctx *K8sExtendedToolContext, name, desc, resource, group, version string) {
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	reg.Register(mcp.ToolDefinition{
		Name:        name,
		Description: desc,
		InputSchema: rawJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
				"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
				"namespace":    map[string]string{"type": "string", "description": "Namespace (default: default)"},
			},
		}),
	}, func(args json.RawMessage) *mcp.CallToolResult {
		return handleListDynamic(args, ctx, gvr)
	})
}

// --- client creation ---

func createDynamicClient(cluster *model.K8sCluster) (dynamic.Interface, error) {
	config, err := restConfig(cluster)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

func restConfig(cluster *model.K8sCluster) (*rest.Config, error) {
	if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		return clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Kubeconfig))
	}
	return &rest.Config{
		Host:        cluster.APIServer,
		BearerToken: cluster.Token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, nil
}

// --- standard list handlers ---

func handleListStandard(args json.RawMessage, ctx *K8sExtendedToolContext, fn listerFunc) *mcp.CallToolResult {
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

	namespace := ""
	if n, ok := params["namespace"].(string); ok {
		namespace = n
	}

	items, err := fn(clientset, namespace, params)
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func nodeLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, n := range nodes.Items {
		items = append(items, map[string]any{
			"name":             n.Name,
			"status":           nodeReadyStatus(n.Status.Conditions),
			"version":          n.Status.NodeInfo.KubeletVersion,
			"os_image":         n.Status.NodeInfo.OSImage,
			"architecture":     n.Status.NodeInfo.Architecture,
			"cpu_capacity":     n.Status.Capacity.Cpu().String(),
			"memory_capacity":  n.Status.Capacity.Memory().String(),
			"pod_capacity":     n.Status.Capacity.Pods().Value(),
			"age":              n.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func serviceLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	svcs, err := cs.CoreV1().Services(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, s := range svcs.Items {
		items = append(items, map[string]any{
			"name":        s.Name,
			"type":        string(s.Spec.Type),
			"cluster_ip":  s.Spec.ClusterIP,
			"ports":       servicePorts(s.Spec.Ports),
			"selector":    s.Spec.Selector,
			"age":         s.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func daemonsetLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.AppsV1().DaemonSets(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, d := range list.Items {
		items = append(items, map[string]any{
			"name":            d.Name,
			"desired":         d.Status.DesiredNumberScheduled,
			"current":         d.Status.CurrentNumberScheduled,
			"ready":           d.Status.NumberReady,
			"up_to_date":      d.Status.UpdatedNumberScheduled,
			"age":             d.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func statefulsetLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, s := range list.Items {
		items = append(items, map[string]any{
			"name":      s.Name,
			"replicas":  s.Status.Replicas,
			"ready":     s.Status.ReadyReplicas,
			"service":   s.Spec.ServiceName,
			"age":       s.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func jobLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.BatchV1().Jobs(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, j := range list.Items {
		items = append(items, map[string]any{
			"name":      j.Name,
			"completions": jobCompletions(j),
			"status":   jobStatus(j),
			"age":      j.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func cronjobLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.BatchV1().CronJobs(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, c := range list.Items {
		items = append(items, map[string]any{
			"name":     c.Name,
			"schedule": c.Spec.Schedule,
			"suspend":  c.Spec.Suspend != nil && *c.Spec.Suspend,
			"active":   c.Status.Active,
			"last_schedule": fmtMetav1Time(c.Status.LastScheduleTime),
			"age":      c.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func ingressLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.NetworkingV1().Ingresses(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, ing := range list.Items {
		var hosts []string
		for _, r := range ing.Spec.Rules {
			hosts = append(hosts, r.Host)
		}
		items = append(items, map[string]any{
			"name":   ing.Name,
			"hosts":  hosts,
			"tls":    len(ing.Spec.TLS) > 0,
			"class":  ingressClass(ing),
			"age":    ing.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

func eventLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	opts := metav1.ListOptions{}
	if objName, ok := args["object_name"].(string); ok && objName != "" {
		objKind, _ := args["object_kind"].(string)
		fieldSel := "involvedObject.name=" + objName
		if objKind != "" {
			fieldSel += ",involvedObject.kind=" + objKind
		}
		opts.FieldSelector = fieldSel
	}
	list, err := cs.CoreV1().Events(ns).List(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, e := range list.Items {
		items = append(items, map[string]any{
			"type":     e.Type,
			"reason":   e.Reason,
			"message":  e.Message,
			"count":    e.Count,
			"object":   fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			"first_seen": fmtTimestamp(e.FirstTimestamp.Time),
			"last_seen":  fmtTimestamp(e.LastTimestamp.Time),
		})
	}
	return items, nil
}

func serviceAccountLister(cs kubernetes.Interface, ns string, args map[string]any) ([]map[string]any, error) {
	ns = getNamespace(ns)
	list, err := cs.CoreV1().ServiceAccounts(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	for _, sa := range list.Items {
		items = append(items, map[string]any{
			"name":      sa.Name,
			"secrets":   len(sa.Secrets),
			"automount": sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken,
			"age":       sa.CreationTimestamp.Format("2006-01-02"),
		})
	}
	return items, nil
}

// --- advanced operation handlers ---

var yamlInputSchema = rawJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
		"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
		"yaml":         map[string]string{"type": "string", "description": "YAML manifest content"},
		"namespace":    map[string]string{"type": "string", "description": "Namespace override (optional)"},
	},
	"required": []string{"yaml"},
})

var resourceInputSchema = rawJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"cluster_id":   map[string]string{"type": "string", "description": "Cluster ID"},
		"cluster_name": map[string]string{"type": "string", "description": "Cluster name"},
		"kind":         map[string]string{"type": "string", "description": "Resource kind (e.g. Pod, Service, Deployment, VirtualService)"},
		"api_version":  map[string]string{"type": "string", "description": "API version (e.g. v1, apps/v1, networking.istio.io/v1beta1)"},
		"namespace":    map[string]string{"type": "string", "description": "Namespace"},
		"name":         map[string]string{"type": "string", "description": "Resource name"},
	},
	"required": []string{"kind", "api_version", "name"},
})

func handleApplyYAML(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`
		YAML        string `json:"yaml"`
		Namespace   string `json:"namespace"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.YAML == "" {
		return errorResult("yaml is required")
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	// Parse multi-document YAML
	docs := splitYAML(params.YAML)
	if len(docs) == 0 {
		return errorResult("no valid YAML documents found")
	}

	var results []string
	for i, doc := range docs {
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
			results = append(results, fmt.Sprintf("doc %d: parse error: %v", i+1, err))
			continue
		}

		if obj.GetKind() == "" {
			results = append(results, fmt.Sprintf("doc %d: missing kind", i+1))
			continue
		}

		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: resourceName(gvk.Kind),
		}
		ns := obj.GetNamespace()
		if ns == "" && params.Namespace != "" {
			ns = params.Namespace
			obj.SetNamespace(ns)
		}

		var nsScope bool
		switch gvk.Kind {
		case "Node", "Namespace", "PersistentVolume", "StorageClass", "ClusterRole", "ClusterRoleBinding":
			nsScope = false
		default:
			nsScope = true
			if ns == "" {
				ns = "default"
				obj.SetNamespace(ns)
			}
		}

		// Try to get the resource first
		var existing *unstructured.Unstructured
		if nsScope {
			existing, err = dyn.Resource(gvr).Namespace(ns).Get(context.Background(), obj.GetName(), metav1.GetOptions{})
		} else {
			existing, err = dyn.Resource(gvr).Get(context.Background(), obj.GetName(), metav1.GetOptions{})
		}

		if err == nil && existing != nil {
			obj.SetResourceVersion(existing.GetResourceVersion())
			var updated *unstructured.Unstructured
			if nsScope {
				updated, err = dyn.Resource(gvr).Namespace(ns).Update(context.Background(), obj, metav1.UpdateOptions{})
			} else {
				updated, err = dyn.Resource(gvr).Update(context.Background(), obj, metav1.UpdateOptions{})
			}
			if err != nil {
				results = append(results, fmt.Sprintf("%s/%s: update error: %v", gvk.Kind, obj.GetName(), err))
			} else {
				results = append(results, fmt.Sprintf("%s/%s: updated (resourceVersion: %s)", gvk.Kind, obj.GetName(), updated.GetResourceVersion()))
			}
		} else {
			if nsScope {
				_, err = dyn.Resource(gvr).Namespace(ns).Create(context.Background(), obj, metav1.CreateOptions{})
			} else {
				_, err = dyn.Resource(gvr).Create(context.Background(), obj, metav1.CreateOptions{})
			}
			if err != nil {
				results = append(results, fmt.Sprintf("%s/%s: create error: %v", gvk.Kind, obj.GetName(), err))
			} else {
				results = append(results, fmt.Sprintf("%s/%s: created", gvk.Kind, obj.GetName()))
			}
		}
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleCreateServiceAccount(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.Name == "" {
		return errorResult("name is required")
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	cs, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	ns := params.Namespace
	if ns == "" {
		ns = "default"
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: ns,
		},
	}

	created, err := cs.CoreV1().ServiceAccounts(ns).Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil {
		return errorResult("create error: " + err.Error())
	}

	data, _ := json.MarshalIndent(map[string]any{
		"name":      created.Name,
		"namespace": created.Namespace,
		"uid":       string(created.UID),
		"secrets":   len(created.Secrets),
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetResource(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	cluster, gvr, name, ns, err := parseResourceArgs(args, ctx)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	var obj *unstructured.Unstructured
	if ns != "" {
		obj, err = dyn.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	} else {
		obj, err = dyn.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
	}
	if err != nil {
		return errorResult("get error: " + err.Error())
	}

	cleanManagedFields(obj)
	data, _ := json.MarshalIndent(obj.Object, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleDescribeResource(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	cluster, gvr, name, ns, err := parseResourceArgs(args, ctx)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	var obj *unstructured.Unstructured
	if ns != "" {
		obj, err = dyn.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	} else {
		obj, err = dyn.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
	}
	if err != nil {
		return errorResult("get error: " + err.Error())
	}

	cleanManagedFields(obj)
	info := extractDescribeInfo(obj)
	data, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleDeleteResource(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	cluster, gvr, name, ns, err := parseResourceArgs(args, ctx)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	if ns != "" {
		err = dyn.Resource(gvr).Namespace(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
	} else {
		err = dyn.Resource(gvr).Delete(context.Background(), name, metav1.DeleteOptions{})
	}
	if err != nil {
		return errorResult("delete error: " + err.Error())
	}

	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: fmt.Sprintf("Deleted %s/%s", gvr.Resource, name)}}}
}

func handlePatchResource(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`
		Kind        string `json:"kind"`
		APIVersion  string `json:"api_version"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
		Patch       string `json:"patch"`
		PatchType   string `json:"patch_type"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.Name == "" || params.Patch == "" {
		return errorResult("name and patch are required")
	}

	patchType := types.StrategicMergePatchType
	if params.PatchType == "merge" {
		patchType = types.MergePatchType
	} else if params.PatchType == "json" {
		patchType = types.JSONPatchType
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	gvr, ns, err := gvrFromKind(params.Kind, params.APIVersion, params.Namespace)
	if err != nil {
		return errorResult(err.Error())
	}

	var updated *unstructured.Unstructured
	if ns != "" {
		updated, err = dyn.Resource(gvr).Namespace(ns).Patch(context.Background(), params.Name, patchType, []byte(params.Patch), metav1.PatchOptions{})
	} else {
		updated, err = dyn.Resource(gvr).Patch(context.Background(), params.Name, patchType, []byte(params.Patch), metav1.PatchOptions{})
	}
	if err != nil {
		return errorResult("patch error: " + err.Error())
	}

	cleanManagedFields(updated)
	data, _ := json.MarshalIndent(updated.Object, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleInspectPod(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`
		Namespace   string `json:"namespace"`
		PodName     string `json:"pod_name"`
		TailLines   int    `json:"tail_lines"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PodName == "" {
		return errorResult("pod_name is required")
	}
	if params.TailLines <= 0 {
		params.TailLines = 50
	}
	ns := params.Namespace
	if ns == "" {
		ns = "default"
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	cs, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	result := make(map[string]any)

	// Pod summary
	pod, err := cs.CoreV1().Pods(ns).Get(context.Background(), params.PodName, metav1.GetOptions{})
	if err != nil {
		return errorResult("get pod error: " + err.Error())
	}
	result["pod"] = map[string]any{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"node":      pod.Spec.NodeName,
		"status":    pod.Status.Phase,
		"ip":        pod.Status.PodIP,
		"host_ip":   pod.Status.HostIP,
		"qos_class": pod.Status.QOSClass,
		"containers": containerStatuses(pod.Status.ContainerStatuses),
		"conditions": podConditions(pod.Status.Conditions),
	}

	// Recent logs
	tail := int64(params.TailLines)
	logOpts := &corev1.PodLogOptions{TailLines: &tail}
	logData, err := cs.CoreV1().Pods(ns).GetLogs(params.PodName, logOpts).DoRaw(context.Background())
	if err == nil {
		result["recent_logs"] = string(logData)
	} else {
		result["recent_logs"] = fmt.Sprintf("log error: %v", err)
	}

	// Events for this pod
	events, err := cs.CoreV1().Events(ns).List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", params.PodName),
	})
	if err == nil {
		var evts []map[string]any
		for _, e := range events.Items {
			evts = append(evts, map[string]any{
				"type":    e.Type,
				"reason":  e.Reason,
				"message": e.Message,
				"count":   e.Count,
				"last":    fmtTimestamp(e.LastTimestamp.Time),
			})
		}
		result["events"] = evts
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleRolloutHistory(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID      string `json:"cluster_id"`
		ClusterName    string `json:"cluster_name"`
		Namespace      string `json:"namespace"`
		DeploymentName string `json:"deployment_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.DeploymentName == "" {
		return errorResult("deployment_name is required")
	}
	ns := params.Namespace
	if ns == "" {
		ns = "default"
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	cs, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	// Get ReplicaSets for this deployment to reconstruct rollout history
	deploy, err := cs.AppsV1().Deployments(ns).Get(context.Background(), params.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return errorResult("get deployment error: " + err.Error())
	}

	sel, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		return errorResult("label selector error: " + err.Error())
	}

	rss, err := cs.AppsV1().ReplicaSets(ns).List(context.Background(), metav1.ListOptions{
		LabelSelector: sel.String(),
	})
	if err != nil {
		return errorResult("list replicasets error: " + err.Error())
	}

	var revisions []map[string]any
	for _, rs := range rss.Items {
		rev := rs.Annotations["deployment.kubernetes.io/revision"]
		revisions = append(revisions, map[string]any{
			"revision":    rev,
			"replicaset":  rs.Name,
			"desired":     rs.Status.Replicas,
			"available":   rs.Status.AvailableReplicas,
			"image":       rs.Spec.Template.Spec.Containers[0].Image,
			"created_at":  rs.CreationTimestamp.Format("2006-01-02 15:04"),
		})
	}

	if revisions == nil {
		revisions = []map[string]any{}
	}

	data, _ := json.MarshalIndent(map[string]any{
		"deployment":    params.DeploymentName,
		"namespace":     ns,
		"current_revision": deploy.Annotations["deployment.kubernetes.io/revision"],
		"revisions":     revisions,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleRolloutUndo(args json.RawMessage, ctx *K8sExtendedToolContext) *mcp.CallToolResult {
	var params struct {
		ClusterID      string `json:"cluster_id"`
		ClusterName    string `json:"cluster_name"`
		Namespace      string `json:"namespace"`
		DeploymentName string `json:"deployment_name"`
		Revision       string `json:"revision"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.DeploymentName == "" {
		return errorResult("deployment_name is required")
	}
	ns := params.Namespace
	if ns == "" {
		ns = "default"
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	cs, err := createClientset(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	deploy, err := cs.AppsV1().Deployments(ns).Get(context.Background(), params.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return errorResult("get deployment error: " + err.Error())
	}

	// Find the target ReplicaSet by revision annotation
	sel, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		return errorResult("label selector error: " + err.Error())
	}

	rss, err := cs.AppsV1().ReplicaSets(ns).List(context.Background(), metav1.ListOptions{
		LabelSelector: sel.String(),
	})
	if err != nil {
		return errorResult("list replicasets error: " + err.Error())
	}

	var targetRS *appsv1.ReplicaSet
	for _, rs := range rss.Items {
		if params.Revision == "" || rs.Annotations["deployment.kubernetes.io/revision"] == params.Revision {
			targetRS = rs.DeepCopy()
			break
		}
	}
	if targetRS == nil {
		return errorResult("no matching revision found")
	}

	// Rollback by copying the template from the target ReplicaSet
	deploy.Spec.Template = targetRS.Spec.Template
	deploy.Spec.Template.ObjectMeta.Labels = targetRS.Spec.Template.Labels

	updated, err := cs.AppsV1().Deployments(ns).Update(context.Background(), deploy, metav1.UpdateOptions{})
	if err != nil {
		return errorResult("rollback error: " + err.Error())
	}

	data, _ := json.MarshalIndent(map[string]any{
		"deployment": params.DeploymentName,
		"namespace":  ns,
		"revision":   params.Revision,
		"status":     "rollback initiated",
		"updated_at": updated.CreationTimestamp.Format("2006-01-02 15:04"),
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

// --- dynamic client list for Istio ---

func handleListDynamic(args json.RawMessage, ctx *K8sExtendedToolContext, gvr schema.GroupVersionResource) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	cluster, err := resolveCluster(params, ctx.ClusterRepo)
	if err != nil {
		return errorResult(err.Error())
	}

	dyn, err := createDynamicClient(cluster)
	if err != nil {
		return errorResult("connect error: " + err.Error())
	}

	ns := "default"
	if n, ok := params["namespace"].(string); ok && n != "" {
		ns = n
	}

	list, err := dyn.Resource(gvr).Namespace(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	var items []map[string]any
	for _, obj := range list.Items {
		items = append(items, unstructuredSummary(obj))
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

// --- utility functions ---

func getNamespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

func nodeReadyStatus(conds []corev1.NodeCondition) string {
	for _, c := range conds {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return fmt.Sprintf("NotReady (%s)", c.Reason)
		}
	}
	return "Unknown"
}

func servicePorts(ports []corev1.ServicePort) string {
	if len(ports) == 0 {
		return ""
	}
	var parts []string
	for _, p := range ports {
		if p.NodePort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	return strings.Join(parts, ", ")
}

func jobCompletions(j batchv1.Job) string {
	if j.Spec.Completions != nil {
		return fmt.Sprintf("%d/%d", j.Status.Succeeded, *j.Spec.Completions)
	}
	return fmt.Sprintf("%d/?", j.Status.Succeeded)
}

func jobStatus(j batchv1.Job) string {
	if j.Status.Failed > 0 {
		return "Failed"
	}
	if j.Status.Succeeded > 0 {
		return "Succeeded"
	}
	return "Running"
}

func ingressClass(ing networkingv1.Ingress) string {
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}
	return ing.Annotations["kubernetes.io/ingress.class"]
}

func containerStatuses(statuses []corev1.ContainerStatus) []map[string]any {
	var res []map[string]any
	for _, s := range statuses {
		state := "waiting"
		if s.State.Running != nil {
			state = "running"
		} else if s.State.Terminated != nil {
			state = "terminated"
		}
		res = append(res, map[string]any{
			"name":   s.Name,
			"ready":  s.Ready,
			"state":  state,
			"image":  s.Image,
			"restarts": s.RestartCount,
		})
	}
	return res
}

func podConditions(conds []corev1.PodCondition) []map[string]any {
	var res []map[string]any
	for _, c := range conds {
		res = append(res, map[string]any{
			"type":   string(c.Type),
			"status": string(c.Status),
			"reason": c.Reason,
		})
	}
	return res
}

func unstructuredSummary(obj unstructured.Unstructured) map[string]any {
	summary := map[string]any{
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
		"apiVersion": obj.GetAPIVersion(),
		"kind":      obj.GetKind(),
		"age":       obj.GetCreationTimestamp().Format("2006-01-02"),
	}
	if labels := obj.GetLabels(); len(labels) > 0 {
		summary["labels"] = labels
	}
	return summary
}

func cleanManagedFields(obj *unstructured.Unstructured) {
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
}

func extractDescribeInfo(obj *unstructured.Unstructured) map[string]any {
	info := make(map[string]any)

	cleanManagedFields(obj)

	info["apiVersion"] = obj.GetAPIVersion()
	info["kind"] = obj.GetKind()
	info["name"] = obj.GetName()
	info["namespace"] = obj.GetNamespace()
	info["uid"] = string(obj.GetUID())
	info["labels"] = obj.GetLabels()
	info["annotations"] = obj.GetAnnotations()
	info["creationTimestamp"] = obj.GetCreationTimestamp().Format(time.RFC3339)
	info["resourceVersion"] = obj.GetResourceVersion()

	// Extract spec summary
	if spec, ok := obj.Object["spec"].(map[string]any); ok {
		specSummary := make(map[string]any)
		for k, v := range spec {
			switch val := v.(type) {
			case string, bool, float64, int64:
				specSummary[k] = val
			case map[string]any:
				if len(val) < 5 {
					specSummary[k] = val
				} else {
					specSummary[k] = fmt.Sprintf("<complex: %d fields>", len(val))
				}
			case []any:
				specSummary[k] = fmt.Sprintf("<array: %d items>", len(val))
			}
		}
		info["spec"] = specSummary
	}

	// Extract status summary
	if status, ok := obj.Object["status"].(map[string]any); ok {
		statusSummary := make(map[string]any)
		for k, v := range status {
			switch val := v.(type) {
			case string, bool, float64, int64:
				statusSummary[k] = val
			case map[string]any:
				if len(val) < 5 {
					statusSummary[k] = val
				}
			}
		}
		if len(statusSummary) > 0 {
			info["status"] = statusSummary
		}
	}

	return info
}

func parseResourceArgs(args json.RawMessage, ctx *K8sExtendedToolContext) (*model.K8sCluster, schema.GroupVersionResource, string, string, error) {
	var params struct {
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`
		Kind        string `json:"kind"`
		APIVersion  string `json:"api_version"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, schema.GroupVersionResource{}, "", "", fmt.Errorf("invalid arguments: %v", err)
	}
	if params.Name == "" {
		return nil, schema.GroupVersionResource{}, "", "", fmt.Errorf("name is required")
	}
	if params.Kind == "" || params.APIVersion == "" {
		return nil, schema.GroupVersionResource{}, "", "", fmt.Errorf("kind and api_version are required")
	}

	clusterArgs := map[string]any{
		"cluster_id":   params.ClusterID,
		"cluster_name": params.ClusterName,
	}
	cluster, err := resolveCluster(clusterArgs, ctx.ClusterRepo)
	if err != nil {
		return nil, schema.GroupVersionResource{}, "", "", err
	}

	gvr, ns, err := gvrFromKind(params.Kind, params.APIVersion, params.Namespace)
	if err != nil {
		return nil, schema.GroupVersionResource{}, "", "", err
	}

	return cluster, gvr, params.Name, ns, nil
}

func gvrFromKind(kind, apiVersion, namespace string) (schema.GroupVersionResource, string, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// Try to guess
		gv = schema.GroupVersion{Group: "", Version: apiVersion}
	}

	resource := resourceName(kind)

	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}

	// Determine if namespaced
	ns := namespace
	if isClusterScoped(kind) {
		ns = ""
	} else if ns == "" {
		ns = "default"
	}

	return gvr, ns, nil
}

func resourceName(kind string) string {
	// Simple pluralization
	switch kind {
	case "Endpoints":
		return "endpoints"
	case "ServiceAccount":
		return "serviceaccounts"
	}

	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower
	}
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	return lower + "s"
}

func isClusterScoped(kind string) bool {
	switch kind {
	case "Node", "Namespace", "PersistentVolume", "StorageClass",
		"ClusterRole", "ClusterRoleBinding", "ValidatingWebhookConfiguration",
		"MutatingWebhookConfiguration", "CustomResourceDefinition", "APIService":
		return true
	}
	return false
}

func fmtTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func fmtMetav1Time(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func splitYAML(yamlContent string) []string {
	var docs []string
	for _, doc := range strings.Split(yamlContent, "---") {
		doc = strings.TrimSpace(doc)
		if doc != "" {
			docs = append(docs, doc)
		}
	}
	return docs
}
