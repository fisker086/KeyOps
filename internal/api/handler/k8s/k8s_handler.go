package k8s

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（开发环境，生产环境需要配置）
	},
}

type K8sHandler struct {
	service          *k8sService.K8sService
	permissionService *k8sService.K8sPermissionService
	roleRepo         *repository.RoleRepository
	authService      *service.AuthService
}

func NewK8sHandler(service *k8sService.K8sService, permissionService *k8sService.K8sPermissionService, roleRepo *repository.RoleRepository, authService *service.AuthService) *K8sHandler {
	return &K8sHandler{
		service:          service,
		permissionService: permissionService,
		roleRepo:         roleRepo,
		authService:      authService,
	}
}

// GetBaseInfo 获取 Kubernetes 基础信息
// @Summary 获取 Kubernetes 基础信息
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
// @Router /api/v1/kube/base [get]
func (h *K8sHandler) GetBaseInfo(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	info, err := h.service.GetBaseInfo(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(info))
}

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

// GetServiceList 获取 Service 列表
// @Summary 获取 Service 列表
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
// @Router /api/v1/kube/service [get]
func (h *K8sHandler) GetServiceList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	services, err := h.service.GetServiceList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(services))
}

// GetIngressList 获取 Ingress 列表
// @Summary 获取 Ingress 列表
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
// @Router /api/v1/kube/ingress [get]
func (h *K8sHandler) GetIngressList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	ingresses, err := h.service.GetIngressList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(ingresses))
}

// GetHPAList 获取 HPA 列表
// @Summary 获取 HPA 列表
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
// @Router /api/v1/kube/hpa [get]
func (h *K8sHandler) GetHPAList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	hpas, err := h.service.GetHPAList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(hpas))
}

// GetEventList 获取 Event 列表
// @Summary 获取 Event 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Param namespace query string false "命名空间"
// @Param object_name query string true "对象名称"
// @Param object_kind query string true "对象类型"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/event [get]
func (h *K8sHandler) GetEventList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")
	objectName := c.Query("object_name")
	objectKind := c.Query("object_kind")

	if objectName == "" || objectKind == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "object_name and object_kind are required"))
		return
	}

	events, err := h.service.GetEventList(clusterID, clusterName, uint(nodeID), uint(envID), namespace, objectName, objectKind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(events))
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

// GetReplica 获取副本数
// @Summary 获取副本数
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
// @Router /api/v1/kube/scale [get]
func (h *K8sHandler) GetReplica(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	replica, err := h.service.GetReplica(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(replica))
}

// ScaleReplicaRequest 扩缩容请求
type ScaleReplicaRequest struct {
	ClusterID       string `json:"cluster_id"`
	ClusterName     string `json:"cluster_name"`
	NodeID          uint   `json:"node_id"`
	EnvID           uint   `json:"env_id"`
	Namespace       string `json:"namespace"`
	DeploymentName  string `json:"deployment_name"` // 新增：Deployment名称
	DesiredReplicas uint   `json:"desired_replicas" binding:"required"`
}

// ScaleReplica 扩缩容
// @Summary 扩缩容副本
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ScaleReplicaRequest true "扩缩容请求"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/scale [post]
func (h *K8sHandler) ScaleReplica(c *gin.Context) {
	var req ScaleReplicaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	replica, err := h.service.ScaleReplica(req.ClusterID, req.ClusterName, req.NodeID, req.EnvID, req.Namespace, req.DeploymentName, req.DesiredReplicas)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(replica))
}

// RestartPodRequest 重启 Pod 请求
type RestartPodRequest struct {
	ClusterID string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	NodeID    uint   `json:"node_id"`
	EnvID     uint   `json:"env_id"`
	Namespace string `json:"namespace"`
	PodName   string `json:"pod_name" binding:"required"`
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
		"revisions":       revisions,
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
		"revisions":       revisions,
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
		"revisions":       revisions,
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

// GetNamespaceList 获取命名空间列表
// @Summary 获取命名空间列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/namespace [get]
func (h *K8sHandler) GetNamespaceList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")

	namespaces, err := h.service.GetNamespaceList(clusterID, clusterName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(namespaces))
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

// GetNodeList 获取 Node 列表
// @Summary 获取 Node 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param node_id query int false "应用ID（兼容旧方式）"
// @Param env_id query int false "环境ID（兼容旧方式）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/node [get]
func (h *K8sHandler) GetNodeList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))

	nodes, err := h.service.GetNodeList(clusterID, clusterName, uint(nodeID), uint(envID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nodes))
}

// GetPVList 获取 PV 列表
// @Summary 获取 PV 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pv [get]
func (h *K8sHandler) GetPVList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))

	pvs, err := h.service.GetPVList(clusterID, clusterName, uint(nodeID), uint(envID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(pvs))
}

// GetStorageClassList 获取 StorageClass 列表
// @Summary 获取 StorageClass 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/storageclass [get]
func (h *K8sHandler) GetStorageClassList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))

	storageClasses, err := h.service.GetStorageClassList(clusterID, clusterName, uint(nodeID), uint(envID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(storageClasses))
}

// GetPVCList 获取 PVC 列表
// @Summary 获取 PVC 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pvc [get]
func (h *K8sHandler) GetPVCList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	pvcs, err := h.service.GetPVCList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(pvcs))
}

// GetConfigMapList 获取 ConfigMap 列表
// @Summary 获取 ConfigMap 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/configmap [get]
func (h *K8sHandler) GetConfigMapList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	configMaps, err := h.service.GetConfigMapList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(configMaps))
}

// GetSecretList 获取 Secret 列表
// @Summary 获取 Secret 列表
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string false "命名空间"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/secret [get]
func (h *K8sHandler) GetSecretList(c *gin.Context) {
	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	nodeID, _ := strconv.Atoi(c.Query("node_id"))
	envID, _ := strconv.Atoi(c.Query("env_id"))
	namespace := c.Query("namespace")

	secrets, err := h.service.GetSecretList(clusterID, clusterName, uint(nodeID), uint(envID), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(secrets))
}

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

// getResourceTypeFromString 将字符串资源类型转换为 ResourceType
func (h *K8sHandler) getResourceTypeFromString(resourceType string) k8sService.ResourceType {
	resourceTypeLower := strings.ToLower(resourceType)
	switch resourceTypeLower {
	case "deployment":
		return k8sService.ResourceTypeDeployment
	case "statefulset":
		return k8sService.ResourceTypeStatefulSet
	case "service":
		return k8sService.ResourceTypeService
	case "pod":
		return k8sService.ResourceTypePod
	case "ingress":
		return k8sService.ResourceTypeIngress
	default:
		// 对于其他资源类型（如 ConfigMap、Secret、PV等），使用 namespace 作为资源类型
		return k8sService.ResourceTypeNamespace
	}
}

// checkYamlEditPermission 检查 YAML 编辑权限
func (h *K8sHandler) checkYamlEditPermission(c *gin.Context, clusterID string, namespace string, resourceType string, resourceName string) bool {
	// 获取当前用户ID
	userID, exists := c.Get("userID")
	if !exists {
		return false
	}
	userIDStr := userID.(string)

	// 获取用户的所有角色
	roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
	if err != nil {
		roles = []model.Role{}
	}

	// 将资源类型转换为 ResourceType
	k8sResourceType := h.getResourceTypeFromString(resourceType)

	// 检查用户权限（需要 write 或 admin 权限才能编辑）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, namespace, k8sResourceType, resourceName, k8sService.ActionWrite)
	if err == nil && !hasPermission {
		// 尝试检查 admin 权限
		hasPermission, err = h.permissionService.CheckPermission(userIDStr, clusterID, namespace, k8sResourceType, resourceName, k8sService.ActionAdmin)
	}

	// 如果用户没有直接权限，检查角色权限
	if err == nil && !hasPermission && len(roles) > 0 {
		for _, role := range roles {
			// 管理员角色默认拥有所有权限
			if role.ID == "role:admin" {
				hasPermission = true
				break
			}
			// 检查角色的 write 权限
			hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sResourceType, resourceName, k8sService.ActionWrite)
			if err == nil && hasPermission {
				break
			}
			// 检查角色的 admin 权限
			if !hasPermission {
				hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sResourceType, resourceName, k8sService.ActionAdmin)
				if err == nil && hasPermission {
					break
				}
			}
		}
	}

	return hasPermission
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
		hasPermission := h.checkYamlEditPermission(c, req.ClusterID, req.Namespace, req.ResourceType, req.ResourceName)
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
		hasPermission := h.checkYamlEditPermission(c, req.ClusterID, req.Namespace, req.ResourceType, req.ResourceName)
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

// StreamPodLogs 流式传输 Pod 日志
// @Summary 流式传输 Pod 日志
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param container query string false "容器名称"
// @Param follow query bool false "是否跟随日志（默认false）"
// @Param tail_lines query int false "显示最后N行（默认100）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/ws/logs [get]
func (h *K8sHandler) StreamPodLogs(c *gin.Context) {
	// 验证 token（从 query 参数获取，因为 WebSocket 不能使用 Authorization header）
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, model.Error(401, "缺少 token 参数"))
		return
	}

	// 验证 token 并获取用户信息
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
		return
	}

	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	container := c.Query("container")
	follow := c.Query("follow") == "true"
	tailLines, _ := strconv.Atoi(c.Query("tail_lines"))

	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and pod_name are required"))
		return
	}

	// 权限检查（WebSocket 请求需要手动检查权限）
	if clusterID != "" {
		userID := claims.UserID
		roles, err := h.roleRepo.GetRolesByUserID(userID)
		if err != nil {
			roles = []model.Role{}
		}

		// 检查用户权限
		hasPermission, err := h.permissionService.CheckPermission(userID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionRead)
		if err == nil && !hasPermission && len(roles) > 0 {
			// 检查角色权限
			for _, role := range roles {
				if role.ID == "role:admin" {
					hasPermission = true
					break
				}
				hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionRead)
				if err == nil && hasPermission {
					break
				}
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有访问该资源的权限"))
			return
		}
	}

	// 升级到 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("升级到 WebSocket 失败: %v", err)))
		return
	}
	defer ws.Close()

	// 创建日志服务
	logsService := k8sService.NewPodLogsService(h.service)

	// 流式传输日志
	if err := logsService.StreamPodLogs(clusterID, clusterName, namespace, podName, container, follow, tailLines, ws); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
		return
	}
}

// ConnectPodTerminal 连接 Pod 终端
// @Summary 连接 Pod 终端
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param container query string false "容器名称"
// @Param command query string false "执行的命令（默认/bin/sh）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/ws/terminal [get]
func (h *K8sHandler) ConnectPodTerminal(c *gin.Context) {
	logger.Infof("[PodTerminal] 收到 Pod 终端连接请求: %s %s", c.Request.Method, c.Request.URL.String())

	// 验证 token（从 query 参数获取，因为 WebSocket 不能使用 Authorization header）
	token := c.Query("token")
	if token == "" {
		logger.Warnf("[PodTerminal] 缺少 token 参数")
		c.JSON(http.StatusUnauthorized, model.Error(401, "缺少 token 参数"))
		return
	}

	// 验证 token 并获取用户信息
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		logger.Errorf("[PodTerminal] Token 验证失败: %v", err)
		c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
		return
	}
	logger.Infof("[PodTerminal] Token 验证成功，用户ID: %s", claims.UserID)

	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	container := c.Query("container")
	command := c.Query("command")

	logger.Infof("[PodTerminal] 请求参数: clusterID=%s, namespace=%s, podName=%s, container=%s, command=%s",
		clusterID, namespace, podName, container, command)

	if namespace == "" || podName == "" {
		logger.Warnf("[PodTerminal] 缺少必要参数: namespace=%s, podName=%s", namespace, podName)
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and pod_name are required"))
		return
	}

	// 权限检查（WebSocket 请求需要手动检查权限，终端需要 write 权限）
	if clusterID != "" {
		userID := claims.UserID
		roles, err := h.roleRepo.GetRolesByUserID(userID)
		if err != nil {
			roles = []model.Role{}
		}

		// 检查用户权限（终端需要 write 权限）
		hasPermission, err := h.permissionService.CheckPermission(userID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionWrite)
		if err == nil && !hasPermission && len(roles) > 0 {
			// 检查角色权限
			for _, role := range roles {
				if role.ID == "role:admin" {
					hasPermission = true
					break
				}
				hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionWrite)
				if err == nil && hasPermission {
					break
				}
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有访问该资源的权限"))
			return
		}
	}

	// 升级到 WebSocket
	logger.Infof("[PodTerminal] 开始升级到 WebSocket")
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("[PodTerminal] 升级到 WebSocket 失败: %v", err)
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("升级到 WebSocket 失败: %v", err)))
		return
	}
	defer ws.Close()

	logger.Infof("[PodTerminal] WebSocket 连接已建立: clusterID=%s, namespace=%s, pod=%s", clusterID, namespace, podName)

	// 创建终端服务
	terminalService := k8sService.NewPodTerminalService(h.service)

	// 获取用户名
	username := ""
	var user model.User
	if err := database.DB.First(&user, "id = ?", claims.UserID).Error; err == nil {
		username = user.Username
	}

	// 处理终端连接（这会阻塞直到连接关闭）
	logger.Infof("[PodTerminal] 开始处理终端连接")
	if err := terminalService.HandlePodTerminal(clusterID, clusterName, namespace, podName, container, command, claims.UserID, username, ws); err != nil {
		logger.Errorf("[PodTerminal] 处理 Pod 终端连接失败: %v", err)
		// 确保错误消息发送到客户端
		errorMsg := fmt.Sprintf("\r\n\x1b[31m[错误] %v\x1b[0m\r\n", err)
		ws.WriteMessage(websocket.TextMessage, []byte(errorMsg))
		// 等待一下确保消息发送完成
		time.Sleep(500 * time.Millisecond)
		return
	}

	logger.Infof("[PodTerminal] Pod 终端连接处理完成")
}
