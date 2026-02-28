package audit

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuditHandler struct{}

func NewAuditHandler() *AuditHandler {
	return &AuditHandler{}
}

// GetOperationLogs 获取操作日志列表
// @Summary 获取操作日志列表
// @Tags 操作审计
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username query string false "用户名"
// @Param ip query string false "IP地址"
// @Param path query string false "API路径"
// @Param method query string false "HTTP方法"
// @Param status query int false "状态码"
// @Param start_time query string false "开始时间 (格式: 2006-01-02 15:04:05)"
// @Param end_time query string false "结束时间 (格式: 2006-01-02 15:04:05)"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.Response{data=model.PaginatedResponse{data=[]model.OperationLog}}
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/audit/operation-logs [get]
func (h *AuditHandler) GetOperationLogs(c *gin.Context) {
	var req struct {
		Username  string `form:"username"`
		IP        string `form:"ip"`
		Path      string `form:"path"`
		Method    string `form:"method"`
		Status    int    `form:"status"`
		StartTime string `form:"start_time"`
		EndTime   string `form:"end_time"`
		Page      int    `form:"page"`
		PageSize  int    `form:"page_size"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 构建查询
	query := database.DB.Model(&model.OperationLog{})

	// 过滤条件
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.IP != "" {
		query = query.Where("ip LIKE ?", "%"+req.IP+"%")
	}
	if req.Path != "" {
		query = query.Where("path LIKE ?", "%"+req.Path+"%")
	}
	if req.Method != "" {
		query = query.Where("method = ?", req.Method)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}
	if req.StartTime != "" {
		query = query.Where("start_time >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("start_time <= ?", req.EndTime)
	}

	// 只查询 K8s 相关的操作（路径包含 /kube/ 或 /k8s/）
	query = query.Where("path LIKE ? OR path LIKE ?", "%/kube/%", "%/k8s/%")

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "查询失败: "+err.Error()))
		return
	}

	// 分页查询
	var logs []model.OperationLog
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("start_time DESC").Offset(offset).Limit(req.PageSize).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "查询失败: "+err.Error()))
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, model.Success(model.PaginatedResponse{
		Data:       logs,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int((total + int64(req.PageSize) - 1) / int64(req.PageSize)),
	}))
}

// GetOperationLogDetail 获取操作日志详情
// @Summary 获取操作日志详情
// @Tags 操作审计
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "日志ID"
// @Success 200 {object} model.Response{data=model.OperationLog}
// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/audit/operation-logs/{id} [get]
func (h *AuditHandler) GetOperationLogDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的ID"))
		return
	}

	var log model.OperationLog
	if err := database.DB.First(&log, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, model.Error(404, "日志不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, "查询失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(log))
}

// DeleteOperationLog 删除操作日志
// @Summary 删除操作日志
// @Tags 操作审计
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "日志ID"
// @Success 200 {object} model.Response
// @Failure 404 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/audit/operation-logs/{id} [delete]
func (h *AuditHandler) DeleteOperationLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的ID"))
		return
	}

	if err := database.DB.Delete(&model.OperationLog{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "删除失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// BatchDeleteOperationLogs 批量删除操作日志
// @Summary 批量删除操作日志
// @Tags 操作审计
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param ids body []uint true "日志ID列表"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/audit/operation-logs/batch [delete]
func (h *AuditHandler) BatchDeleteOperationLogs(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := database.DB.Where("id IN ?", req.IDs).Delete(&model.OperationLog{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "批量删除失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetPodCommandLogs 获取 Pod 命令日志列表
// @Summary 获取 Pod 命令日志列表
// @Tags 操作审计
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID"
// @Param namespace query string false "命名空间"
// @Param pod_name query string false "Pod名称"
// @Param username query string false "用户名"
// @Param command query string false "命令（模糊匹配）"
// @Param start_time query string false "开始时间 (格式: 2006-01-02 15:04:05)"
// @Param end_time query string false "结束时间 (格式: 2006-01-02 15:04:05)"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.Response{data=model.PaginatedResponse{data=[]model.PodCommandRecord}}
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/audit/pod-commands [get]
func (h *AuditHandler) GetPodCommandLogs(c *gin.Context) {
	var req struct {
		ClusterID string `form:"cluster_id"`
		Namespace string `form:"namespace"`
		PodName   string `form:"pod_name"`
		Username  string `form:"username"`
		Command   string `form:"command"`
		StartTime string `form:"start_time"`
		EndTime   string `form:"end_time"`
		Page      int    `form:"page"`
		PageSize  int    `form:"page_size"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 构建查询
	query := database.DB.Model(&model.PodCommandRecord{})

	// 过滤条件
	if req.ClusterID != "" {
		query = query.Where("cluster_id = ?", req.ClusterID)
	}
	if req.Namespace != "" {
		query = query.Where("namespace LIKE ?", "%"+req.Namespace+"%")
	}
	if req.PodName != "" {
		query = query.Where("pod_name LIKE ?", "%"+req.PodName+"%")
	}
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Command != "" {
		query = query.Where("command LIKE ?", "%"+req.Command+"%")
	}
	if req.StartTime != "" {
		query = query.Where("executed_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("executed_at <= ?", req.EndTime)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "查询失败: "+err.Error()))
		return
	}

	// 分页查询
	var logs []model.PodCommandRecord
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("executed_at DESC").Offset(offset).Limit(req.PageSize).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "查询失败: "+err.Error()))
		return
	}

	// 批量查询集群名称并填充
	if len(logs) > 0 {
		// 收集所有需要查询的 cluster_id
		clusterIDSet := make(map[string]bool)
		for _, log := range logs {
			if log.ClusterID != "" && log.ClusterName == "" {
				clusterIDSet[log.ClusterID] = true
			}
		}

		// 批量查询集群名称
		if len(clusterIDSet) > 0 {
			clusterIDs := make([]string, 0, len(clusterIDSet))
			for id := range clusterIDSet {
				clusterIDs = append(clusterIDs, id)
			}

			var clusters []model.K8sCluster
			if err := database.DB.Select("id, name").Where("id IN ?", clusterIDs).Find(&clusters).Error; err == nil {
				// 构建 cluster_id -> cluster_name 映射
				clusterNameMap := make(map[string]string)
				for _, cluster := range clusters {
					clusterNameMap[cluster.ID] = cluster.Name
				}

				// 填充集群名称
				for i := range logs {
					if logs[i].ClusterName == "" {
						if name, ok := clusterNameMap[logs[i].ClusterID]; ok {
							logs[i].ClusterName = name
						}
					}
				}
			}
		}
	}

	// 返回结果
	c.JSON(http.StatusOK, model.Success(model.PaginatedResponse{
		Data:       logs,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int((total + int64(req.PageSize) - 1) / int64(req.PageSize)),
	}))
}

