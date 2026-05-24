package k8s

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

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
