package system

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EnvironmentHandler struct {
	repo repository.EnvironmentRepository
}

func NewEnvironmentHandler(repo repository.EnvironmentRepository) *EnvironmentHandler {
	return &EnvironmentHandler{repo: repo}
}

// ListEnvironments 获取环境列表
// @Summary 获取环境列表
// @Tags environments
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/environments [get]
func (h *EnvironmentHandler) ListEnvironments(c *gin.Context) {
	envs, err := h.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to fetch environments"})
		return
	}
	c.JSON(http.StatusOK, model.Response{Code: http.StatusOK, Message: "Success", Data: envs})
}

// GetEnvironment 获取单个环境
// @Summary 获取单个环境
// @Tags environments
// @Produce json
// @Param id path string true "Environment ID"
// @Success 200 {object} model.Response
// @Router /api/environments/{id} [get]
func (h *EnvironmentHandler) GetEnvironment(c *gin.Context) {
	id := c.Param("id")
	env, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: http.StatusNotFound, Message: "Environment not found"})
		return
	}
	c.JSON(http.StatusOK, model.Response{Code: http.StatusOK, Message: "Success", Data: env})
}

// CreateEnvironment 创建环境
// @Summary 创建环境
// @Tags environments
// @Accept json
// @Produce json
// @Param environment body model.CreateEnvironmentRequest true "Environment"
// @Success 200 {object} model.Response
// @Router /api/environments [post]
func (h *EnvironmentHandler) CreateEnvironment(c *gin.Context) {
	var req model.CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "Invalid request body: " + err.Error()})
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "Code and name are required"})
		return
	}
	exists, err := h.repo.CheckCodeExists(req.Code, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to check environment code"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "Environment code already exists"})
		return
	}

	env := &model.Environment{
		ID:          uuid.New().String(),
		Code:        req.Code,
		Name:        req.Name,
		IsActive:    req.IsActive,
		SortOrder:   req.SortOrder,
		Description: req.Description,
	}
	if err := h.repo.Create(env); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to create environment: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Code: http.StatusOK, Message: "Environment created successfully", Data: env})
}

// UpdateEnvironment 更新环境
// @Summary 更新环境
// @Tags environments
// @Accept json
// @Produce json
// @Param id path string true "Environment ID"
// @Param environment body model.UpdateEnvironmentRequest true "Environment"
// @Success 200 {object} model.Response
// @Router /api/environments/{id} [put]
func (h *EnvironmentHandler) UpdateEnvironment(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "Invalid request body: " + err.Error()})
		return
	}
	env, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: http.StatusNotFound, Message: "Environment not found"})
		return
	}
	if code := strings.TrimSpace(req.Code); code != "" {
		exists, err := h.repo.CheckCodeExists(code, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to check environment code"})
			return
		}
		if exists {
			c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "Environment code already exists"})
			return
		}
		env.Code = code
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		env.Name = name
	}
	if req.IsActive != nil {
		env.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		env.SortOrder = *req.SortOrder
	}
	env.Description = req.Description

	if err := h.repo.Update(env); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to update environment: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Code: http.StatusOK, Message: "Environment updated successfully", Data: env})
}

// DeleteEnvironment 删除环境
// @Summary 删除环境
// @Tags environments
// @Produce json
// @Param id path string true "Environment ID"
// @Success 200 {object} model.Response
// @Router /api/environments/{id} [delete]
func (h *EnvironmentHandler) DeleteEnvironment(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.repo.FindByID(id); err != nil {
		c.JSON(http.StatusNotFound, model.Response{Code: http.StatusNotFound, Message: "Environment not found"})
		return
	}
	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: http.StatusInternalServerError, Message: "Failed to delete environment"})
		return
	}
	c.JSON(http.StatusOK, model.Response{Code: http.StatusOK, Message: "Environment deleted successfully"})
}
