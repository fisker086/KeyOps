package dms

import (
	"fmt"
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service/dms"
	"github.com/gin-gonic/gin"
)

type QueryHandler struct {
	queryService *dms.QueryService
}

func NewQueryHandler(queryService *dms.QueryService) *QueryHandler {
	return &QueryHandler{
		queryService: queryService,
	}
}

// ExecuteQuery 执行查询
// @Summary 执行SQL/MongoDB/Redis查询
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.ExecuteQueryRequest true "查询请求"
// @Success 200 {object} model.Response{data=dms.QueryResult}
// @Router /api/dms/query/execute [post]
func (h *QueryHandler) ExecuteQuery(c *gin.Context) {
	var req dms.ExecuteQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	username, _ := c.Get("username")
	userIDStr := userID.(string)
	usernameStr := username.(string)

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.queryService.ExecuteQuery(&req, userIDStr, usernameStr, clientIP, userAgent)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetDatabases 获取数据库列表
// @Summary 获取数据库列表
// @Tags DMS
// @Accept json
// @Produce json
// @Param instance_id query int true "实例ID"
// @Success 200 {object} model.Response{data=[]string}
// @Router /api/dms/query/databases [get]
func (h *QueryHandler) GetDatabases(c *gin.Context) {
	instanceIDStr := c.Query("instance_id")
	if instanceIDStr == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "instance_id 参数不能为空"))
		return
	}

	var instanceID uint
	if _, err := fmt.Sscanf(instanceIDStr, "%d", &instanceID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的实例ID"))
		return
	}

	databases, err := h.queryService.GetDatabases(instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(databases))
}

// GetTables 获取表列表
// @Summary 获取表列表
// @Tags DMS
// @Accept json
// @Produce json
// @Param instance_id query int true "实例ID"
// @Param database_name query string true "数据库名"
// @Success 200 {object} model.Response{data=[]string}
// @Router /api/dms/query/tables [get]
func (h *QueryHandler) GetTables(c *gin.Context) {
	instanceIDStr := c.Query("instance_id")
	databaseName := c.Query("database_name")
	if instanceIDStr == "" || databaseName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "instance_id 和 database_name 参数不能为空"))
		return
	}

	var instanceID uint
	if _, err := fmt.Sscanf(instanceIDStr, "%d", &instanceID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的实例ID"))
		return
	}

	tables, err := h.queryService.GetTables(instanceID, databaseName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(tables))
}
