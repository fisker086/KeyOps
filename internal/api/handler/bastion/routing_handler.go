package bastion

import (
	"log"
	"net/http"

	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
)

// RoutingHandler 路由管理处理器（基于标签的路由配置）
type RoutingHandler struct {
	hostRepo repository.HostRepository
}

// NewRoutingHandler 创建路由处理器
func NewRoutingHandler(
	hostRepo repository.HostRepository,
) *RoutingHandler {
	return &RoutingHandler{
		hostRepo: hostRepo,
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
	decision, err := makeRoutingDecision(h.hostRepo, hostID, username)
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

// GetRoutingConfig 获取路由配置（纯直连）
// GET /api/routing/config
func (h *RoutingHandler) GetRoutingConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"mode": "direct",
		},
		"msg": "success",
	})
}

// UpdateRoutingConfig 更新路由配置
// PUT /api/routing/config
func (h *RoutingHandler) UpdateRoutingConfig(c *gin.Context) {
	log.Printf("[Routing] Pure direct mode enabled, proxy routing config ignored")

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "Pure direct mode: no routing config required",
	})
}
