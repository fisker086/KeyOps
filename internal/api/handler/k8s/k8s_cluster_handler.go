package k8s

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type K8sClusterHandler struct {
	clusterService    *k8sService.K8sClusterService
	permissionService *k8sService.K8sPermissionService
	roleRepo          *repository.RoleRepository
}

func NewK8sClusterHandler(clusterService *k8sService.K8sClusterService, permissionService *k8sService.K8sPermissionService, roleRepo *repository.RoleRepository) *K8sClusterHandler {
	return &K8sClusterHandler{
		clusterService:    clusterService,
		permissionService: permissionService,
		roleRepo:          roleRepo,
	}
}

// ListClusters 获取集群列表
// @Summary 获取集群列表
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters [get]
func (h *K8sClusterHandler) ListClusters(c *gin.Context) {
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

	// 获取所有活跃的集群
	allClusters, err := h.clusterService.ListClusters()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 从 Casbin 获取用户有权限的集群（管理员也按权限管理控制，不特殊对待）
	accessibleClusters := make([]model.K8sCluster, 0)
	clusterIDMap := make(map[string]bool)

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

	// 过滤出用户有权限的集群
	for _, cluster := range allClusters {
		if cluster.Status == "active" && clusterIDMap[cluster.ID] {
			accessibleClusters = append(accessibleClusters, cluster)
		}
	}

	c.JSON(http.StatusOK, model.Success(accessibleClusters))
}

// GetCluster 获取集群详情
// @Summary 获取集群详情
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/:id [get]
func (h *K8sClusterHandler) GetCluster(c *gin.Context) {
	clusterID := c.Param("id")

	// 获取当前用户ID
	userID, _ := c.Get("userID")
	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}

	// 获取用户的所有角色
	roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
	if err != nil {
		// 如果获取角色失败，继续检查用户直接权限
		roles = []model.Role{}
	}

	// 检查权限（使用新的 Casbin 权限系统）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionRead)
	if err == nil && hasPermission {
		// 用户有直接权限
	} else if len(roles) > 0 {
		// 检查角色权限
		for _, role := range roles {
			hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionRead)
			if err == nil && hasPermission {
				break
			}
			// 管理员角色默认拥有所有权限
			if role.ID == "role:admin" {
				hasPermission = true
				break
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, model.Error(403, "没有访问该集群的权限"))
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(cluster))
}

// CreateClusterRequest 创建集群请求
type CreateClusterRequest struct {
	Name             string `json:"name" binding:"required"`
	Description      string `json:"description"`
	APIServer        string `json:"apiServer"` // 使用 kubeconfig 时可选，会自动从 kubeconfig 中提取
	Token            string `json:"token"`
	Kubeconfig       string `json:"kubeconfig"`
	AuthType         string `json:"authType"`
	Version          string `json:"version"`
	Region           string `json:"region"`
	Environment      string `json:"environment"`
	DefaultNamespace string `json:"defaultNamespace"`
}

// CreateCluster 创建集群
// @Summary 创建集群
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateClusterRequest true "创建集群请求"
// @Success 201 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters [post]
func (h *K8sClusterHandler) CreateCluster(c *gin.Context) {
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前用户ID
	userID, _ := c.Get("userID")
	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}

	cluster := &model.K8sCluster{
		ID:               uuid.New().String(),
		Name:             req.Name,
		Description:      req.Description,
		APIServer:        req.APIServer,
		Token:            req.Token,
		Kubeconfig:       req.Kubeconfig,
		AuthType:         req.AuthType,
		Version:          req.Version,
		Region:           req.Region,
		Environment:      req.Environment,
		DefaultNamespace: req.DefaultNamespace,
		Status:           "active",
		CreatedBy:        userIDStr,
	}

	if cluster.AuthType == "" {
		if cluster.Token != "" {
			cluster.AuthType = "token"
		} else if cluster.Kubeconfig != "" {
			cluster.AuthType = "kubeconfig"
		} else {
			c.JSON(http.StatusBadRequest, model.Error(400, "必须提供Token或Kubeconfig"))
			return
		}
	}

	// 如果提供了Token或Kubeconfig，验证连接并自动获取版本
	if cluster.Token == "" && cluster.Kubeconfig == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "必须提供Token或Kubeconfig"))
		return
	}

	// 如果使用 token 认证，API Server 是必需的
	if cluster.AuthType == "token" && cluster.APIServer == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "使用Token认证时，API Server地址是必需的"))
		return
	}

	// CreateCluster会自动测试连接并获取版本，如果连接失败会返回错误
	if err := h.clusterService.CreateCluster(cluster); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 重新获取集群信息，确保返回最新的数据（包括版本信息）
	createdCluster, err := h.clusterService.GetCluster(cluster.ID)
	if err != nil {
		// 如果获取失败，仍然返回创建的集群对象
		c.JSON(http.StatusCreated, model.Success(cluster))
		return
	}

	c.JSON(http.StatusCreated, model.Success(createdCluster))
}

// UpdateClusterRequest 更新集群请求
type UpdateClusterRequest struct {
	Description      string `json:"description"`
	APIServer        string `json:"apiServer"`
	Token            string `json:"token"`
	Kubeconfig       string `json:"kubeconfig"`
	AuthType         string `json:"authType"`
	Version          string `json:"version"`
	Region           string `json:"region"`
	Environment      string `json:"environment"`
	Status           string `json:"status"`
	DefaultNamespace string `json:"defaultNamespace"`
}

// UpdateCluster 更新集群
// @Summary 更新集群
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Param request body UpdateClusterRequest true "更新集群请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/:id [put]
func (h *K8sClusterHandler) UpdateCluster(c *gin.Context) {
	clusterID := c.Param("id")

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

	// 检查权限（使用新的 Casbin 权限系统，需要 admin 权限）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionAdmin)
	if err == nil && hasPermission {
		// 用户有直接权限
	} else if len(roles) > 0 {
		// 检查角色权限
		for _, role := range roles {
			// 管理员角色默认拥有所有权限
			if role.ID == "role:admin" {
				hasPermission = true
				break
			}
			hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionAdmin)
			if err == nil && hasPermission {
				break
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, model.Error(403, "没有修改该集群的权限"))
		return
	}

	var req UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取现有集群
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 更新字段
	if req.Description != "" {
		cluster.Description = req.Description
	}
	if req.APIServer != "" {
		cluster.APIServer = req.APIServer
	}
	if req.Token != "" {
		cluster.Token = req.Token
	}
	if req.Kubeconfig != "" {
		cluster.Kubeconfig = req.Kubeconfig
	}
	if req.AuthType != "" {
		cluster.AuthType = req.AuthType
	}
	if req.Version != "" {
		cluster.Version = req.Version
	}
	if req.Region != "" {
		cluster.Region = req.Region
	}
	if req.Environment != "" {
		cluster.Environment = req.Environment
	}
	if req.Status != "" {
		cluster.Status = req.Status
	}
	if req.DefaultNamespace != "" {
		cluster.DefaultNamespace = req.DefaultNamespace
	}

	if err := h.clusterService.UpdateCluster(cluster); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(cluster))
}

// DeleteCluster 删除集群
// @Summary 删除集群
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Success 200 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/:id [delete]
func (h *K8sClusterHandler) DeleteCluster(c *gin.Context) {
	clusterID := c.Param("id")

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

	// 检查权限（使用新的 Casbin 权限系统，需要 admin 权限）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionAdmin)
	if err == nil && hasPermission {
		// 用户有直接权限
	} else if len(roles) > 0 {
		// 检查角色权限
		for _, role := range roles {
			// 管理员角色默认拥有所有权限
			if role.ID == "role:admin" {
				hasPermission = true
				break
			}
			hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionAdmin)
			if err == nil && hasPermission {
				break
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, model.Error(403, "没有删除该集群的权限"))
		return
	}

	if err := h.clusterService.DeleteCluster(clusterID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success("ok"))
}

// GetClusterSummary 获取集群摘要
// @Summary 获取集群摘要
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/:id/summary [get]
func (h *K8sClusterHandler) GetClusterSummary(c *gin.Context) {
	clusterID := c.Param("id")

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

	// 检查权限（使用新的 Casbin 权限系统）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionRead)
	if err == nil && hasPermission {
		// 用户有直接权限
	} else if len(roles) > 0 {
		// 检查角色权限
		for _, role := range roles {
			hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, "", k8sService.ResourceTypeNamespace, "", k8sService.ActionRead)
			if err == nil && hasPermission {
				break
			}
			// 管理员角色默认拥有所有权限
			if role.ID == "role:admin" {
				hasPermission = true
				break
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, model.Error(403, "没有访问该集群的权限"))
		return
	}

	summary, err := h.clusterService.GetClusterSummary(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(summary))
}

// GetAllClustersSummary 获取所有集群摘要
// @Summary 获取所有集群摘要
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/summary [get]
func (h *K8sClusterHandler) GetAllClustersSummary(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 从 Casbin 获取用户有权限的集群
	clusterIDMap := make(map[string]bool)

	// 如果是管理员，所有集群都有权限
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

	// 获取每个集群的摘要
	summaries := make([]*k8sService.ClusterSummary, 0)
	for _, cluster := range allClusters {
		if cluster.Status == "active" && clusterIDMap[cluster.ID] {
			summary, err := h.clusterService.GetClusterSummary(cluster.ID)
			if err != nil {
				// 如果获取摘要失败，跳过该集群
				continue
			}
			summaries = append(summaries, summary)
		}
	}

	c.JSON(http.StatusOK, model.Success(summaries))
}

// GetDashboardStatistics 获取 K8s 大盘统计数据
// @Summary 获取 K8s 大盘统计数据
// @Tags K8s Cluster
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/dashboard/statistics [get]
func (h *K8sClusterHandler) GetDashboardStatistics(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 从 Casbin 获取用户有权限的集群
	clusterIDMap := make(map[string]bool)

	// 如果是管理员，所有集群都有权限
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

	// 获取每个集群的摘要并聚合统计数据
	var totalClusters, healthyClusters, unhealthyClusters int
	var totalNodes, readyNodes, notReadyNodes int
	var totalPods, runningPods, pendingPods, failedPods int
	var totalDeployments, totalStatefulSets, totalDaemonSets, totalServices, totalIngresses, totalNamespaces int

	clusterDist := []map[string]interface{}{}
	podDist := []map[string]interface{}{}
	nodeDist := []map[string]interface{}{}
	versionMap := make(map[string]int)

	for _, cluster := range allClusters {
		if cluster.Status == "active" && clusterIDMap[cluster.ID] {
			totalClusters++
			healthyClusters++

			// 版本分布
			version := cluster.Version
			if version == "" {
				version = "unknown"
			}
			versionMap[version]++

			// 获取集群摘要
			summary, err := h.clusterService.GetClusterSummary(cluster.ID)
			if err != nil {
				// 如果获取摘要失败，跳过该集群
				continue
			}

			if summary.K8sStatus != nil {
				status := summary.K8sStatus

				// 聚合节点统计
				totalNodes += status.NodeCount
				readyNodes += status.ReadyNodes
				notReadyNodes += status.NotReadyNodes

				// 聚合 Pod 统计
				totalPods += status.TotalPods
				runningPods += status.RunningPods
				pendingPods += status.PendingPods
				failedPods += status.FailedPods

				// 聚合工作负载统计
				totalDeployments += status.DeploymentCount
				totalStatefulSets += status.StatefulSetCount
				totalDaemonSets += status.DaemonSetCount
				totalServices += status.ServiceCount
				totalIngresses += status.IngressCount
				totalNamespaces += status.NamespaceCount

				// 集群分布
				clusterDist = append(clusterDist, map[string]interface{}{
					"clusterId":   summary.ClusterID,
					"clusterName": summary.ClusterName,
					"value":       1,
				})

				// Pod 分布
				podDist = append(podDist, map[string]interface{}{
					"clusterId":   summary.ClusterID,
					"clusterName": summary.ClusterName,
					"value":       status.TotalPods,
				})

				// Node 分布
				nodeDist = append(nodeDist, map[string]interface{}{
					"clusterId":   summary.ClusterID,
					"clusterName": summary.ClusterName,
					"value":       status.NodeCount,
				})
			}
		}
	}

	// 版本分布
	versionDist := []map[string]interface{}{}
	for version, count := range versionMap {
		versionDist = append(versionDist, map[string]interface{}{
			"version": version,
			"count":   count,
		})
	}

	statistics := map[string]interface{}{
		"totalClusters":       totalClusters,
		"healthyClusters":     healthyClusters,
		"unhealthyClusters":   unhealthyClusters,
		"totalNodes":          totalNodes,
		"readyNodes":          readyNodes,
		"notReadyNodes":       notReadyNodes,
		"totalPods":           totalPods,
		"runningPods":         runningPods,
		"pendingPods":         pendingPods,
		"failedPods":          failedPods,
		"totalDeployments":    totalDeployments,
		"totalStatefulSets":   totalStatefulSets,
		"totalDaemonSets":     totalDaemonSets,
		"totalServices":       totalServices,
		"totalIngresses":      totalIngresses,
		"totalNamespaces":     totalNamespaces,
		"clusterDistribution": clusterDist,
		"podDistribution":     podDist,
		"nodeDistribution":    nodeDist,
		"versionDistribution": versionDist,
	}

	c.JSON(http.StatusOK, model.Success(statistics))
}
