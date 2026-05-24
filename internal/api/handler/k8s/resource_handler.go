package k8s

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

// GetResourceYaml 获取 K8s 资源的 YAML 内容
// @Summary 获取 K8s 资源的 YAML 内容
// @Tags K8s
// @Accept json
// @Produce text/plain
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string false "命名空间（PV和StorageClass不需要）"
// @Param resource_type query string true "资源类型（pod, service, ingress, deployment, daemonset, statefulset, job, cronjob, pv, pvc, storageclass, configmap, secret, destinationrule, gateway, virtualservice）"
// @Param resource_name query string true "资源名称"
// @Success 200 {string} string "YAML内容"
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/yaml [get]
func (h *K8sHandler) GetResourceYaml(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	resourceType := c.Query("resource_type")
	resourceName := c.Query("resource_name")

	// PV 和 StorageClass 是集群级别的资源，不需要 namespace
	resourceTypeLower := strings.ToLower(resourceType)
	clusterLevelResources := map[string]bool{
		"pv":            true,
		"storageclass":  true,
		"sc":            true,
	}

	// 对于需要 namespace 的资源，检查 namespace 参数
	if !clusterLevelResources[resourceTypeLower] && namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填（PV和StorageClass除外）"))
		return
	}

	if resourceType == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_type参数必填"))
		return
	}

	if resourceName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_name参数必填"))
		return
	}

	yamlContent, err := h.service.GetResourceYaml(clusterID, clusterName, namespace, resourceType, resourceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, yamlContent)
}

// UpdateResourceYamlRequest 更新资源 YAML 请求
type UpdateResourceYamlRequest struct {
	ClusterID    string `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	Namespace    string `json:"namespace"`
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceName string `json:"resource_name" binding:"required"`
	Yaml         string `json:"yaml" binding:"required"`
}

// UpdateResourceYaml 更新 K8s 资源的 YAML 内容
// @Summary 更新 K8s 资源的 YAML 内容
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateResourceYamlRequest true "更新资源 YAML 请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/yaml [put]
func (h *K8sHandler) UpdateResourceYaml(c *gin.Context) {
	var req UpdateResourceYamlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// PV 和 StorageClass 是集群级别的资源，不需要 namespace
	resourceTypeLower := strings.ToLower(req.ResourceType)
	clusterLevelResources := map[string]bool{
		"pv":           true,
		"storageclass": true,
		"sc":           true,
	}

	// 对于需要 namespace 的资源，检查 namespace 参数
	if !clusterLevelResources[resourceTypeLower] && req.Namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填（PV和StorageClass除外）"))
		return
	}

	// 权限检查：编辑 YAML 需要 write 或 admin 权限
	if req.ClusterID != "" && !clusterLevelResources[resourceTypeLower] {
		// 获取当前用户ID
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusForbidden, model.Error(403, "没有编辑该资源的权限"))
			return
		}
		userIDStr := userID.(string)

		roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
		if err != nil {
			roles = []model.Role{}
		}

		hasPermission := h.permissionService.CheckYamlEditPermission(userIDStr, req.ClusterID, req.Namespace, k8sService.GetResourceTypeFromString(req.ResourceType), req.ResourceName, roles)
		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有编辑该资源的权限，需要 write 或 admin 权限"))
			return
		}
	}

	err := h.service.UpdateResourceYaml(req.ClusterID, req.ClusterName, req.Namespace, req.ResourceType, req.ResourceName, req.Yaml)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// DeleteResource 删除 K8s 资源
// @Summary 删除 K8s 资源
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string true "集群ID"
// @Param cluster_name query string false "集群名称"
// @Param namespace query string false "命名空间（PV和StorageClass不需要）"
// @Param resource_type query string true "资源类型（service, ingress, deployment, daemonset, statefulset, pvc 等）"
// @Param resource_name query string true "资源名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/yaml [delete]
func (h *K8sHandler) DeleteResource(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	resourceType := c.Query("resource_type")
	resourceName := c.Query("resource_name")

	resourceTypeLower := strings.ToLower(resourceType)
	clusterLevelResources := map[string]bool{
		"pv":           true,
		"storageclass": true,
		"sc":           true,
	}

	if !clusterLevelResources[resourceTypeLower] && namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填（PV和StorageClass除外）"))
		return
	}
	if resourceType == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_type参数必填"))
		return
	}
	if resourceName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_name参数必填"))
		return
	}

	err := h.service.DeleteResource(clusterID, clusterName, namespace, resourceType, resourceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// DryRunResourceYamlRequest Dry-run 资源 YAML 请求
type DryRunResourceYamlRequest struct {
	ClusterID    string `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	Namespace    string `json:"namespace"`
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceName string `json:"resource_name" binding:"required"`
	Yaml         string `json:"yaml" binding:"required"`
}

// DryRunResourceYaml Dry-run 预览 K8s 资源变更
// @Summary Dry-run 预览 K8s 资源变更
// @Tags K8s
// @Accept json
// @Produce text/plain
// @Security BearerAuth
// @Param request body DryRunResourceYamlRequest true "Dry-run 资源 YAML 请求"
// @Success 200 {string} string "Dry-run 结果 YAML"
// @Failure 400 {object} model.Response
// @Failure 403 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/yaml/dry-run [post]
func (h *K8sHandler) DryRunResourceYaml(c *gin.Context) {
	var req DryRunResourceYamlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 提供更详细的错误信息
		errorMsg := err.Error()
		if errorMsg == "EOF" {
			errorMsg = "请求体为空或格式错误，请检查请求内容"
		}
		// 记录详细错误信息用于调试
		logger.Errorf("Dry-run 请求解析失败: %v, ContentLength: %d, ContentType: %s",
			err, c.Request.ContentLength, c.Request.Header.Get("Content-Type"))
		c.JSON(http.StatusBadRequest, model.Error(400, errorMsg))
		return
	}

	// 验证必填字段
	if req.ResourceType == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_type 参数必填"))
		return
	}
	if req.ResourceName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "resource_name 参数必填"))
		return
	}
	if req.Yaml == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "yaml 参数必填且不能为空"))
		return
	}

	// PV 和 StorageClass 是集群级别的资源，不需要 namespace
	resourceTypeLower := strings.ToLower(req.ResourceType)
	clusterLevelResources := map[string]bool{
		"pv":           true,
		"storageclass": true,
		"sc":           true,
	}

	// 对于需要 namespace 的资源，检查 namespace 参数
	if !clusterLevelResources[resourceTypeLower] && req.Namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填（PV和StorageClass除外）"))
		return
	}

	// 权限检查：Dry-run 也需要 write 或 admin 权限（因为 Dry-run 会验证 YAML，相当于预览编辑）
	if req.ClusterID != "" && !clusterLevelResources[resourceTypeLower] {
		// 获取当前用户ID
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusForbidden, model.Error(403, "没有预览该资源变更的权限"))
			return
		}
		userIDStr := userID.(string)

		roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
		if err != nil {
			roles = []model.Role{}
		}

		hasPermission := h.permissionService.CheckYamlEditPermission(userIDStr, req.ClusterID, req.Namespace, k8sService.GetResourceTypeFromString(req.ResourceType), req.ResourceName, roles)
		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有预览该资源变更的权限，需要 write 或 admin 权限"))
			return
		}
	}

	yamlContent, err := h.service.DryRunResourceYaml(req.ClusterID, req.ClusterName, req.Namespace, req.ResourceType, req.ResourceName, req.Yaml)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, yamlContent)
}
