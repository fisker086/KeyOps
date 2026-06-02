package ticket

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/logger"
)

// FormTemplateHandler 表单模板处理器
type FormTemplateHandler struct {
	db *gorm.DB
}

// ensureFormTemplateUUID 为历史数据补全 uuid
func ensureFormTemplateUUID(db *gorm.DB, template *model.FormTemplate) error {
	if strings.TrimSpace(template.UUID) != "" {
		return nil
	}
	template.UUID = uuid.New().String()
	return db.Model(template).Update("uuid", template.UUID).Error
}

// NewFormTemplateHandler 创建表单模板处理器
func NewFormTemplateHandler(db *gorm.DB) *FormTemplateHandler {
	return &FormTemplateHandler{
		db: db,
	}
}

// ListFormTemplates 获取表单模板列表
func (h *FormTemplateHandler) ListFormTemplates(c *gin.Context) {
	var templates []model.FormTemplate
	query := h.db.Model(&model.FormTemplate{})

	// 过滤条件
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
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
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板列表失败",
			"error":   err.Error(),
		})
		return
	}

	for i := range templates {
		if err := ensureFormTemplateUUID(h.db, &templates[i]); err != nil {
			logger.Warnf("补全模板 UUID 失败 template_id=%d: %v", templates[i].ID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
		"total":   total,
	})
}

// GetFormTemplate 获取表单模板详情
func (h *FormTemplateHandler) GetFormTemplate(c *gin.Context) {
	id := c.Param("id")

	var template model.FormTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "模板不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板详情失败",
			"error":   err.Error(),
		})
		return
	}

	if err := ensureFormTemplateUUID(h.db, &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "补全模板 UUID 失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}

// CreateFormTemplate 创建表单模板
func (h *FormTemplateHandler) CreateFormTemplate(c *gin.Context) {
	var template model.FormTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 设置默认值
	if template.Status == "" {
		template.Status = "active"
	}
	if template.Version == "" {
		template.Version = "1.0.0"
	}

	if err := h.db.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建模板失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}

// UpdateFormTemplate 更新表单模板
func (h *FormTemplateHandler) UpdateFormTemplate(c *gin.Context) {
	id := c.Param("id")

	var template model.FormTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "模板不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板失败",
			"error":   err.Error(),
		})
		return
	}

	var updateData model.FormTemplate
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	if updateData.Name != "" {
		template.Name = updateData.Name
	}
	if updateData.Category != "" {
		template.Category = updateData.Category
	}
	if updateData.Description != "" {
		template.Description = updateData.Description
	}
	if updateData.Schema != nil {
		template.Schema = updateData.Schema
	}
	if updateData.ApprovalConfig != nil {
		template.ApprovalConfig = updateData.ApprovalConfig
	}
	if updateData.Status != "" {
		template.Status = updateData.Status
	}
	// 版本号：如果前端传了版本号就使用，否则保持原版本（前端会自动计算并传递）
	if updateData.Version != "" {
		template.Version = updateData.Version
	}

	if err := h.db.Save(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新模板失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}

// DeleteFormTemplate 删除表单模板
func (h *FormTemplateHandler) DeleteFormTemplate(c *gin.Context) {
	id := c.Param("id")

	var template model.FormTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "模板不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查是否有工单使用此模板
	var count int64
	h.db.Model(&model.Ticket{}).Where("template_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该模板正在被使用，无法删除",
		})
		return
	}

	if err := h.db.Delete(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除模板失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetApprovalCallbackURL 生成该工单模板专用的三方审批回调地址
func (h *FormTemplateHandler) GetApprovalCallbackURL(c *gin.Context) {
	id := c.Param("id")
	platform := strings.ToLower(strings.TrimSpace(c.DefaultQuery("platform", "feishu")))
	source := strings.Trim(strings.TrimSpace(c.DefaultQuery("source", "ticket")), "/")
	if source == "" {
		source = "ticket"
	}
	if platform == "" {
		platform = "feishu"
	}

	var template model.FormTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "模板不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板失败",
			"error":   err.Error(),
		})
		return
	}

	baseURL := strings.TrimSpace(c.Query("base_url"))
	if baseURL == "" {
		scheme := "https"
		if c.Request.TLS == nil && !strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "http"
		}
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = strings.ToLower(strings.TrimSpace(proto))
		}
		baseURL = scheme + "://" + c.Request.Host
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if err := ensureFormTemplateUUID(h.db, &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "补全模板 UUID 失败",
			"error":   err.Error(),
		})
		return
	}

	path := BuildApprovalCallbackPath(source, platform, template.UUID)
	callbackURL := baseURL + path

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"callback_url":  callbackURL,
			"path":          path,
			"source":        source,
			"platform":      platform,
			"scope_id":      template.UUID,
			"template_id":   template.ID,
			"template_uuid": template.UUID,
			"template_name": template.Name,
		},
	})
}

// PreviewFormTemplate 预览表单模板
func (h *FormTemplateHandler) PreviewFormTemplate(c *gin.Context) {
	id := c.Param("id")

	var template model.FormTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "模板不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取模板失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}
