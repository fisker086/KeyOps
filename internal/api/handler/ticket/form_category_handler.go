package ticket

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/model"
)

// FormCategoryHandler 表单模板分类
type FormCategoryHandler struct {
	db *gorm.DB
}

// NewFormCategoryHandler 创建表单模板分类处理器
func NewFormCategoryHandler(db *gorm.DB) *FormCategoryHandler {
	return &FormCategoryHandler{db: db}
}

// ListCategories 获取分类列表
func (h *FormCategoryHandler) ListCategories(c *gin.Context) {
	var categories []model.FormCategory

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	query := h.db.Model(&model.FormCategory{})

	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize

	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    categories,
		"total":   total,
	})
}

// CreateCategory 创建分类
func (h *FormCategoryHandler) CreateCategory(c *gin.Context) {
	var payload model.FormCategory
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "分类名称不能为空",
		})
		return
	}

	// 唯一性检查
	var exist model.FormCategory
	if err := h.db.Where("name = ?", payload.Name).First(&exist).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "分类已存在",
		})
		return
	}

	if err := h.db.Create(&payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    payload,
	})
}

// UpdateCategory 更新分类
func (h *FormCategoryHandler) UpdateCategory(c *gin.Context) {
	id := c.Param("id")

	var category model.FormCategory
	if err := h.db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "分类不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类失败",
			"error":   err.Error(),
		})
		return
	}

	var payload model.FormCategory
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}

	// 名称更新且唯一性检查
	if payload.Name != "" && payload.Name != category.Name {
		var exist model.FormCategory
		if err := h.db.Where("name = ?", payload.Name).First(&exist).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "分类已存在",
			})
			return
		}
		// 同步更新模板的分类名称
		if err := h.db.Model(&model.FormTemplate{}).
			Where("category = ?", category.Name).
			Update("category", payload.Name).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "更新分类关联模板失败",
				"error":   err.Error(),
			})
			return
		}
		category.Name = payload.Name
	}

	if payload.Description != "" {
		category.Description = payload.Description
	}

	if err := h.db.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    category,
	})
}

// DeleteCategory 删除分类，同时将使用该分类的模板分类置空
func (h *FormCategoryHandler) DeleteCategory(c *gin.Context) {
	id := c.Param("id")

	var category model.FormCategory
	if err := h.db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "分类不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类失败",
			"error":   err.Error(),
		})
		return
	}

	// 将该分类的模板分类置空
	if err := h.db.Model(&model.FormTemplate{}).
		Where("category = ?", category.Name).
		Update("category", "").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新模板分类失败",
			"error":   err.Error(),
		})
		return
	}

	if err := h.db.Delete(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetCategory 获取单个分类
func (h *FormCategoryHandler) GetCategory(c *gin.Context) {
	id := c.Param("id")
	var category model.FormCategory
	if err := h.db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "分类不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    category,
	})
}


