package deployparam

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeployParamHandler struct {
	db *gorm.DB
}

func NewDeployParamHandler(db *gorm.DB) *DeployParamHandler {
	return &DeployParamHandler{db: db}
}

type createDeployParamReq struct {
	Name           string `json:"name" binding:"required"`
	DataType       string `json:"data_type" binding:"required"`
	ChineseName    string `json:"chinese_name" binding:"required"`
	ValidationRule string `json:"validation_rule"`
	Description    string `json:"description"`
	GroupName      string `json:"group_name"`
	SortOrder      int    `json:"sort_order"`
	Category       string `json:"category"`
}

type updateDeployParamReq struct {
	Name           string `json:"name" binding:"required"`
	DataType       string `json:"data_type" binding:"required"`
	ChineseName    string `json:"chinese_name" binding:"required"`
	ValidationRule string `json:"validation_rule"`
	Description    string `json:"description"`
	GroupName      string `json:"group_name"`
	SortOrder      int    `json:"sort_order"`
	Category       string `json:"category"`
}

type upsertTemplateReq struct {
	Language    string                 `json:"language" binding:"required"`
	VersionName string                 `json:"versionName" binding:"required"`
	Description string                 `json:"description"`
	IsDefault   bool                   `json:"isDefault"`
	Params      map[string]interface{} `json:"params" binding:"required"`
}

// ListParams 获取部署参数定义列表
func (h *DeployParamHandler) ListParams(c *gin.Context) {
	var params []model.DeployParam
	if err := h.db.Order("sort_order asc, id asc").Find(&params).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch deploy params",
		})
		return
	}
	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    params,
	})
}

// CreateParam 创建部署参数
func (h *DeployParamHandler) CreateParam(c *gin.Context) {
	var req createDeployParamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}
	param := model.DeployParam{
		Name:           req.Name,
		DataType:       req.DataType,
		ChineseName:    req.ChineseName,
		ValidationRule: req.ValidationRule,
		Description:    req.Description,
		GroupName:      req.GroupName,
		SortOrder:      req.SortOrder,
		Category:       req.Category,
	}
	if param.Category == "" {
		param.Category = "other"
	}
	if err := h.db.Create(&param).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Success(param))
}

// UpdateParam 更新部署参数
func (h *DeployParamHandler) UpdateParam(c *gin.Context) {
	id := c.Param("id")
	var req updateDeployParamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}
	updates := map[string]interface{}{
		"name":            req.Name,
		"data_type":       req.DataType,
		"chinese_name":    req.ChineseName,
		"validation_rule": req.ValidationRule,
		"description":     req.Description,
		"group_name":      req.GroupName,
		"sort_order":      req.SortOrder,
		"category":        req.Category,
	}
	if req.Category == "" {
		updates["category"] = "other"
	}
	result := h.db.Model(&model.DeployParam{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新失败: " + result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, model.Response{Code: 404, Message: "记录不存在"})
		return
	}
	var param model.DeployParam
	h.db.First(&param, id)
	c.JSON(http.StatusOK, model.Success(param))
}

// DeleteParam 删除部署参数（检查引用保护）
func (h *DeployParamHandler) DeleteParam(c *gin.Context) {
	id := c.Param("id")

	// 获取参数名用于引用检查
	var param model.DeployParam
	if err := h.db.First(&param, id).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: 404, Message: "记录不存在"})
		return
	}

	// 检查是否被 AppDeployParamConfig 引用
	var appCount int64
	h.db.Model(&model.AppDeployParamConfig{}).Where("param_name = ?", param.Name).Count(&appCount)
	if appCount > 0 {
		c.JSON(http.StatusConflict, model.Response{Code: 409, Message: "该参数被应用配置引用，无法删除"})
		return
	}

	// 检查是否被 AppDeployParamDefault 引用
	var defCount int64
	h.db.Model(&model.AppDeployParamDefault{}).Where("param_name = ?", param.Name).Count(&defCount)
	if defCount > 0 {
		c.JSON(http.StatusConflict, model.Response{Code: 409, Message: "该参数被全局默认值引用，无法删除"})
		return
	}

	result := h.db.Delete(&model.DeployParam{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "删除失败: " + result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// ListTemplates 获取参数模板列表
func (h *DeployParamHandler) ListTemplates(c *gin.Context) {
	var rows []model.ParamTemplate
	if err := h.db.Order("language asc, is_default desc, created_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "获取模板失败"})
		return
	}
	data := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		params := map[string]string{}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(r.Content), &obj); err == nil {
			for k, v := range obj {
				switch vv := v.(type) {
				case string:
					params[k] = vv
				default:
					params[k] = ""
				}
			}
		} else {
			var arr []string
			if err2 := json.Unmarshal([]byte(r.Content), &arr); err2 == nil {
				for _, k := range arr {
					if k == "" {
						continue
					}
					params[k] = ""
				}
			}
		}
		data = append(data, map[string]interface{}{
			"id":          r.ID,
			"language":    r.Language,
			"versionName": r.VersionName,
			"description": r.Description,
			"isDefault":   r.IsDefault,
			"params":      params,
		})
	}
	c.JSON(http.StatusOK, model.Success(data))
}

// UpsertTemplate 新增/更新参数模板
func (h *DeployParamHandler) UpsertTemplate(c *gin.Context) {
	var req upsertTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}
	normalized := map[string]string{}
	for k, v := range req.Params {
		if k == "" {
			continue
		}
		switch vv := v.(type) {
		case string:
			normalized[k] = vv
		case float64:
			normalized[k] = strconv.FormatFloat(vv, 'f', -1, 64)
		case bool:
			if vv {
				normalized[k] = "true"
			} else {
				normalized[k] = "false"
			}
		default:
			normalized[k] = ""
		}
	}
	b, _ := json.Marshal(normalized)
	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存模板失败: " + tx.Error.Error()})
		return
	}
	defer tx.Rollback()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var exists model.ParamTemplate
	if err := tx.Where("language = ? AND version_name = ?", req.Language, req.VersionName).First(&exists).Error; err == nil {
		c.JSON(http.StatusConflict, model.Response{Code: 409, Message: "同语言下版本名已存在，请更换版本名"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存模板失败: " + err.Error()})
		return
	}

	if req.IsDefault {
		if err := tx.Model(&model.ParamTemplate{}).Where("language = ?", req.Language).Update("is_default", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存模板失败: " + err.Error()})
			return
		}
	}

	row := model.ParamTemplate{
		Language:    req.Language,
		VersionName: req.VersionName,
		Description: req.Description,
		IsDefault:   req.IsDefault,
		Content:     string(b),
	}
	if err := tx.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存模板失败: " + err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存模板失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Success(row))
}

// UpdateTemplate 更新模板版本内容
func (h *DeployParamHandler) UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req upsertTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	var row model.ParamTemplate
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: 404, Message: "模板不存在"})
		return
	}

	normalized := map[string]string{}
	for k, v := range req.Params {
		if k == "" {
			continue
		}
		switch vv := v.(type) {
		case string:
			normalized[k] = vv
		case float64:
			normalized[k] = strconv.FormatFloat(vv, 'f', -1, 64)
		case bool:
			if vv {
				normalized[k] = "true"
			} else {
				normalized[k] = "false"
			}
		default:
			normalized[k] = ""
		}
	}
	b, _ := json.Marshal(normalized)

	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新模板失败: " + tx.Error.Error()})
		return
	}
	defer tx.Rollback()

	var dup model.ParamTemplate
	if err := tx.Where("language = ? AND version_name = ? AND id <> ?", req.Language, req.VersionName, id).First(&dup).Error; err == nil {
		c.JSON(http.StatusConflict, model.Response{Code: 409, Message: "同语言下版本名已存在，请更换版本名"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新模板失败: " + err.Error()})
		return
	}

	if req.IsDefault {
		if err := tx.Model(&model.ParamTemplate{}).Where("language = ?", req.Language).Update("is_default", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新模板失败: " + err.Error()})
			return
		}
	}

	updates := map[string]interface{}{
		"language":     req.Language,
		"version_name": req.VersionName,
		"description":  req.Description,
		"is_default":   req.IsDefault,
		"content":      string(b),
	}
	if err := tx.Model(&model.ParamTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新模板失败: " + err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "更新模板失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// DeleteTemplate 删除模板版本
func (h *DeployParamHandler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	var row model.ParamTemplate
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: 404, Message: "模板不存在"})
		return
	}

	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "删除失败: " + tx.Error.Error()})
		return
	}
	defer tx.Rollback()

	if err := tx.Delete(&model.ParamTemplate{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "删除失败: " + err.Error()})
		return
	}

	// 如果删的是默认版本，补一个新的默认版本（按最新更新时间）
	if row.IsDefault {
		var next model.ParamTemplate
		if err := tx.Where("language = ?", row.Language).Order("updated_at desc, id desc").First(&next).Error; err == nil {
			if err2 := tx.Model(&model.ParamTemplate{}).Where("id = ?", next.ID).Update("is_default", true).Error; err2 != nil {
				c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "删除失败: " + err2.Error()})
				return
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Success(nil))
}
