package k8s

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/gin-gonic/gin"
)

type K8sPermissionHandler struct {
	permissionService *k8sService.K8sPermissionService
	clusterService    *k8sService.K8sClusterService
	roleRepo          *repository.RoleRepository
}

func NewK8sPermissionHandler(permissionService *k8sService.K8sPermissionService, clusterService *k8sService.K8sClusterService, roleRepo *repository.RoleRepository) *K8sPermissionHandler {
	return &K8sPermissionHandler{
		permissionService: permissionService,
		clusterService:    clusterService,
		roleRepo:          roleRepo,
	}
}

// AddPermissionRequest 添加权限请求
type AddPermissionRequest struct {
	UserID       *string `json:"userId,omitempty"` // 用户ID（用户级权限）
	RoleID       *string `json:"roleId,omitempty"` // 角色ID（角色级权限）
	ClusterID    string  `json:"clusterId" binding:"required"`
	Namespace    string  `json:"namespace" binding:"required"`
	ResourceType string  `json:"resourceType"`              // deployment, statefulset, service, pod, ingress, namespace
	ResourceName string  `json:"resourceName"`              // 资源名称（可选）
	Action       string  `json:"action" binding:"required"` // read, write, delete, admin
}

// AddPermission 添加权限
// @Summary 添加K8s资源权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body AddPermissionRequest true "添加权限请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/permissions [post]
func (h *K8sPermissionHandler) AddPermission(c *gin.Context) {
	var req AddPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 验证：必须提供userId或roleId之一
	if req.UserID == nil && req.RoleID == nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "必须提供userId或roleId"))
		return
	}

	// 确定资源类型
	resourceType := k8sService.ResourceTypeNamespace
	switch strings.ToLower(req.ResourceType) {
	case "deployment":
		resourceType = k8sService.ResourceTypeDeployment
	case "statefulset":
		resourceType = k8sService.ResourceTypeStatefulSet
	case "service":
		resourceType = k8sService.ResourceTypeService
	case "pod":
		resourceType = k8sService.ResourceTypePod
	case "ingress":
		resourceType = k8sService.ResourceTypeIngress
	case "pvc":
		resourceType = k8sService.ResourceTypePVC
	case "daemonset":
		resourceType = k8sService.ResourceTypeDaemonSet
	case "namespace":
		resourceType = k8sService.ResourceTypeNamespace
	default:
		if req.ResourceName != "" {
			c.JSON(http.StatusBadRequest, model.Error(400, "无效的资源类型"))
			return
		}
	}

	// 确定操作类型
	action := k8sService.ActionRead
	switch req.Action {
	case "read":
		action = k8sService.ActionRead
	case "write":
		action = k8sService.ActionWrite
	case "delete":
		action = k8sService.ActionDelete
	case "admin":
		action = k8sService.ActionAdmin
	default:
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的操作类型"))
		return
	}

	// 添加权限
	var sub string
	if req.UserID != nil {
		sub = *req.UserID
	} else {
		sub = *req.RoleID
	}

	success, err := h.permissionService.AddPermission(sub, req.ClusterID, req.Namespace, resourceType, req.ResourceName, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	if !success {
		c.JSON(http.StatusBadRequest, model.Error(400, "权限已存在"))
		return
	}

	// 重新加载策略
	if err := k8sService.ReloadPolicy(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "重新加载策略失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success("权限添加成功"))
}

// UpdatePermissionRequest 更新权限请求（原子操作）
type UpdatePermissionRequest struct {
	Old struct {
		UserID       *string `json:"userId,omitempty"`
		RoleID       *string `json:"roleId,omitempty"`
		ClusterID    string  `json:"clusterId" binding:"required"`
		Namespace    string  `json:"namespace" binding:"required"`
		ResourceType string  `json:"resourceType"`
		ResourceName string  `json:"resourceName"`
		Action       string  `json:"action" binding:"required"`
	} `json:"old" binding:"required"`
	New struct {
		UserID       *string `json:"userId,omitempty"`
		RoleID       *string `json:"roleId,omitempty"`
		ClusterID    string  `json:"clusterId" binding:"required"`
		Namespace    string  `json:"namespace" binding:"required"`
		ResourceType string  `json:"resourceType"`
		ResourceName string  `json:"resourceName"`
		Action       string  `json:"action" binding:"required"`
	} `json:"new" binding:"required"`
}

// UpdatePermission 更新权限（原子操作，使用 Casbin UpdatePolicy）
// @Summary 更新K8s资源权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdatePermissionRequest true "更新权限请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/permissions [put]
func (h *K8sPermissionHandler) UpdatePermission(c *gin.Context) {
	var req UpdatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	parseResourceType := func(rt string) k8sService.ResourceType {
		switch strings.ToLower(rt) {
		case "deployment":
			return k8sService.ResourceTypeDeployment
		case "statefulset":
			return k8sService.ResourceTypeStatefulSet
		case "daemonset":
			return k8sService.ResourceTypeDaemonSet
		case "service":
			return k8sService.ResourceTypeService
		case "pod":
			return k8sService.ResourceTypePod
		case "ingress":
			return k8sService.ResourceTypeIngress
		case "pvc":
			return k8sService.ResourceTypePVC
		default:
			return k8sService.ResourceTypeNamespace
		}
	}
	parseAction := func(a string) (k8sService.Action, bool) {
		switch a {
		case "read":
			return k8sService.ActionRead, true
		case "write":
			return k8sService.ActionWrite, true
		case "delete":
			return k8sService.ActionDelete, true
		case "admin":
			return k8sService.ActionAdmin, true
		default:
			return k8sService.ActionRead, false
		}
	}

	if (req.Old.UserID == nil && req.Old.RoleID == nil) || (req.New.UserID == nil && req.New.RoleID == nil) {
		c.JSON(http.StatusBadRequest, model.Error(400, "old 和 new 都必须提供 userId 或 roleId 之一"))
		return
	}

	oldAction, ok := parseAction(req.Old.Action)
	if !ok {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的 old.action"))
		return
	}
	newAction, ok := parseAction(req.New.Action)
	if !ok {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的 new.action"))
		return
	}

	var oldSub, newSub string
	if req.Old.UserID != nil {
		oldSub = *req.Old.UserID
	} else {
		oldSub = *req.Old.RoleID
	}
	if req.New.UserID != nil {
		newSub = *req.New.UserID
	} else {
		newSub = *req.New.RoleID
	}

	success, err := h.permissionService.UpdatePermission(
		oldSub, req.Old.ClusterID, req.Old.Namespace, parseResourceType(req.Old.ResourceType), req.Old.ResourceName, oldAction,
		newSub, req.New.ClusterID, req.New.Namespace, parseResourceType(req.New.ResourceType), req.New.ResourceName, newAction,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	if !success {
		c.JSON(http.StatusBadRequest, model.Error(400, "权限不存在或更新失败"))
		return
	}

	if err := k8sService.ReloadPolicy(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "重新加载策略失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success("权限更新成功"))
}

// RemovePermissionRequest 删除权限请求
type RemovePermissionRequest struct {
	UserID       *string `json:"userId,omitempty"`
	RoleID       *string `json:"roleId,omitempty"`
	ClusterID    string  `json:"clusterId" binding:"required"`
	Namespace    string  `json:"namespace" binding:"required"`
	ResourceType string  `json:"resourceType"`
	ResourceName string  `json:"resourceName"`
	Action       string  `json:"action" binding:"required"`
}

// RemovePermission 删除权限
// @Summary 删除K8s资源权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RemovePermissionRequest true "删除权限请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/permissions [delete]
func (h *K8sPermissionHandler) RemovePermission(c *gin.Context) {
	var req RemovePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 确定资源类型和操作类型（与AddPermission相同）
	resourceType := k8sService.ResourceTypeNamespace
	switch strings.ToLower(req.ResourceType) {
	case "deployment":
		resourceType = k8sService.ResourceTypeDeployment
	case "statefulset":
		resourceType = k8sService.ResourceTypeStatefulSet
	case "service":
		resourceType = k8sService.ResourceTypeService
	case "pod":
		resourceType = k8sService.ResourceTypePod
	case "ingress":
		resourceType = k8sService.ResourceTypeIngress
	case "pvc":
		resourceType = k8sService.ResourceTypePVC
	case "daemonset":
		resourceType = k8sService.ResourceTypeDaemonSet
	case "namespace":
		resourceType = k8sService.ResourceTypeNamespace
	}

	action := k8sService.ActionRead
	switch strings.ToLower(req.Action) {
	case "read":
		action = k8sService.ActionRead
	case "write":
		action = k8sService.ActionWrite
	case "delete":
		action = k8sService.ActionDelete
	case "admin":
		action = k8sService.ActionAdmin
	default:
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的操作类型"))
		return
	}

	var sub string
	if req.UserID != nil {
		sub = *req.UserID
	} else if req.RoleID != nil {
		sub = *req.RoleID
	} else {
		c.JSON(http.StatusBadRequest, model.Error(400, "必须提供userId或roleId"))
		return
	}

	success, err := h.permissionService.RemovePermission(sub, req.ClusterID, req.Namespace, resourceType, req.ResourceName, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	if !success {
		c.JSON(http.StatusBadRequest, model.Error(400, "权限不存在"))
		return
	}

	// 重新加载策略
	if err := k8sService.ReloadPolicy(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "重新加载策略失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success("权限删除成功"))
}

// GetPermissions 获取权限列表
// @Summary 获取K8s资源权限列表
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId query string false "用户ID"
// @Param roleId query string false "角色ID"
// @Param clusterId query string false "集群ID"
// @Success 200 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/permissions [get]
func (h *K8sPermissionHandler) GetPermissions(c *gin.Context) {
	userID := c.Query("userId")
	roleID := c.Query("roleId")
	clusterID := c.Query("clusterId")

	var allPermissions [][]string

	// 如果指定了userId或roleId，获取该用户/角色的权限
	if userID != "" {
		permissions, err := h.permissionService.GetPermissions(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
			return
		}
		allPermissions = append(allPermissions, permissions...)
	}

	if roleID != "" {
		permissions, err := h.permissionService.GetPermissions(roleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
			return
		}
		allPermissions = append(allPermissions, permissions...)
	}

	// 过滤集群ID（如果指定）
	if clusterID != "" {
		filtered := make([][]string, 0)
		for _, perm := range allPermissions {
			if len(perm) >= 2 {
				path := perm[1]
				parsedClusterID, _, _, _, err := k8sService.ParseResourcePath(path)
				if err == nil && parsedClusterID == clusterID {
					filtered = append(filtered, perm)
				}
			}
		}
		allPermissions = filtered
	}

	c.JSON(http.StatusOK, model.Success(allPermissions))
}

// PermissionInfo 权限信息
type PermissionInfo struct {
	Sub          string `json:"sub"` // 用户ID或角色ID
	ClusterID    string `json:"clusterId"`
	Namespace    string `json:"namespace"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	Action       string `json:"action"`
}

// GetClusterPermissions 获取集群的所有权限（兼容旧API）
// @Summary 获取集群的所有权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Success 200 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/clusters/:id/permissions [get]
func (h *K8sPermissionHandler) GetClusterPermissions(c *gin.Context) {
	clusterID := c.Param("id")

	// 获取所有权限策略，然后过滤
	allPolicies, err := k8sService.GetAllPolicies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	permissions := make([]PermissionInfo, 0)
	for _, policy := range allPolicies {
		if len(policy) >= 3 {
			sub := policy[0]
			path := policy[1]
			action := policy[2]

			parsedClusterID, namespace, resourceType, resourceName, err := k8sService.ParseResourcePath(path)
			if err == nil && parsedClusterID == clusterID {
				permissions = append(permissions, PermissionInfo{
					Sub:          sub,
					ClusterID:    parsedClusterID,
					Namespace:    namespace,
					ResourceType: string(resourceType),
					ResourceName: resourceName,
					Action:       action,
				})
			}
		}
	}

	c.JSON(http.StatusOK, model.Success(permissions))
}

// CheckPermissionRequest 检查权限请求
type CheckPermissionRequest struct {
	ClusterID    string `json:"cluster_id"`
	Namespace    string `json:"namespace"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Action       string `json:"action" binding:"required"` // read, write, delete, admin
}

// CheckPermission 检查当前用户是否有指定权限
// @Summary 检查K8s资源权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CheckPermissionRequest true "检查权限请求"
// @Success 200 {object} model.Response{data=bool}
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/k8s/permissions/check [post]
func (h *K8sPermissionHandler) CheckPermission(c *gin.Context) {
	var req CheckPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}
	userIDStr := userID.(string)

	// 确定资源类型
	resourceType := k8sService.ResourceTypeNamespace
	switch strings.ToLower(req.ResourceType) {
	case "deployment":
		resourceType = k8sService.ResourceTypeDeployment
	case "statefulset":
		resourceType = k8sService.ResourceTypeStatefulSet
	case "service":
		resourceType = k8sService.ResourceTypeService
	case "pod":
		resourceType = k8sService.ResourceTypePod
	case "ingress":
		resourceType = k8sService.ResourceTypeIngress
	case "namespace":
		resourceType = k8sService.ResourceTypeNamespace
	}

	// 确定操作类型
	action := k8sService.ActionRead
	switch req.Action {
	case "read":
		action = k8sService.ActionRead
	case "write":
		action = k8sService.ActionWrite
	case "delete":
		action = k8sService.ActionDelete
	case "admin":
		action = k8sService.ActionAdmin
	default:
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的操作类型"))
		return
	}

	// 检查用户权限
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, req.ClusterID, req.Namespace, resourceType, req.ResourceName, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 如果用户没有直接权限，检查角色权限
	if !hasPermission && h.roleRepo != nil {
		// 获取用户角色
		roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
		if err == nil && len(roles) > 0 {
			for _, role := range roles {
				// 管理员角色默认拥有所有权限
				if role.ID == "role:admin" {
					hasPermission = true
					break
				}
				// 检查角色权限
				hasPermission, err = h.permissionService.CheckPermission(role.ID, req.ClusterID, req.Namespace, resourceType, req.ResourceName, action)
				if err == nil && hasPermission {
					break
				}
			}
		}
	}

	c.JSON(http.StatusOK, model.Success(hasPermission))
}

// GetEffectiveAction 获取当前用户在指定 cluster/namespace 下对某资源类型的最高权限
// 用于前端按权限显示扩缩容、回滚、删除等按钮
// @Summary 获取有效操作权限
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param clusterId path string true "集群ID"
// @Param namespace query string true "命名空间"
// @Param resourceType query string true "资源类型 deployment/statefulset/daemonset/pod 等"
// @Success 200 {object} model.Response{data=string}
// @Router /api/k8s/clusters/:clusterId/effective-action [get]
func (h *K8sPermissionHandler) GetEffectiveAction(c *gin.Context) {
	clusterID := c.Param("id")
	namespace := c.Query("namespace")
	resourceTypeStr := strings.ToLower(c.DefaultQuery("resourceType", "namespace"))

	if clusterID == "" || namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "clusterId 和 namespace 必填"))
		return
	}

	resourceType := k8sService.ResourceTypeNamespace
	switch resourceTypeStr {
	case "deployment":
		resourceType = k8sService.ResourceTypeDeployment
	case "statefulset":
		resourceType = k8sService.ResourceTypeStatefulSet
	case "daemonset":
		resourceType = k8sService.ResourceTypeDaemonSet
	case "pod":
		resourceType = k8sService.ResourceTypePod
	case "service":
		resourceType = k8sService.ResourceTypeService
	case "ingress":
		resourceType = k8sService.ResourceTypeIngress
	case "pvc":
		resourceType = k8sService.ResourceTypePVC
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}
	userIDStr, _ := userID.(string)

	subs := []string{userIDStr}
	if h.roleRepo != nil {
		roles, _ := h.roleRepo.GetRolesByUserID(userIDStr)
		for _, role := range roles {
			subs = append(subs, role.ID)
		}
	}

	action, err := h.permissionService.GetEffectiveActionForSubs(clusterID, namespace, resourceType, subs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 无权限时返回 read（保守：仅展示查看类按钮）
	result := string(action)
	if result == "" {
		result = "read"
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// GetPermittedNamespaces 获取当前用户在指定集群下可访问的命名空间列表
// @Summary 获取可访问的命名空间
// @Tags K8s Permission
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "集群ID"
// @Success 200 {object} model.Response{data=[]string}
// @Router /api/k8s/clusters/:id/permitted-namespaces [get]
func (h *K8sPermissionHandler) GetPermittedNamespaces(c *gin.Context) {
	clusterID := c.Param("id")
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "集群ID不能为空"))
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}
	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Error(401, "用户ID格式错误"))
		return
	}

	subs := []string{userIDStr}
	if h.roleRepo != nil {
		roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
		if err == nil {
			for _, role := range roles {
				subs = append(subs, role.ID)
			}
		}
	}

	namespaces, err := h.permissionService.GetPermittedNamespacesForSubs(clusterID, subs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(namespaces))
}
