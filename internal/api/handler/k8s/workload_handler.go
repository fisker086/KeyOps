package k8s

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

// GetDeploymentList 获取 Deployment 列表
// @Summary 获取 Deployment 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/deployment [get]
func (h *K8sHandler) GetDeploymentList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	deployments, err := h.service.GetDeploymentList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(deployments))
}

// GetDeploymentRevisions 获取 Deployment 历史版本列表
// @Summary 获取 Deployment 历史版本列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param deployment_name path string true "Deployment名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/deployment/{deployment_name}/revisions [get]
func (h *K8sHandler) GetDeploymentRevisions(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	deploymentName := c.Param("deployment_name")

	if namespace == "" || deploymentName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and deployment_name are required"))
		return
	}

	revisions, currentRevision, err := h.service.GetDeploymentRevisions(clusterID, clusterName, namespace, deploymentName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"revisions":        revisions,
		"current_revision": currentRevision,
	}))
}

// RollbackDeployment 回滚 Deployment 到指定版本
// @Summary 回滚 Deployment 到指定版本
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param deployment_name path string true "Deployment名称"
// @Param to_revision body int true "目标版本号"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/deployment/{deployment_name}/rollback [post]
func (h *K8sHandler) RollbackDeployment(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	deploymentName := c.Param("deployment_name")

	var req struct {
		ToRevision int64 `json:"to_revision" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if namespace == "" || deploymentName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and deployment_name are required"))
		return
	}

	err := h.service.RollbackDeployment(clusterID, clusterName, namespace, deploymentName, req.ToRevision)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"message": fmt.Sprintf("Deployment '%s' 已成功回滚到版本 %d", deploymentName, req.ToRevision),
	}))
}

// GetDeploymentMetrics 获取 Deployment 监控数据
// @Summary 获取 Deployment 监控数据
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param deployment_name path string true "Deployment名称"
// @Param last_time query int true "最近时间（秒）"
// @Param step query int true "步长（秒）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/deployment/{deployment_name}/metrics [get]
func (h *K8sHandler) GetDeploymentMetrics(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	deploymentName := c.Param("deployment_name")
	lastTime, _ := strconv.Atoi(c.Query("last_time"))
	step, _ := strconv.Atoi(c.Query("step"))

	if namespace == "" || deploymentName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and deployment_name are required"))
		return
	}

	if lastTime == 0 {
		lastTime = 3600 // 默认1小时
	}
	if step == 0 {
		step = 300 // 默认5分钟
	}

	metrics, err := h.service.GetDeploymentMetrics(clusterID, clusterName, namespace, deploymentName, uint(lastTime), uint(step))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(metrics))
}

// GetDeploymentDetail 获取 Deployment 详情
// @Summary 获取 Deployment 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param deployment_name path string true "Deployment名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/deployment/{deployment_name} [get]
func (h *K8sHandler) GetDeploymentDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	deploymentName := c.Param("deployment_name")

	if namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填"))
		return
	}

	if deploymentName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "deployment_name参数必填"))
		return
	}

	detail, err := h.service.GetDeploymentDetail(clusterID, clusterName, namespace, deploymentName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// GetDaemonSetMetrics 获取 DaemonSet 监控数据
// @Summary 获取 DaemonSet 监控数据
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param daemonset_name path string true "DaemonSet名称"
// @Param last_time query int true "最近时间（秒）"
// @Param step query int true "步长（秒）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/daemonset/{daemonset_name}/metrics [get]
func (h *K8sHandler) GetDaemonSetMetrics(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	daemonSetName := c.Param("daemonset_name")
	lastTime, _ := strconv.Atoi(c.Query("last_time"))
	step, _ := strconv.Atoi(c.Query("step"))

	if namespace == "" || daemonSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and daemonset_name are required"))
		return
	}

	if lastTime == 0 {
		lastTime = 3600 // 默认1小时
	}
	if step == 0 {
		step = 300 // 默认5分钟
	}

	metrics, err := h.service.GetDaemonSetMetrics(clusterID, clusterName, namespace, daemonSetName, uint(lastTime), uint(step))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(metrics))
}

// GetDaemonSetRevisions 获取 DaemonSet 历史版本列表
// @Summary 获取 DaemonSet 历史版本列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param daemonset_name path string true "DaemonSet名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/daemonset/{daemonset_name}/revisions [get]
func (h *K8sHandler) GetDaemonSetRevisions(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	daemonSetName := c.Param("daemonset_name")

	if namespace == "" || daemonSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and daemonset_name are required"))
		return
	}

	revisions, currentRevision, err := h.service.GetDaemonSetRevisions(clusterID, clusterName, namespace, daemonSetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"revisions":        revisions,
		"current_revision": currentRevision,
	}))
}

// RollbackDaemonSet 回滚 DaemonSet 到指定版本
// @Summary 回滚 DaemonSet 到指定版本
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param daemonset_name path string true "DaemonSet名称"
// @Param request body object true "回滚请求" SchemaExample({"to_revision": 2})
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/daemonset/{daemonset_name}/rollback [post]
func (h *K8sHandler) RollbackDaemonSet(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	daemonSetName := c.Param("daemonset_name")

	var req struct {
		ToRevision int64 `json:"to_revision" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if namespace == "" || daemonSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and daemonset_name are required"))
		return
	}

	err := h.service.RollbackDaemonSet(clusterID, clusterName, namespace, daemonSetName, req.ToRevision)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"message": fmt.Sprintf("DaemonSet '%s' 已成功回滚到版本 %d", daemonSetName, req.ToRevision),
	}))
}

// GetStatefulSetList 获取 StatefulSet 列表
// @Summary 获取 StatefulSet 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/statefulset [get]
func (h *K8sHandler) GetStatefulSetList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	statefulsets, err := h.service.GetStatefulSetList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(statefulsets))
}

// GetStatefulSetDetail 获取 StatefulSet 详情
// @Summary 获取 StatefulSet 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param statefulset_name path string true "StatefulSet名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/statefulset/{statefulset_name} [get]
func (h *K8sHandler) GetStatefulSetDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	statefulSetName := c.Param("statefulset_name")

	if namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填"))
		return
	}

	if statefulSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "statefulset_name参数必填"))
		return
	}

	detail, err := h.service.GetStatefulSetDetail(clusterID, clusterName, namespace, statefulSetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// GetStatefulSetMetrics 获取 StatefulSet 监控数据
// @Summary 获取 StatefulSet 监控数据
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param statefulset_name path string true "StatefulSet名称"
// @Param last_time query int true "最近时间（秒）"
// @Param step query int true "步长（秒）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/statefulset/{statefulset_name}/metrics [get]
func (h *K8sHandler) GetStatefulSetMetrics(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	statefulSetName := c.Param("statefulset_name")
	lastTime, _ := strconv.Atoi(c.Query("last_time"))
	step, _ := strconv.Atoi(c.Query("step"))

	if namespace == "" || statefulSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and statefulset_name are required"))
		return
	}

	if lastTime == 0 {
		lastTime = 3600 // 默认1小时
	}
	if step == 0 {
		step = 300 // 默认5分钟
	}

	metrics, err := h.service.GetStatefulSetMetrics(clusterID, clusterName, namespace, statefulSetName, uint(lastTime), uint(step))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(metrics))
}

// GetStatefulSetRevisions 获取 StatefulSet 历史版本列表
// @Summary 获取 StatefulSet 历史版本列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param statefulset_name path string true "StatefulSet名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/statefulset/{statefulset_name}/revisions [get]
func (h *K8sHandler) GetStatefulSetRevisions(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	statefulSetName := c.Param("statefulset_name")

	if namespace == "" || statefulSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and statefulset_name are required"))
		return
	}

	revisions, currentRevision, err := h.service.GetStatefulSetRevisions(clusterID, clusterName, namespace, statefulSetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"revisions":        revisions,
		"current_revision": currentRevision,
	}))
}

// RollbackStatefulSet 回滚 StatefulSet 到指定版本
// @Summary 回滚 StatefulSet 到指定版本
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param statefulset_name path string true "StatefulSet名称"
// @Param request body object true "回滚请求" SchemaExample({"to_revision": 2})
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/statefulset/{statefulset_name}/rollback [post]
func (h *K8sHandler) RollbackStatefulSet(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	statefulSetName := c.Param("statefulset_name")

	var req struct {
		ToRevision int64 `json:"to_revision" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if namespace == "" || statefulSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and statefulset_name are required"))
		return
	}

	err := h.service.RollbackStatefulSet(clusterID, clusterName, namespace, statefulSetName, req.ToRevision)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"message": fmt.Sprintf("StatefulSet '%s' 已成功回滚到版本 %d", statefulSetName, req.ToRevision),
	}))
}

// GetDaemonSetList 获取 DaemonSet 列表
// @Summary 获取 DaemonSet 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/daemonset [get]
func (h *K8sHandler) GetDaemonSetList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	daemonsets, err := h.service.GetDaemonSetList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(daemonsets))
}

// GetDaemonSetDetail 获取 DaemonSet 详情
// @Summary 获取 DaemonSet 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param daemonset_name path string true "DaemonSet名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/daemonset/{daemonset_name} [get]
func (h *K8sHandler) GetDaemonSetDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	daemonSetName := c.Param("daemonset_name")

	if namespace == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace参数必填"))
		return
	}

	if daemonSetName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "daemonset_name参数必填"))
		return
	}

	detail, err := h.service.GetDaemonSetDetail(clusterID, clusterName, namespace, daemonSetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// GetCronJobList 获取 CronJob 列表
// @Summary 获取 CronJob 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/cronjob [get]
func (h *K8sHandler) GetCronJobList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	cronjobs, err := h.service.GetCronJobList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(cronjobs))
}

// GetJobList 获取 Job 列表
// @Summary 获取 Job 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/job [get]
func (h *K8sHandler) GetJobList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	jobs, err := h.service.GetJobList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(jobs))
}

// GetJobDetail 获取 Job 详情
// @Summary 获取 Job 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param job_name path string true "Job名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/job/{job_name} [get]
func (h *K8sHandler) GetJobDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	jobName := c.Param("job_name")

	if namespace == "" || jobName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and job_name are required"))
		return
	}

	detail, err := h.service.GetJobDetail(clusterID, clusterName, namespace, jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// GetCronJobDetail 获取 CronJob 详情
// @Summary 获取 CronJob 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param cronjob_name path string true "CronJob名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/cronjob/{cronjob_name} [get]
func (h *K8sHandler) GetCronJobDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	cronJobName := c.Param("cronjob_name")

	if namespace == "" || cronJobName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and cronjob_name are required"))
		return
	}

	detail, err := h.service.GetCronJobDetail(clusterID, clusterName, namespace, cronJobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}
