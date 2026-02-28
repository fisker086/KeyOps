package ticket

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/model"
)

// TicketDraftHandler 工单草稿处理器
type TicketDraftHandler struct {
	db *gorm.DB
}

// NewTicketDraftHandler 创建工单草稿处理器
func NewTicketDraftHandler(db *gorm.DB) *TicketDraftHandler {
	return &TicketDraftHandler{
		db: db,
	}
}

// ListDrafts 获取草稿列表
func (h *TicketDraftHandler) ListDrafts(c *gin.Context) {
	var tickets []model.Ticket
	query := h.db.Model(&model.Ticket{}).Where("status = ?", "draft")

	// 只获取当前用户的草稿
	if userID, exists := c.Get("user_id"); exists {
		query = query.Where("applicant_id = ?", userID)
	}

	// 是否共享草稿（暂时不支持，保留接口）
	share := c.Query("share")
	if share == "true" {
		// 未来可以实现共享草稿逻辑
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("updated_at DESC").Find(&tickets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取草稿列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tickets,
		"total":   total,
	})
}

// SaveDraft 保存草稿
func (h *TicketDraftHandler) SaveDraft(c *gin.Context) {
	var ticket model.Ticket
	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 强制设置为草稿状态
	ticket.Status = "draft"

	// 生成工单编号（如果不存在）
	if ticket.TicketNumber == "" {
		ticket.TicketNumber = fmt.Sprintf("TKT-%s", time.Now().Format("20060102150405")+uuid.New().String()[:8])
	}

	// 设置默认值
	if ticket.Priority == "" {
		ticket.Priority = "normal"
	}
	// 设置工单类型：发布工单的草稿默认为 deployment
	if ticket.Type == "" {
		ticket.Type = "deployment"
	}

	// 从上下文获取当前用户信息
	if userID, exists := c.Get("user_id"); exists {
		if ticket.ApplicantID == "" {
			ticket.ApplicantID = userID.(string)
		}
	}
	if userName, exists := c.Get("username"); exists {
		if ticket.ApplicantName == "" {
			ticket.ApplicantName = userName.(string)
		}
	}

	// 如果提供了 ID，则更新；否则创建
	if ticket.ID > 0 {
		var existingTicket model.Ticket
		if err := h.db.First(&existingTicket, ticket.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"code":    404,
					"message": "草稿不存在",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "获取草稿失败",
				"error":   err.Error(),
			})
			return
		}

		// 更新字段
		if ticket.Title != "" {
			existingTicket.Title = ticket.Title
		}
		if ticket.FormData != nil {
			existingTicket.FormData = ticket.FormData
		}
		if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
			existingTicket.TemplateID = ticket.TemplateID
		}
		if ticket.Priority != "" {
			existingTicket.Priority = ticket.Priority
		}
		// 更新工单类型（如果提供）
		if ticket.Type != "" {
			existingTicket.Type = ticket.Type
		}

		if err := h.db.Save(&existingTicket).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "更新草稿失败",
				"error":   err.Error(),
			})
			return
		}

		// 如果 template_id 不为空，预加载模板
		if existingTicket.TemplateID != nil && *existingTicket.TemplateID > 0 {
			h.db.Preload("Template").First(&existingTicket, existingTicket.ID)
		} else {
			h.db.First(&existingTicket, existingTicket.ID)
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    existingTicket,
		})
		return
	}

	// 创建新草稿
	if err := h.db.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建草稿失败",
			"error":   err.Error(),
		})
		return
	}

	// 重新加载以获取关联数据（如果 template_id 不为空）
	if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
		h.db.Preload("Template").First(&ticket, ticket.ID)
	} else {
		h.db.First(&ticket, ticket.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ticket,
	})
}

// UpdateDraft 更新草稿
func (h *TicketDraftHandler) UpdateDraft(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "草稿不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取草稿失败",
			"error":   err.Error(),
		})
		return
	}

	// 只能更新草稿状态的工单
	if ticket.Status != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "只能更新草稿状态的工单",
		})
		return
	}

	var updateData model.Ticket
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	if updateData.Title != "" {
		ticket.Title = updateData.Title
	}
	if updateData.FormData != nil {
		ticket.FormData = updateData.FormData
	}
	if updateData.TemplateID != nil && *updateData.TemplateID > 0 {
		ticket.TemplateID = updateData.TemplateID
	}
	if updateData.Priority != "" {
		ticket.Priority = updateData.Priority
	}

		if err := h.db.Save(&ticket).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "更新草稿失败",
				"error":   err.Error(),
			})
			return
		}

		// 如果 template_id 不为空，预加载模板
		if ticket.TemplateID != nil && *ticket.TemplateID > 0 {
			h.db.Preload("Template").First(&ticket, ticket.ID)
		} else {
			h.db.First(&ticket, ticket.ID)
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    ticket,
		})
	}

// DeleteDraft 删除草稿
func (h *TicketDraftHandler) DeleteDraft(c *gin.Context) {
	id := c.Param("id")

	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "草稿不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取草稿失败",
			"error":   err.Error(),
		})
		return
	}

	// 只能删除草稿状态的工单
	if ticket.Status != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "只能删除草稿状态的工单",
		})
		return
	}

	if err := h.db.Delete(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除草稿失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
