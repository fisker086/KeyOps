package bastion

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/routing"
	"github.com/gin-gonic/gin"
)

// RoutingHandler 路由管理处理器（基于标签的路由配置）
type RoutingHandler struct {
	router      *routing.ConnectionRouter
	settingRepo *repository.SettingRepository
	hostRepo    *repository.HostRepository
	proxyRepo   *repository.ProxyRepository
}

// NewRoutingHandler 创建路由处理器
func NewRoutingHandler(
	r *routing.ConnectionRouter,
	settingRepo *repository.SettingRepository,
	hostRepo *repository.HostRepository,
	proxyRepo *repository.ProxyRepository,
) *RoutingHandler {
	return &RoutingHandler{
		router:      r,
		settingRepo: settingRepo,
		hostRepo:    hostRepo,
		proxyRepo:   proxyRepo,
	}
}

// GetRoutingDecision 获取主机的路由决策（供前端查询）
// GET /api/hosts/:id/route
func (h *RoutingHandler) GetRoutingDecision(c *gin.Context) {
	hostID := c.Param("id")

	// 从上下文获取用户信息（由认证中间件设置）
	userID := c.GetString("userID")
	username := c.GetString("username")

	if userID == "" {
		userID = "system" // fallback
		username = "admin"
	}

	// 执行路由决策
	decision, err := h.router.MakeRoutingDecision(hostID, userID, username)
	if err != nil {
		log.Printf("[Routing] Decision failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": decision,
		"msg":  "success",
	})
}

// GetRoutingConfig 获取路由配置（基于标签）
// GET /api/routing/config
func (h *RoutingHandler) GetRoutingConfig(c *gin.Context) {
	// 获取需要代理的标签列表
	proxyTagsSetting, _ := h.settingRepo.GetSettingByKey("routing_proxy_tags")
	proxyTags := []string{}
	if proxyTagsSetting != nil && proxyTagsSetting.Value != "" {
		json.Unmarshal([]byte(proxyTagsSetting.Value), &proxyTags)
	}

	// 获取默认代理ID
	defaultProxySetting, _ := h.settingRepo.GetSettingByKey("routing_default_proxy_id")
	defaultProxyID := ""
	if defaultProxySetting != nil {
		defaultProxyID = defaultProxySetting.Value
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"proxyTags":      proxyTags,
			"defaultProxyID": defaultProxyID,
		},
		"msg": "success",
	})
}

// UpdateRoutingConfig 更新路由配置
// PUT /api/routing/config
func (h *RoutingHandler) UpdateRoutingConfig(c *gin.Context) {
	var req struct {
		ProxyTags      []string `json:"proxyTags"`
		DefaultProxyID string   `json:"defaultProxyID"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证代理ID是否有效
	if req.DefaultProxyID != "" {
		_, err := h.proxyRepo.FindProxyInfoByID(req.DefaultProxyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
			return
		}
	}

	// 保存标签列表
	tagsJSON, _ := json.Marshal(req.ProxyTags)
	h.settingRepo.SetSetting(&model.Setting{
		Key:   "routing_proxy_tags",
		Value: string(tagsJSON),
	})

	// 保存默认代理ID
	h.settingRepo.SetSetting(&model.Setting{
		Key:   "routing_default_proxy_id",
		Value: req.DefaultProxyID,
	})

	log.Printf("[Routing] Updated routing config: tags=%v, proxy=%s", req.ProxyTags, req.DefaultProxyID)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "Routing configuration updated successfully",
	})
}

// GetAvailableProxies 获取可用的代理列表
// GET /api/routing/proxies
func (h *RoutingHandler) GetAvailableProxies(c *gin.Context) {
	proxies, err := h.proxyRepo.FindOnlineProxies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": proxies,
		"msg":  "success",
	})
}

// Deprecated: 以下方法为兼容性保留，将来会移除
func (h *RoutingHandler) ListRoutingRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []interface{}{},
		"msg":  "Routing rules are deprecated, use tag-based routing instead",
	})
}

func (h *RoutingHandler) GetRoutingRule(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "Routing rules are deprecated"})
}

func (h *RoutingHandler) CreateRoutingRule(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Routing rules are deprecated, use tag-based routing instead"})
}

func (h *RoutingHandler) UpdateRoutingRule(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Routing rules are deprecated, use tag-based routing instead"})
}

func (h *RoutingHandler) DeleteRoutingRule(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Routing rules are deprecated, use tag-based routing instead"})
}

func (h *RoutingHandler) ToggleRoutingRule(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Routing rules are deprecated, use tag-based routing instead"})
}
