package k8s

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	clusterService    *k8sService.K8sClusterService
	k8sService        *k8sService.K8sService
	permissionService *k8sService.K8sPermissionService
	roleRepo          *repository.RoleRepository
}

func NewSearchHandler(
	clusterService *k8sService.K8sClusterService,
	k8sService *k8sService.K8sService,
	permissionService *k8sService.K8sPermissionService,
	roleRepo *repository.RoleRepository,
) *SearchHandler {
	return &SearchHandler{
		clusterService:    clusterService,
		k8sService:        k8sService,
		permissionService: permissionService,
		roleRepo:          roleRepo,
	}
}

// SearchResult 搜索结果结构
type SearchResult struct {
	Type        string `json:"type"`        // cluster, node, pod, workload
	ID          string `json:"id"`          // 资源ID
	Name        string `json:"name"`        // 资源名称
	Namespace   string `json:"namespace,omitempty"`
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	IP          string `json:"ip,omitempty"`
	Kind        string `json:"kind,omitempty"` // Deployment, StatefulSet, DaemonSet
}

// GlobalSearch 全局搜索
// @Summary 全局搜索
// @Tags K8s Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string true "搜索关键词"
// @Success 200 {object} model.Response{data=map[string]interface{}}
// @Router /api/k8s/search [get]
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "搜索关键词不能为空"))
		return
	}

	// 获取当前用户ID
	userID, _ := c.Get("userID")
	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}

	// 获取用户的所有角色
	roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
	if err != nil {
		roles = []model.Role{}
	}

	// 检查是否是管理员
	isAdmin := false
	for _, role := range roles {
		if role.ID == "role:admin" {
			isAdmin = true
			break
		}
	}

	// 获取所有活跃的集群
	allClusters, err := h.clusterService.ListClusters()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取集群列表失败: "+err.Error()))
		return
	}

	// 获取用户有权限的集群
	clusterIDMap := make(map[string]bool)
	if isAdmin {
		for _, cluster := range allClusters {
			if cluster.Status == "active" {
				clusterIDMap[cluster.ID] = true
			}
		}
	} else {
		// 检查用户直接权限
		userPermissions, err := h.permissionService.GetPermissions(userIDStr)
		if err == nil {
			for _, perm := range userPermissions {
				if len(perm) >= 2 {
					path := perm[1]
					clusterID, _, _, _, err := k8sService.ParseResourcePath(path)
					if err == nil && clusterID != "" {
						clusterIDMap[clusterID] = true
					}
				}
			}
		}

		// 检查角色权限
		for _, role := range roles {
			rolePermissions, err := h.permissionService.GetPermissions(role.ID)
			if err == nil {
				for _, perm := range rolePermissions {
					if len(perm) >= 2 {
						path := perm[1]
						clusterID, _, _, _, err := k8sService.ParseResourcePath(path)
						if err == nil && clusterID != "" {
							clusterIDMap[clusterID] = true
						}
					}
				}
			}
		}
	}

	var results []SearchResult
	queryLower := strings.ToLower(query)

	// 搜索集群
	for _, cluster := range allClusters {
		if !clusterIDMap[cluster.ID] {
			continue
		}
		if strings.Contains(strings.ToLower(cluster.Name), queryLower) ||
			strings.Contains(strings.ToLower(cluster.APIServer), queryLower) {
			results = append(results, SearchResult{
				Type:        "cluster",
				ID:          cluster.ID,
				Name:        cluster.Name,
				ClusterID:   cluster.ID,
				ClusterName: cluster.Name,
				Status:      cluster.Status,
				Description: cluster.APIServer,
			})
		}
	}

	// 搜索节点、Pod和工作负载
	for _, cluster := range allClusters {
		if !clusterIDMap[cluster.ID] {
			continue
		}

		// 搜索节点
		nodes, err := h.k8sService.GetNodeList(cluster.ID, "", 0, 0)
		if err == nil {
			for _, node := range nodes {
				if strings.Contains(strings.ToLower(node.Name), queryLower) ||
					strings.Contains(strings.ToLower(node.InternalIP), queryLower) ||
					strings.Contains(strings.ToLower(node.ExternalIP), queryLower) {
					results = append(results, SearchResult{
						Type:        "node",
						ID:          node.Name,
						Name:        node.Name,
						ClusterID:   cluster.ID,
						ClusterName: cluster.Name,
						Status:      node.Status,
						Description: node.InternalIP,
						IP:          node.InternalIP,
					})
				}
			}
		}

		// 获取所有 namespace
		namespaces, err := h.k8sService.GetNamespaceList(cluster.ID, "")
		if err != nil {
			continue
		}

		// 搜索 Pod
		for _, ns := range namespaces {
			pods, err := h.k8sService.GetPodList(cluster.ID, "", 0, 0, ns.Name)
			if err == nil {
				for _, pod := range pods {
					if strings.Contains(strings.ToLower(pod.Name), queryLower) ||
						strings.Contains(strings.ToLower(pod.PodIP), queryLower) ||
						strings.Contains(strings.ToLower(pod.Namespace), queryLower) {
						results = append(results, SearchResult{
							Type:        "pod",
							ID:          pod.Name,
							Name:        pod.Name,
							Namespace:   pod.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Status:      pod.Status,
							Description: pod.Node,
							IP:          pod.PodIP,
						})
					}
				}
			}
		}

		// 搜索工作负载
		for _, ns := range namespaces {
			// Deployment
			deployments, err := h.k8sService.GetDeploymentList(cluster.ID, "", 0, 0, ns.Name)
			if err == nil {
				for _, deployment := range deployments {
					if strings.Contains(strings.ToLower(deployment.Name), queryLower) ||
						strings.Contains(strings.ToLower(deployment.Namespace), queryLower) {
						replicas := strconv.Itoa(int(deployment.Ready))
						status := "Available"
						if deployment.Available < deployment.UpToDate {
							status = "Updating"
						}
						results = append(results, SearchResult{
							Type:        "workload",
							ID:          deployment.Name,
							Name:        deployment.Name,
							Namespace:   deployment.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Status:      status,
							Kind:        "Deployment",
							Description: replicas,
						})
					}
				}
			}

			// StatefulSet
			statefulSets, err := h.k8sService.GetStatefulSetList(cluster.ID, "", 0, 0, ns.Name)
			if err == nil {
				for _, sts := range statefulSets {
					if strings.Contains(strings.ToLower(sts.Name), queryLower) ||
						strings.Contains(strings.ToLower(sts.Namespace), queryLower) {
						replicas := strconv.Itoa(int(sts.Ready))
						status := "Ready"
						results = append(results, SearchResult{
							Type:        "workload",
							ID:          sts.Name,
							Name:        sts.Name,
							Namespace:   sts.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Status:      status,
							Kind:        "StatefulSet",
							Description: replicas,
						})
					}
				}
			}

			// DaemonSet
			daemonSets, err := h.k8sService.GetDaemonSetList(cluster.ID, "", 0, 0, ns.Name)
			if err == nil {
				for _, ds := range daemonSets {
					if strings.Contains(strings.ToLower(ds.Name), queryLower) ||
						strings.Contains(strings.ToLower(ds.Namespace), queryLower) {
						status := "Ready"
						if ds.Ready < ds.Desired {
							status = "Updating"
						}
						results = append(results, SearchResult{
							Type:        "workload",
							ID:          ds.Name,
							Name:        ds.Name,
							Namespace:   ds.Namespace,
							ClusterID:   cluster.ID,
							ClusterName: cluster.Name,
							Status:      status,
							Kind:        "DaemonSet",
							Description: "DaemonSet",
						})
					}
				}
			}
		}
	}

	// 计算统计信息
	stats := struct {
		Cluster  int `json:"cluster"`
		Node     int `json:"node"`
		Pod      int `json:"pod"`
		Workload int `json:"workload"`
	}{}

	for _, result := range results {
		switch result.Type {
		case "cluster":
			stats.Cluster++
		case "node":
			stats.Node++
		case "pod":
			stats.Pod++
		case "workload":
			stats.Workload++
		}
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"results": results,
		"total":   len(results),
		"stats":   stats,
	}))
}

