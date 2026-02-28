package alert

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	alertService "github.com/fisker086/keyops/internal/alert/service"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
)

// AlertHandler 告警处理器
type AlertHandler struct {
	service             *alertService.AlertService
	notificationManager *notification.NotificationManager
	onCallService       interface{} // 避免循环依赖，使用 interface{}
	domainCertRepo      interface{} // 域名证书仓库
	sslCertRepo         interface{} // SSL证书仓库
	hostedCertRepo      interface{} // 托管证书仓库
	certificateAlertService interface{} // 证书告警服务（避免循环依赖，使用 interface{}）
}

func NewAlertHandler(service *alertService.AlertService, notificationManager *notification.NotificationManager) *AlertHandler {
	return &AlertHandler{
		service:             service,
		notificationManager: notificationManager,
	}
}

func (h *AlertHandler) SetCertificateRepositories(domainCertRepo, sslCertRepo, hostedCertRepo interface{}) {
	h.domainCertRepo = domainCertRepo
	h.sslCertRepo = sslCertRepo
	h.hostedCertRepo = hostedCertRepo
}

func (h *AlertHandler) SetOnCallService(onCallService interface{}) {
	h.onCallService = onCallService
}

func (h *AlertHandler) SetCertificateAlertService(certificateAlertService interface{}) {
	h.certificateAlertService = certificateAlertService
}

// ==================== 告警规则数据源 ====================

// GetRuleSources 获取数据源列表
func (h *AlertHandler) GetRuleSources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, sources, err := h.service.GetRuleSources(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    sources,
		"total":   total,
	})
}

// GetRuleSource 获取数据源详情
func (h *AlertHandler) GetRuleSource(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	source, err := h.service.GetRuleSource(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "数据源不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(source))
}

// CreateRuleSource 创建数据源
func (h *AlertHandler) CreateRuleSource(c *gin.Context) {
	var req model.AlertRuleSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 验证 API Key 必填
	if req.APIKey == "" || len(req.APIKey) == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "API Key is required"))
		return
	}

	source, err := h.service.CreateRuleSource(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(source))
}

// UpdateRuleSource 更新数据源
func (h *AlertHandler) UpdateRuleSource(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertRuleSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 验证 API Key 必填
	if req.APIKey == "" || len(req.APIKey) == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "API Key is required"))
		return
	}

	source, err := h.service.UpdateRuleSource(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(source))
}

// DeleteRuleSource 删除数据源
func (h *AlertHandler) DeleteRuleSource(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteRuleSource(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// SyncRulesFromDatasource 从数据源同步规则
func (h *AlertHandler) SyncRulesFromDatasource(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.SyncRulesFromDatasource(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "规则同步成功",
	}))
}

// GetRuleSourcesByDepartment 根据部门获取数据源列表（已简化：不再区分部门，返回所有数据源）
func (h *AlertHandler) GetRuleSourcesByDepartment(c *gin.Context) {
	// 不再区分部门，返回所有数据源
	sources, err := h.service.GetRuleSourcesByDepartment(nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(sources))
}

// GetRuleSourcesByGroup 根据规则组获取数据源列表
func (h *AlertHandler) GetRuleSourcesByGroup(c *gin.Context) {
	groupIDStr := c.Query("group_id")
	var groupID *uint
	if groupIDStr != "" {
		id, err := strconv.ParseUint(groupIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "无效的规则组ID"))
			return
		}
		groupIDUint := uint(id)
		groupID = &groupIDUint
	}
	sources, err := h.service.GetRuleSourcesByGroup(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(sources))
}

// ==================== 规则组管理 ====================

// GetRuleGroups 获取规则组列表（支持 department_id 或 source_id 筛选；source_id 时支持 page、page_size 分页）
func (h *AlertHandler) GetRuleGroups(c *gin.Context) {
	sourceIDStr := c.Query("source_id")
	if sourceIDStr != "" {
		sourceID, err := strconv.ParseUint(sourceIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "无效的 source_id"))
			return
		}
		page, _ := strconv.Atoi(c.Query("page"))
		pageSize, _ := strconv.Atoi(c.Query("page_size"))
		// 仅当显式传了 page 和 page_size 时使用分页，否则返回全部（供下拉框等使用）
		if page >= 1 && pageSize >= 1 {
			if pageSize > 100 {
				pageSize = 100
			}
			total, groups, err := h.service.GetRuleGroupsBySourceIDWithPagination(uint(sourceID), page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": groups, "total": total})
			return
		}
		groups, err := h.service.GetRuleGroupsBySourceID(uint(sourceID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
			return
		}
		c.JSON(http.StatusOK, model.Success(groups))
		return
	}
	departmentIDStr := c.Query("department_id")
	var departmentID *string
	if departmentIDStr != "" {
		departmentID = &departmentIDStr
	}
	groups, err := h.service.GetRuleGroups(departmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(groups))
}

// GetRuleGroup 获取规则组详情
func (h *AlertHandler) GetRuleGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	group, err := h.service.GetRuleGroup(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "规则组不存在"))
		return
	}
	c.JSON(http.StatusOK, model.Success(group))
}

// CreateRuleGroup 创建规则组
func (h *AlertHandler) CreateRuleGroup(c *gin.Context) {
	var req model.AlertRuleGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	group, err := h.service.CreateRuleGroup(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(group))
}

// UpdateRuleGroup 更新规则组
func (h *AlertHandler) UpdateRuleGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertRuleGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	group, err := h.service.UpdateRuleGroup(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(group))
}

// DeleteRuleGroup 删除规则组
func (h *AlertHandler) DeleteRuleGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteRuleGroup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警规则 ====================

// GetRules 获取告警规则列表
func (h *AlertHandler) GetRules(c *gin.Context) {
	sourceID, _ := strconv.ParseUint(c.Query("source_id"), 10, 32)
	groupIDStr := c.Query("group_id")
	var groupID *uint
	if groupIDStr != "" {
		id, _ := strconv.ParseUint(groupIDStr, 10, 32)
		if id > 0 {
			uid := uint(id)
			groupID = &uid
		}
	}
	excludeGroupIDStr := c.Query("exclude_group_id")
	var excludeGroupID *uint
	if excludeGroupIDStr != "" {
		id, _ := strconv.ParseUint(excludeGroupIDStr, 10, 32)
		if id > 0 {
			uid := uint(id)
			excludeGroupID = &uid
		}
	}
	group := c.Query("group")
	name := c.Query("name")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, rules, err := h.service.GetRules(uint(sourceID), groupID, excludeGroupID, group, name, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
		"total":   total,
	})
}

// GetRule 获取告警规则详情
func (h *AlertHandler) GetRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	rule, err := h.service.GetRule(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警规则不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(rule))
}

// CreateRule 创建告警规则
func (h *AlertHandler) CreateRule(c *gin.Context) {
	var req model.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	rule, err := h.service.CreateRule(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(rule))
}

// UpdateRule 更新告警规则
func (h *AlertHandler) UpdateRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	rule, err := h.service.UpdateRule(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(rule))
}

// ReloadDatasource 重新加载数据源配置（触发 Prometheus reload）
func (h *AlertHandler) ReloadDatasource(c *gin.Context) {
	sourceID, _ := strconv.ParseUint(c.Param("source_id"), 10, 32)
	if sourceID == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "数据源ID不能为空"))
		return
	}

	if err := h.service.ReloadDatasource(uint(sourceID)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// DeleteRule 删除告警规则
func (h *AlertHandler) DeleteRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// MoveRuleToDefaultGroup 将规则移出当前规则组，归入该数据源的默认规则组
func (h *AlertHandler) MoveRuleToDefaultGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "规则ID不能为空"))
		return
	}
	if err := h.service.MoveRuleToDefaultGroup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// ToggleRule 启用/禁用告警规则
func (h *AlertHandler) ToggleRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.ToggleRule(uint(id), req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警事件 ====================

// GetEvents 获取告警事件列表
func (h *AlertHandler) GetEvents(c *gin.Context) {
	departmentID := c.Query("department_id")
	level, _ := strconv.Atoi(c.Query("level"))
	progress, _ := strconv.Atoi(c.Query("progress"))
	title := c.Query("alert_title")
	timeRange := c.DefaultQuery("time_range", "24h")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, events, err := h.service.GetEvents(departmentID, level, progress, title, timeRange, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    events,
		"total":   total,
	})
}

// GetEvent 获取告警事件详情
func (h *AlertHandler) GetEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	event, err := h.service.GetEvent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警事件不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(event))
}

// ClaimEvent 认领告警
func (h *AlertHandler) ClaimEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
		return
	}
	uidStr, ok := uid.(string)
	if !ok {
		uidStr = fmt.Sprintf("%v", uid)
	}

	if err := h.service.ClaimEvent(id, uidStr); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// CancelClaimEvent 取消认领
func (h *AlertHandler) CancelClaimEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
		return
	}
	uidStr, ok := uid.(string)
	if !ok {
		uidStr = fmt.Sprintf("%v", uid)
	}

	if err := h.service.CancelClaimEvent(id, uidStr); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// CloseEvent 关闭告警
func (h *AlertHandler) CloseEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
		return
	}
	uidStr, ok := uid.(string)
	if !ok {
		uidStr = fmt.Sprintf("%v", uid)
	}

	if err := h.service.CloseEvent(id, uidStr); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// OpenEvent 打开告警
func (h *AlertHandler) OpenEvent(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
		return
	}
	uidStr, ok := uid.(string)
	if !ok {
		uidStr = fmt.Sprintf("%v", uid)
	}

	if err := h.service.OpenEvent(id, uidStr); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}


// WebhookPrometheus 接收Prometheus告警Webhook
// 必须使用API Key认证，支持以下方式：
// 1. Authorization: Bearer <api_key> (推荐)
// 2. X-API-Key: <api_key>
// 3. 查询参数: ?api_key=<api_key>
// 通过API Key自动识别数据源，无需提供source_id
func (h *AlertHandler) WebhookPrometheus(c *gin.Context) {
	var alert model.PrometheusAlert
	if err := c.ShouldBindJSON(&alert); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "Invalid request body: "+err.Error()))
		return
	}

	// 1. 优先从 Authorization Header 获取 (Bearer Token)
	apiKey := ""
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// 支持 "Bearer <token>" 格式
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			apiKey = authHeader[7:]
		} else {
			// 也支持直接传递token
			apiKey = authHeader
		}
	}

	// 2. 如果Authorization中没有，尝试从X-API-Key Header获取
	if apiKey == "" {
		apiKey = c.GetHeader("X-API-Key")
	}

	// 3. 如果Header中都没有，尝试从查询参数获取
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	// 必须提供API Key
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, model.Error(401, "Missing API key: provide api_key via Authorization header (Bearer <api_key>), X-API-Key header, or api_key query parameter"))
		return
	}

	// 验证API Key并获取数据源ID
	source, err := h.service.GetRuleSourceByAPIKey(apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Error(401, "Invalid API key"))
		return
	}

	sourceIP := c.ClientIP()
	if err = h.service.ProcessPrometheusAlert(&alert, source.ID, sourceIP, h.notificationManager); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警策略 ====================

// GetStrategies 获取告警策略列表
func (h *AlertHandler) GetStrategies(c *gin.Context) {
	departmentID := c.Query("department_id")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, strategies, err := h.service.GetStrategies(departmentID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    strategies,
		"total":   total,
	})
}

// GetStrategy 获取告警策略详情
func (h *AlertHandler) GetStrategy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	strategy, err := h.service.GetStrategy(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警策略不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(strategy))
}

// CreateStrategy 创建告警策略
func (h *AlertHandler) CreateStrategy(c *gin.Context) {
	var req model.AlertStrategy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 调试日志：打印接收到的数据
	log.Printf("[AlertHandler] CreateStrategy received: StrategyName=%s, Filters=%s, StrategySet=%s", 
		req.StrategyName, string(req.Filters), string(req.StrategySet))

	strategy, err := h.service.CreateStrategy(&req)
	if err != nil {
		log.Printf("[AlertHandler] CreateStrategy error: %v", err)
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	log.Printf("[AlertHandler] CreateStrategy success: ID=%d, Filters=%s, StrategySet=%s", 
		strategy.ID, string(strategy.Filters), string(strategy.StrategySet))

	c.JSON(http.StatusOK, model.Success(strategy))
}

// UpdateStrategy 更新告警策略
func (h *AlertHandler) UpdateStrategy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertStrategy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	strategy, err := h.service.UpdateStrategy(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(strategy))
}

// DeleteStrategy 删除告警策略
func (h *AlertHandler) DeleteStrategy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteStrategy(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ToggleStrategy 启用/禁用告警策略
func (h *AlertHandler) ToggleStrategy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.ToggleStrategy(uint(id), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警等级 ====================

// GetLevels 获取告警等级列表
func (h *AlertHandler) GetLevels(c *gin.Context) {
	levels, err := h.service.GetLevels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(levels))
}

// ==================== 告警静默 ====================

// GetSilences 获取告警静默列表
func (h *AlertHandler) GetSilences(c *gin.Context) {
	departmentID := c.Query("department_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, silences, err := h.service.GetSilences(departmentID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    silences,
		"total":   total,
	})
}

// GetSilence 获取告警静默详情
func (h *AlertHandler) GetSilence(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	silence, err := h.service.GetSilence(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警静默不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(silence))
}

// CreateSilence 创建告警静默
func (h *AlertHandler) CreateSilence(c *gin.Context) {
	var req model.AlertSilence
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	silence, err := h.service.CreateSilence(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(silence))
}

// UpdateSilence 更新告警静默
func (h *AlertHandler) UpdateSilence(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertSilence
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	silence, err := h.service.UpdateSilence(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(silence))
}

// DeleteSilence 删除告警静默
func (h *AlertHandler) DeleteSilence(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteSilence(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警聚合 ====================

// GetAggregations 获取告警聚合列表
func (h *AlertHandler) GetAggregations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, aggs, err := h.service.GetAggregations(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    aggs,
		"total":   total,
	})
}

// GetAggregation 获取告警聚合详情
func (h *AlertHandler) GetAggregation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	agg, err := h.service.GetAggregation(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警聚合不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(agg))
}

// CreateAggregation 创建告警聚合
func (h *AlertHandler) CreateAggregation(c *gin.Context) {
	var req model.AlertAggregation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	agg, err := h.service.CreateAggregation(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(agg))
}

// UpdateAggregation 更新告警聚合
func (h *AlertHandler) UpdateAggregation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertAggregation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	agg, err := h.service.UpdateAggregation(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(agg))
}

// DeleteAggregation 删除告警聚合
func (h *AlertHandler) DeleteAggregation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteAggregation(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警抑制 ====================

// GetRestrains 获取告警抑制列表
func (h *AlertHandler) GetRestrains(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, restrains, err := h.service.GetRestrains(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    restrains,
		"total":   total,
	})
}

// GetRestrain 获取告警抑制详情
func (h *AlertHandler) GetRestrain(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	restrain, err := h.service.GetRestrain(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警抑制不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(restrain))
}

// CreateRestrain 创建告警抑制
func (h *AlertHandler) CreateRestrain(c *gin.Context) {
	var req model.AlertRestrain
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	restrain, err := h.service.CreateRestrain(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(restrain))
}

// UpdateRestrain 更新告警抑制
func (h *AlertHandler) UpdateRestrain(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertRestrain
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	restrain, err := h.service.UpdateRestrain(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(restrain))
}

// DeleteRestrain 删除告警抑制
func (h *AlertHandler) DeleteRestrain(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteRestrain(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警模板 ====================

// GetTemplates 获取告警模板列表
func (h *AlertHandler) GetTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, templates, err := h.service.GetTemplates(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
		"total":   total,
	})
}

// GetTemplate 获取告警模板详情
func (h *AlertHandler) GetTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	template, err := h.service.GetTemplate(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警模板不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(template))
}

// CreateTemplate 创建告警模板
func (h *AlertHandler) CreateTemplate(c *gin.Context) {
	var req model.AlertTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	template, err := h.service.CreateTemplate(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(template))
}

// UpdateTemplate 更新告警模板
func (h *AlertHandler) UpdateTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	template, err := h.service.UpdateTemplate(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(template))
}

// DeleteTemplate 删除告警模板
func (h *AlertHandler) DeleteTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteTemplate(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetChannelTemplates 获取模板的所有渠道模板内容
func (h *AlertHandler) GetChannelTemplates(c *gin.Context) {
	templateID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	channelTemplates, err := h.service.GetChannelTemplates(uint(templateID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(channelTemplates))
}

// UpdateChannelTemplate 更新或创建渠道模板内容
func (h *AlertHandler) UpdateChannelTemplate(c *gin.Context) {
	templateID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	channelID, _ := strconv.ParseUint(c.Param("channelId"), 10, 32)

	var req struct {
		Content  string `json:"content"`
		Finished bool   `json:"finished"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	channelTemplate, err := h.service.UpdateChannelTemplate(uint(templateID), uint(channelID), req.Content, req.Finished)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(channelTemplate))
}

// DeleteChannelTemplate 删除渠道模板内容
func (h *AlertHandler) DeleteChannelTemplate(c *gin.Context) {
	templateID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	channelID, _ := strconv.ParseUint(c.Param("channelId"), 10, 32)

	if err := h.service.DeleteChannelTemplate(uint(templateID), uint(channelID)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警渠道 ====================

// GetChannels 获取告警渠道列表
func (h *AlertHandler) GetChannels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, channels, err := h.service.GetChannels(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    channels,
		"total":   total,
	})
}

// GetChannel 获取告警渠道详情
func (h *AlertHandler) GetChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	channel, err := h.service.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警渠道不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(channel))
}

// CreateChannel 创建告警渠道
func (h *AlertHandler) CreateChannel(c *gin.Context) {
	var req model.AlertChannel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	channel, err := h.service.CreateChannel(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(channel))
}

// UpdateChannel 更新告警渠道
func (h *AlertHandler) UpdateChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertChannel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	channel, err := h.service.UpdateChannel(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(channel))
}

// DeleteChannel 删除告警渠道
func (h *AlertHandler) DeleteChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteChannel(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 策略日志 ====================

// StrategyLogResponse 策略日志响应（包含策略名称）
type StrategyLogResponse struct {
	model.StrategyLog
	StrategyName string `json:"strategy_name,omitempty"` // 策略名称
}

// GetStrategyLogs 获取策略日志列表
func (h *AlertHandler) GetStrategyLogs(c *gin.Context) {
	alertIDStr := c.Query("alert_id")
	alertID, _ := strconv.ParseUint(alertIDStr, 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	log.Printf("[AlertHandler] GetStrategyLogs: alert_id=%s, parsed=%d, page=%d, pageSize=%d", alertIDStr, alertID, page, pageSize)

	// alert_id 是可选的，如果不传则返回所有日志
	total, logs, err := h.service.GetStrategyLogs(alertID, page, pageSize)
	if err != nil {
		log.Printf("[AlertHandler] GetStrategyLogs error: alertID=%d, error=%v", alertID, err)
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	log.Printf("[AlertHandler] GetStrategyLogs result: alertID=%d, total=%d, logs count=%d", alertID, total, len(logs))

	// 获取策略名称映射（所有策略都是自定义策略）
	strategyMap := make(map[uint]string)
	for _, log := range logs {
		if log.StrategyID > 0 {
			if strategy, err := h.service.GetStrategy(log.StrategyID); err == nil {
				strategyMap[log.StrategyID] = strategy.StrategyName
			}
		}
	}

	// 构建响应数据
	responseData := make([]StrategyLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = StrategyLogResponse{
			StrategyLog:  log,
			StrategyName: strategyMap[log.StrategyID],
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    responseData,
		"total":   total,
	})
}

// GetStrategyLog 获取策略日志详情
func (h *AlertHandler) GetStrategyLog(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, err := h.service.GetStrategyLog(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "策略日志不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(log))
}

// ==================== 告警组 ====================

// GetAlertGroups 获取告警组列表
func (h *AlertHandler) GetAlertGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	departmentID := c.Query("department_id")

	var deptID *string
	if departmentID != "" {
		deptID = &departmentID
	}

	total, groups, err := h.service.GetAlertGroups(deptID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    groups,
		"total":   total,
	})
}

// GetAllAlertGroups 获取所有告警组（不分页，用于下拉选择）
func (h *AlertHandler) GetAllAlertGroups(c *gin.Context) {
	departmentID := c.Query("department_id")

	var deptID *string
	if departmentID != "" {
		deptID = &departmentID
	}

	groups, err := h.service.GetAllAlertGroups(deptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(groups))
}

// GetAlertGroup 获取告警组详情
func (h *AlertHandler) GetAlertGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	group, err := h.service.GetAlertGroup(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "告警组不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(group))
}

// CreateAlertGroup 创建告警组
func (h *AlertHandler) CreateAlertGroup(c *gin.Context) {
	var req model.AlertGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	group, err := h.service.CreateAlertGroup(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(group))
}

// UpdateAlertGroup 更新告警组
func (h *AlertHandler) UpdateAlertGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.AlertGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	group, err := h.service.UpdateAlertGroup(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(group))
}

// DeleteAlertGroup 删除告警组
func (h *AlertHandler) DeleteAlertGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteAlertGroup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== 告警统计接口 ====================

// GetStatistics 获取告警统计信息
func (h *AlertHandler) GetStatistics(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	
	stats, err := h.service.GetStatistics(timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(stats))
}

// GetTrendStatistics 获取告警趋势统计
func (h *AlertHandler) GetTrendStatistics(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	
	trends, err := h.service.GetTrendStatistics(timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(trends))
}

// GetTopAlerts 获取Top N告警
func (h *AlertHandler) GetTopAlerts(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	
	topAlerts, err := h.service.GetTopAlerts(timeRange, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(topAlerts))
}

// ==================== 证书管理 ====================

// GetDomainCertificates 获取域名证书列表
func (h *AlertHandler) GetDomainCertificates(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.DefaultQuery("keyword", "")
	
	total, certs, err := repo.List(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    certs,
		"total":   total,
	})
}

// GetDomainCertificate 获取域名证书详情
func (h *AlertHandler) GetDomainCertificate(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	cert, err := repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "证书不存在"))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(cert))
}

// CreateDomainCertificate 创建域名证书
func (h *AlertHandler) CreateDomainCertificate(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	var req model.DomainCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	if err := repo.Create(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	cert, _ := repo.FindByID(req.ID)
	c.JSON(http.StatusOK, model.Success(cert))
}

// UpdateDomainCertificate 更新域名证书
func (h *AlertHandler) UpdateDomainCertificate(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.DomainCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	req.ID = uint(id)
	if err := repo.Update(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	cert, _ := repo.FindByID(uint(id))
	c.JSON(http.StatusOK, model.Success(cert))
}

// DeleteDomainCertificate 删除域名证书
func (h *AlertHandler) DeleteDomainCertificate(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(nil))
}

// RefreshDomainCertificate 刷新域名证书信息（通过HTTPS连接获取证书信息）
func (h *AlertHandler) RefreshDomainCertificate(c *gin.Context) {
	if h.domainCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.domainCertRepo.(*repository.DomainCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	cert, err := repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "证书不存在"))
		return
	}
	
	// 使用 crypto/tls 连接域名并获取证书信息
	if err := h.fetchCertificateInfo(cert); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("获取证书信息失败: %v", err)))
		return
	}
	
	// 更新证书信息
	if err := repo.Update(cert); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	// 返回更新后的证书信息
	updatedCert, _ := repo.FindByID(uint(id))
	c.JSON(http.StatusOK, model.Success(updatedCert))
}

// CheckCertificateAlerts 手动触发证书告警检查
func (h *AlertHandler) CheckCertificateAlerts(c *gin.Context) {
	if h.certificateAlertService == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书告警服务未初始化"))
		return
	}

	// 使用类型断言获取证书告警服务
	type CertificateAlertServiceInterface interface {
		CheckAndSendAlerts() error
	}
	
	service, ok := h.certificateAlertService.(CertificateAlertServiceInterface)
	if !ok {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书告警服务类型错误"))
		return
	}

	// 执行检查
	if err := service.CheckAndSendAlerts(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("证书告警检查失败: %v", err)))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "证书告警检查已执行",
	}))
}

// fetchCertificateInfo 通过HTTPS连接获取证书信息
func (h *AlertHandler) fetchCertificateInfo(cert *model.DomainCertificate) error {
	address := fmt.Sprintf("%s:%d", cert.Domain, cert.Port)
	if cert.Port == 0 {
		address = fmt.Sprintf("%s:443", cert.Domain)
		cert.Port = 443
	}
	
	// 建立TCP连接
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		cert.ConnectStatus = boolPtr(false)
		return fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()
	
	// 创建TLS连接
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         cert.Domain,
		InsecureSkipVerify: true, // 允许自签名证书
	})
	defer tlsConn.Close()
	
	// 握手获取证书
	if err := tlsConn.Handshake(); err != nil {
		cert.ConnectStatus = boolPtr(false)
		return fmt.Errorf("TLS握手失败: %w", err)
	}
	
	// 获取证书链
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		cert.ConnectStatus = boolPtr(false)
		return fmt.Errorf("未获取到证书")
	}
	
	// 获取第一个证书（服务器证书）
	serverCert := state.PeerCertificates[0]
	
	// 解析证书信息
	cert.ConnectStatus = boolPtr(true)
	cert.StartTime = &serverCert.NotBefore
	cert.ExpireTime = &serverCert.NotAfter
	
	// 计算剩余天数
	if cert.ExpireTime != nil {
		days := int(time.Until(*cert.ExpireTime).Hours() / 24)
		cert.ExpireDays = days
	}
	
	// 将证书转换为PEM格式
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCert.Raw,
	})
	cert.SSLCertificate = string(certPEM)
	
	// 私钥不在这里获取（需要服务器权限）
	cert.SSLCertificateKey = ""
	
	return nil
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}

// GetSslCertificates 获取SSL证书列表
func (h *AlertHandler) GetSslCertificates(c *gin.Context) {
	if h.sslCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.sslCertRepo.(*repository.SSLCertificateRepository)
	
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.DefaultQuery("keyword", "")
	
	total, certs, err := repo.List(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    certs,
		"total":   total,
	})
}

// GetSslCertificate 获取SSL证书详情
func (h *AlertHandler) GetSslCertificate(c *gin.Context) {
	if h.sslCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.sslCertRepo.(*repository.SSLCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	cert, err := repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "证书不存在"))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(cert))
}

// CreateSslCertificate 创建SSL证书
func (h *AlertHandler) CreateSslCertificate(c *gin.Context) {
	if h.sslCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.sslCertRepo.(*repository.SSLCertificateRepository)
	
	var req model.SSLCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	if err := repo.Create(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(req))
}

// UpdateSslCertificate 更新SSL证书
func (h *AlertHandler) UpdateSslCertificate(c *gin.Context) {
	if h.sslCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.sslCertRepo.(*repository.SSLCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.SSLCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	req.ID = uint(id)
	if err := repo.Update(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	cert, _ := repo.FindByID(uint(id))
	c.JSON(http.StatusOK, model.Success(cert))
}

// DeleteSslCertificate 删除SSL证书
func (h *AlertHandler) DeleteSslCertificate(c *gin.Context) {
	if h.sslCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.sslCertRepo.(*repository.SSLCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(nil))
}

// GetHostedCertificates 获取托管证书列表
func (h *AlertHandler) GetHostedCertificates(c *gin.Context) {
	if h.hostedCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.hostedCertRepo.(*repository.HostedCertificateRepository)
	
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.DefaultQuery("keyword", "")
	
	total, certs, err := repo.List(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    certs,
		"total":   total,
	})
}

// GetHostedCertificate 获取托管证书详情
func (h *AlertHandler) GetHostedCertificate(c *gin.Context) {
	if h.hostedCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.hostedCertRepo.(*repository.HostedCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	cert, err := repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "证书不存在"))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(cert))
}

// CreateHostedCertificate 创建托管证书
func (h *AlertHandler) CreateHostedCertificate(c *gin.Context) {
	if h.hostedCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.hostedCertRepo.(*repository.HostedCertificateRepository)
	
	var req model.HostedCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	if err := repo.Create(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(req))
}

// UpdateHostedCertificate 更新托管证书
func (h *AlertHandler) UpdateHostedCertificate(c *gin.Context) {
	if h.hostedCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.hostedCertRepo.(*repository.HostedCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.HostedCertificate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	
	req.ID = uint(id)
	if err := repo.Update(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	cert, _ := repo.FindByID(uint(id))
	c.JSON(http.StatusOK, model.Success(cert))
}

// DeleteHostedCertificate 删除托管证书
func (h *AlertHandler) DeleteHostedCertificate(c *gin.Context) {
	if h.hostedCertRepo == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "证书仓库未初始化"))
		return
	}
	repo := h.hostedCertRepo.(*repository.HostedCertificateRepository)
	
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, model.Success(nil))
}
