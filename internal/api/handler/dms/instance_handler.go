package dms

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service/dms"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

type InstanceHandler struct {
	instanceService *dms.InstanceService
}

func NewInstanceHandler(instanceService *dms.InstanceService) *InstanceHandler {
	return &InstanceHandler{
		instanceService: instanceService,
	}
}

// ListInstances 获取实例列表
// @Summary 获取数据库实例列表
// @Tags DMS
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param db_type query string false "数据库类型"
// @Param is_enabled query bool false "是否启用"
// @Param name query string false "实例名称（模糊搜索）"
// @Success 200 {object} model.Response{data=object{list=[]model.DBInstance,total=int64}}
// @Router /api/dms/instances [get]
func (h *InstanceHandler) ListInstances(c *gin.Context) {
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
	if dbType := c.Query("db_type"); dbType != "" {
		filters["db_type"] = dbType
	}
	if isEnabled := c.Query("is_enabled"); isEnabled != "" {
		filters["is_enabled"] = isEnabled == "true"
	}
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}

	instances, total, err := h.instanceService.ListInstances(offset, pageSize, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取实例列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"list":  instances,
		"total": total,
	}))
}

// CreateInstance 创建实例
// @Summary 创建数据库实例
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.CreateInstanceRequest true "实例信息"
// @Success 200 {object} model.Response{data=model.DBInstance}
// @Router /api/dms/instances [post]
func (h *InstanceHandler) CreateInstance(c *gin.Context) {
	var req dms.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	// 对于非 Redis 和 MongoDB 类型，密码是必填的
	if req.DBType != "redis" && req.DBType != "mongodb" && req.Password == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "密码是必填项（Redis 和 MongoDB 类型除外）"))
		return
	}

	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	instance, err := h.instanceService.CreateInstance(&req, userIDStr)
	if err != nil {
		if errors.Is(err, dms.ErrInstanceNameExists) {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, "创建实例失败: "+err.Error()))
		return
	}

	// 显式返回带 id 的 data，避免前端解析不到 id（如网关/代理或前端 request 解包导致）
	logger.Infof("DMS CreateInstance success, id=%d, name=%s", instance.ID, instance.Name)
	data := gin.H{
		"id":              instance.ID,
		"name":            instance.Name,
		"dbType":          instance.DBType,
		"host":            instance.Host,
		"port":            instance.Port,
		"username":        instance.Username,
		"databaseName":    instance.DatabaseName,
		"authDatabase":    instance.AuthDatabase,
		"charset":         instance.Charset,
		"connectionString": instance.ConnectionString,
		"sslEnabled":      instance.SSLEnabled,
		"sslCert":         instance.SSLCert,
		"description":     instance.Description,
		"isEnabled":       instance.IsEnabled,
		"createdBy":       instance.CreatedBy,
		"createdAt":       instance.CreatedAt,
		"updatedAt":       instance.UpdatedAt,
	}
	c.JSON(http.StatusOK, model.Success(data))
}

// GetInstance 获取实例详情
// @Summary 获取数据库实例详情
// @Tags DMS
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} model.Response{data=model.DBInstance}
// @Router /api/dms/instances/:id [get]
func (h *InstanceHandler) GetInstance(c *gin.Context) {
	var uriParams struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriParams); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	instance, err := h.instanceService.GetInstance(uriParams.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "实例不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(instance))
}

// UpdateInstance 更新实例
// @Summary 更新数据库实例
// @Tags DMS
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Param request body dms.UpdateInstanceRequest true "实例信息"
// @Success 200 {object} model.Response{data=model.DBInstance}
// @Router /api/dms/instances/:id [put]
func (h *InstanceHandler) UpdateInstance(c *gin.Context) {
	var uriParams struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriParams); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	var req dms.UpdateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	instance, err := h.instanceService.UpdateInstance(uriParams.ID, &req, userIDStr)
	if err != nil {
		if errors.Is(err, dms.ErrInstanceNameExists) {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新实例失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(instance))
}

// DeleteInstance 删除实例
// @Summary 删除数据库实例
// @Tags DMS
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} model.Response
// @Router /api/dms/instances/:id [delete]
func (h *InstanceHandler) DeleteInstance(c *gin.Context) {
	var uriParams struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriParams); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.instanceService.DeleteInstance(uriParams.ID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "删除实例失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// TestConnection 测试连接（已存在的实例）
// @Summary 测试数据库连接
// @Tags DMS
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} model.Response
// @Router /api/dms/instances/:id/test [post]
func (h *InstanceHandler) TestConnection(c *gin.Context) {
	var uriParams struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uriParams); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.instanceService.TestConnection(uriParams.ID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "连接测试失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "连接成功"}))
}

// TestConnectionWithBody 仅测试连接（不创建实例，用于新增前测试）
// @Summary 测试连接（请求体，不落库）
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.CreateInstanceRequest true "连接参数"
// @Success 200 {object} model.Response
// @Router /api/dms/instances/test-connection [post]
func (h *InstanceHandler) TestConnectionWithBody(c *gin.Context) {
	var req dms.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}
	// 便于排查：Docker 前端经 Nginx 代理 vs 直连 8080 时请求是否一致
	logger.Infof("DMS TestConnectionWithBody received: dbType=%s host=%s port=%d client=%s",
		req.DBType, req.Host, req.Port, c.ClientIP())
	if req.DBType != "redis" && req.DBType != "mongodb" && req.Password == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "密码是必填项（Redis 和 MongoDB 类型除外）"))
		return
	}
	if err := h.instanceService.TestConnectionWithRequest(&req); err != nil {
		logger.Infof("DMS TestConnectionWithBody failed: host=%s port=%d err=%v", req.Host, req.Port, err)
		c.JSON(http.StatusBadRequest, model.Error(400, "连接测试失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"message": "连接成功"}))
}
