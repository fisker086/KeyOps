package k8s

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

// GetPodList 获取 Pod 列表
// @Summary 获取 Pod 列表
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
// @Router /api/v1/kube/pod [get]
func (h *K8sHandler) GetPodList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	pods, err := h.service.GetPodList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(pods))
}

// GetPodDetail 获取 Pod 详情
// @Summary 获取 Pod 详情
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/detail [get]
func (h *K8sHandler) GetPodDetail(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")

	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and pod_name are required"))
		return
	}

	detail, err := h.service.GetPodDetail(clusterID, clusterName, namespace, podName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// GetContainersList 获取容器列表
// @Summary 获取容器列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Param pod_name query string true "Pod名称"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/containers [get]
func (h *K8sHandler) GetContainersList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")

	if podName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "pod_name is required"))
		return
	}

	containers, err := h.service.GetContainersList(clusterID, clusterName, uint(nodeID), uint(envID), namespace, podName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(containers))
}

// RestartPodRequest 重启 Pod 请求
type RestartPodRequest struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	NodeID      uint   `json:"node_id"`
	EnvID       uint   `json:"env_id"`
	Namespace   string `json:"namespace"`
	PodName     string `json:"pod_name" binding:"required"`
}

// RestartPod 重启 Pod
// @Summary 重启Pod
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RestartPodRequest true "重启Pod请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod [delete]
func (h *K8sHandler) RestartPod(c *gin.Context) {
	var req RestartPodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.RestartPod(req.ClusterID, req.ClusterName, req.NodeID, req.EnvID, req.Namespace, req.PodName); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success("ok"))
}

// GetPodMetrics 获取Pod指标
// @Summary 获取Pod指标
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param metrics_name query string true "指标名称"
// @Param last_time query int true "最近时间"
// @Param step query int true "步长"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/metrics [get]
func (h *K8sHandler) GetPodMetrics(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	metricsName := c.Query("metrics_name")
	lastTime, _ := strconv.Atoi(c.Query("last_time"))
	step, _ := strconv.Atoi(c.Query("step"))

	if namespace == "" || podName == "" || metricsName == "" || lastTime == 0 || step == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace, pod_name, metrics_name, last_time and step are required"))
		return
	}

	metrics, err := h.service.GetPodMetrics(clusterID, clusterName, namespace, podName, metricsName, uint(lastTime), uint(step))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(metrics))
}

// DownloadContainerLogs 下载容器日志
// @Summary 下载容器日志
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param container query string true "容器名称"
// @Param limit_bytes query int false "限制字节数"
// @Param since_second query int false "时间范围（秒）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/down_logs [get]
func (h *K8sHandler) DownloadContainerLogs(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	container := c.Query("container")
	limitBytes, _ := strconv.Atoi(c.Query("limit_bytes"))
	sinceSecond, _ := strconv.Atoi(c.Query("since_second"))

	if podName == "" || container == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "pod_name and container are required"))
		return
	}

	logs, err := h.service.DownloadContainerLogs(clusterID, clusterName, uint(nodeID), uint(envID), namespace, podName, container, limitBytes, sinceSecond)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(logs))
}
