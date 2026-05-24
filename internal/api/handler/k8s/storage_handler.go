package k8s

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

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
