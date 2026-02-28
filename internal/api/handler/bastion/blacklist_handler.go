package bastion

import (
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BlacklistHandler 黑名单处理器
type BlacklistHandler struct {
	db *gorm.DB
}

// NewBlacklistHandler 创建黑名单处理器
func NewBlacklistHandler(db *gorm.DB) *BlacklistHandler {
	return &BlacklistHandler{db: db}
}

// GetCommands 获取黑名单命令列表
func (h *BlacklistHandler) GetCommands(c *gin.Context) {
	var rules []model.BlacklistRule

	if err := h.db.Order("created_at DESC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取黑名单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"rules": rules,
		},
	})
}

// CreateCommand 创建黑名单规则
func (h *BlacklistHandler) CreateCommand(c *gin.Context) {
	var req model.BlacklistRule

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 生成ID和时间戳
	req.ID = uuid.New().String()
	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now

	// 默认启用
	if !req.Enabled {
		req.Enabled = true
	}

	// 保存到数据库
	if err := h.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建黑名单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    req,
	})
}

// UpdateCommand 更新黑名单规则
func (h *BlacklistHandler) UpdateCommand(c *gin.Context) {
	id := c.Param("id")

	var req model.BlacklistRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新时间戳
	req.UpdatedAt = time.Now()

	// 更新数据库
	if err := h.db.Model(&model.BlacklistRule{}).Where("id = ?", id).Updates(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新黑名单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// DeleteCommand 删除黑名单规则
func (h *BlacklistHandler) DeleteCommand(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.Where("id = ?", id).Delete(&model.BlacklistRule{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除黑名单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// GetActiveCommands 获取启用的黑名单命令（供 proxy 使用，不需要认证）
// 路由：GET /api/proxy/blacklist
// 返回完整的规则信息，包括作用范围和用户列表
func (h *BlacklistHandler) GetActiveCommands(c *gin.Context) {
	var rules []model.BlacklistRule

	if err := h.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取黑名单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"rules": rules,
		},
	})
}
