package monitor

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	monitorService "github.com/fisker086/keyops/internal/service/monitor"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MonitorHandler struct {
	service *monitorService.MonitorService
}

func NewMonitorHandler(service *monitorService.MonitorService) *MonitorHandler {
	return &MonitorHandler{service: service}
}

// CreateMonitor 创建监控查询语句
// @Summary 创建监控查询语句
// @Description 创建 Prometheus 监控查询语句
// @Tags monitor
// @Accept json
// @Produce json
// @Param monitor body object true "监控信息" SchemaExample({"name":"CPU使用率","expr":"100 - (avg(irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)"})
// @Success 200 {object} model.Response
// @Router /api/monitors/prom [post]
func (h *MonitorHandler) CreateMonitor(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Expr string `json:"expr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前用户ID（由中间件设置）
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}
	userID := userIDInterface.(string)

	monitor, err := h.service.CreateMonitor(req.Name, req.Expr, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(monitor))
}

// GetMonitor 获取监控详情
// @Summary 获取监控详情
// @Description 根据ID获取监控查询语句详情
// @Tags monitor
// @Accept json
// @Produce json
// @Param id path int true "监控ID"
// @Success 200 {object} model.Response
// @Router /api/monitors/prom/:id [get]
func (h *MonitorHandler) GetMonitor(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的ID格式"))
		return
	}

	monitor, err := h.service.GetMonitor(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, model.Error(404, "监控不存在"))
		} else {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, model.Success(monitor))
}

// ListMonitors 获取监控列表
// @Summary 获取监控列表
// @Description 获取监控查询语句列表，支持分页和搜索
// @Tags monitor
// @Accept json
// @Produce json
// @Param name query string false "名称搜索（模糊匹配）"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} model.Response
// @Router /api/monitors/prom [get]
func (h *MonitorHandler) ListMonitors(c *gin.Context) {
	name := c.DefaultQuery("name", "")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	// 参数验证和默认值处理
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	total, monitors, err := h.service.ListMonitors(name, page, pageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result := map[string]interface{}{
		"total":    total,
		"monitors": monitors,
		"page":     page,
		"page_size": pageSize,
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// CountMonitors 统计监控数量
// @Summary 统计监控数量
// @Description 统计监控查询语句数量，用于分页
// @Tags monitor
// @Accept json
// @Produce json
// @Param name query string false "名称搜索（模糊匹配）"
// @Success 200 {object} model.Response
// @Router /api/monitors/prom/count [get]
func (h *MonitorHandler) CountMonitors(c *gin.Context) {
	name := c.DefaultQuery("name", "")

	total, err := h.service.CountMonitors(name)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(map[string]interface{}{
		"total": total,
	}))
}

// UpdateMonitor 更新监控查询语句
// @Summary 更新监控查询语句
// @Description 更新 Prometheus 监控查询表达式
// @Tags monitor
// @Accept json
// @Produce json
// @Param id path int true "监控ID"
// @Param monitor body object true "监控信息" SchemaExample({"expr":"100 - (avg(irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)"})
// @Success 200 {object} model.Response
// @Router /api/monitors/prom/:id [put]
func (h *MonitorHandler) UpdateMonitor(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的ID格式"))
		return
	}

	var req struct {
		Expr string `json:"expr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 获取当前用户ID（由中间件设置）
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未登录"))
		return
	}
	userID := userIDInterface.(string)

	monitor, err := h.service.UpdateMonitor(uint(id), req.Expr, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, model.Error(404, "监控不存在"))
		} else {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, model.Success(monitor))
}

// DeleteMonitor 删除监控查询语句
// @Summary 删除监控查询语句
// @Description 根据ID删除监控查询语句
// @Tags monitor
// @Accept json
// @Produce json
// @Param id path int true "监控ID"
// @Success 200 {object} model.Response
// @Router /api/monitors/prom/:id [delete]
func (h *MonitorHandler) DeleteMonitor(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的ID格式"))
		return
	}

	if err := h.service.DeleteMonitor(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, model.Error(404, "监控不存在"))
		} else {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetProbe 查询 Probe 监控数据
// @Summary 查询 Probe 监控数据
// @Description 查询监控探针数据
// @Tags monitor
// @Accept json
// @Produce json
// @Param group query string true "分组"
// @Param project query string true "项目"
// @Param env query string true "环境"
// @Param module query string false "模块"
// @Param address query string false "地址"
// @Success 200 {object} model.Response
// @Router /api/monitors/probe [get]
func (h *MonitorHandler) GetProbe(c *gin.Context) {
	group := c.DefaultQuery("group", "")
	project := c.DefaultQuery("project", "")
	env := c.DefaultQuery("env", "")
	module := c.DefaultQuery("module", "")
	address := c.DefaultQuery("address", "")

	// 参数验证
	if group == "" || project == "" || env == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "group、project、env 参数不能为空"))
		return
	}

	result, err := h.service.QueryProbe(group, project, env, module, address)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

