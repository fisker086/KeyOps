package k8s

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

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
// @Param object_name query string false "对象名称"
// @Param object_kind query string false "对象类型"
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

	events, err := h.service.GetEventList(clusterID, clusterName, uint(nodeID), uint(envID), namespace, objectName, objectKind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(events))
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
