package dms

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
)

type QueryLogHandler struct {
	queryLogRepo *repository.QueryLogRepository
}

func NewQueryLogHandler(queryLogRepo *repository.QueryLogRepository) *QueryLogHandler {
	return &QueryLogHandler{
		queryLogRepo: queryLogRepo,
	}
}

// ListQueryLogs 获取查询日志列表
// @Summary 获取查询日志列表
// @Tags DMS
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query string false "用户ID"
// @Param instance_id query int false "实例ID"
// @Param db_type query string false "数据库类型"
// @Param query_type query string false "查询类型"
// @Param status query string false "状态"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param search query string false "搜索关键词"
// @Success 200 {object} model.Response{data=object{list=[]model.QueryLog,total=int64}}
// @Router /api/dms/logs/queries [get]
func (h *QueryLogHandler) ListQueryLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	filters := make(map[string]interface{})
	if userID := c.Query("user_id"); userID != "" {
		filters["user_id"] = userID
	}
	if instanceIDStr := c.Query("instance_id"); instanceIDStr != "" {
		if instanceID, err := strconv.ParseUint(instanceIDStr, 10, 32); err == nil {
			filters["instance_id"] = uint(instanceID)
		}
	}
	if dbType := c.Query("db_type"); dbType != "" {
		filters["db_type"] = dbType
	}
	if queryType := c.Query("query_type"); queryType != "" {
		filters["query_type"] = queryType
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if startTime := c.Query("start_time"); startTime != "" {
		filters["start_time"] = startTime
	}
	if endTime := c.Query("end_time"); endTime != "" {
		filters["end_time"] = endTime
	}
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}

	logs, total, err := h.queryLogRepo.List(offset, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取日志列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"list":  logs,
		"total": total,
	}))
}

// GetQueryLog 获取查询日志详情
// @Summary 获取查询日志详情
// @Tags DMS
// @Accept json
// @Produce json
// @Param id path int true "日志ID"
// @Success 200 {object} model.Response{data=model.QueryLog}
// @Router /api/dms/logs/queries/:id [get]
func (h *QueryLogHandler) GetQueryLog(c *gin.Context) {
	var uriParams struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriParams); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	log, err := h.queryLogRepo.GetByID(uriParams.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "日志不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(log))
}
