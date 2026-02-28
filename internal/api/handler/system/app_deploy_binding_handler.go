package system

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AppDeployBindingHandler struct {
	bindingRepo *repository.ApplicationDeployBindingRepository
	appRepo     *repository.ApplicationRepository
}

func NewAppDeployBindingHandler(
	bindingRepo *repository.ApplicationDeployBindingRepository,
	appRepo *repository.ApplicationRepository,
) *AppDeployBindingHandler {
	return &AppDeployBindingHandler{
		bindingRepo: bindingRepo,
		appRepo:     appRepo,
	}
}

// ListApplicationDeployBindings 获取应用-发布绑定列表
// @Summary 获取应用-发布绑定列表
// @Tags app-deploy-bindings
// @Produce json
// @Param applicationId query string false "应用ID"
// @Param deployType query string false "发布类型: jenkins, argocd"
// @Param environment query string false "环境: dev, test, qa, staging, prod"
// @Param enabled query boolean false "是否启用"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} model.Response
// @Router /api/app-deploy-bindings [get]
func (h *AppDeployBindingHandler) ListApplicationDeployBindings(c *gin.Context) {
	var req model.ListApplicationDeployBindingsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request parameters: " + err.Error(),
			Data:    nil,
		})
		return
	}

	bindings, total, err := h.bindingRepo.FindAll(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch application deploy bindings",
			Data:    nil,
		})
		return
	}
	scheme := "https"
	if x := c.GetHeader("X-Forwarded-Proto"); x != "" {
		scheme = strings.TrimSpace(strings.Split(x, ",")[0])
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	base := scheme + "://" + c.Request.Host
	for i := range bindings {
		if bindings[i].WebhookToken != "" {
			bindings[i].WebhookURL = base + "/api/release/webhook/push?token=" + bindings[i].WebhookToken
		}
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data: gin.H{
			"list":      bindings,
			"total":     total,
			"page":      req.Page,
			"pageSize":  req.PageSize,
		},
	})
}

// CreateApplicationDeployBinding 创建应用-发布绑定
// @Summary 创建应用-发布绑定
// @Tags app-deploy-bindings
// @Accept json
// @Produce json
// @Param binding body model.CreateApplicationDeployBindingRequest true "绑定信息"
// @Success 200 {object} model.Response
// @Router /api/app-deploy-bindings [post]
func (h *AppDeployBindingHandler) CreateApplicationDeployBinding(c *gin.Context) {
	var req model.CreateApplicationDeployBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 验证应用是否存在
	_, err := h.appRepo.FindByID(req.ApplicationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Application not found",
			Data:    nil,
		})
		return
	}

	// 验证必填字段
	if req.DeployType == "jenkins" && req.JenkinsJob == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Jenkins Job is required when deployType is jenkins",
			Data:    nil,
		})
		return
	}

	if req.DeployType == "argocd" && req.ArgoCDApplication == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "ArgoCD Application is required when deployType is argocd",
			Data:    nil,
		})
		return
	}

	// 检查绑定关系是否已存在
	exists, err := h.bindingRepo.CheckBindingExists(
		req.ApplicationID,
		req.DeployType,
		req.DeployConfigID,
		req.Environment,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to check binding existence",
			Data:    nil,
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Binding already exists",
			Data:    nil,
		})
		return
	}

	// 获取当前用户ID（从上下文或token中获取）
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "system"
	}

	wt := uuid.New().String() + uuid.New().String()
	webhookToken := strings.ReplaceAll(wt, "-", "")
	if len(webhookToken) > 48 {
		webhookToken = webhookToken[:48]
	}
	binding := &model.ApplicationDeployBinding{
		ID:                uuid.New().String(),
		ApplicationID:     req.ApplicationID,
		DeployType:        req.DeployType,
		DeployConfigID:    req.DeployConfigID,
		DeployConfigName:  req.DeployConfigName,
		Environment:       req.Environment,
		JenkinsJob:        req.JenkinsJob,
		ArgoCDApplication: req.ArgoCDApplication,
		DeployStrategy:    req.DeployStrategy,
		StrategyOptions:   req.StrategyOptions,
		PipelineID:        req.PipelineID,
		WebhookToken:      webhookToken,
		Enabled:           req.Enabled,
		Description:       req.Description,
		CreatedBy:         userID,
	}

	if err := h.bindingRepo.Create(binding); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create binding: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Binding created successfully",
		Data:    binding,
	})
}

// UpdateApplicationDeployBinding 更新应用-发布绑定
// @Summary 更新应用-发布绑定
// @Tags app-deploy-bindings
// @Accept json
// @Produce json
// @Param id path string true "绑定ID"
// @Param binding body model.UpdateApplicationDeployBindingRequest true "绑定信息"
// @Success 200 {object} model.Response
// @Router /api/app-deploy-bindings/{id} [put]
func (h *AppDeployBindingHandler) UpdateApplicationDeployBinding(c *gin.Context) {
	id := c.Param("id")

	var req model.UpdateApplicationDeployBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 查找绑定
	binding, err := h.bindingRepo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "Binding not found",
			Data:    nil,
		})
		return
	}

	// 更新字段
	if req.DeployConfigID != "" {
		binding.DeployConfigID = req.DeployConfigID
	}
	if req.DeployConfigName != "" {
		binding.DeployConfigName = req.DeployConfigName
	}
	if req.Environment != "" {
		binding.Environment = req.Environment
	}
	if req.JenkinsJob != "" {
		binding.JenkinsJob = req.JenkinsJob
	}
	if req.ArgoCDApplication != "" {
		binding.ArgoCDApplication = req.ArgoCDApplication
	}
	binding.DeployStrategy = req.DeployStrategy
	binding.StrategyOptions = req.StrategyOptions
	binding.PipelineID = req.PipelineID
	if req.Enabled != nil {
		binding.Enabled = *req.Enabled
	}
	if req.Description != "" {
		binding.Description = req.Description
	}

	if err := h.bindingRepo.Update(binding); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update binding: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Binding updated successfully",
		Data:    binding,
	})
}

// DeleteApplicationDeployBinding 删除应用-发布绑定
// @Summary 删除应用-发布绑定
// @Tags app-deploy-bindings
// @Produce json
// @Param id path string true "绑定ID"
// @Success 200 {object} model.Response
// @Router /api/app-deploy-bindings/{id} [delete]
func (h *AppDeployBindingHandler) DeleteApplicationDeployBinding(c *gin.Context) {
	id := c.Param("id")

	if err := h.bindingRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete binding: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Binding deleted successfully",
		Data:    nil,
	})
}

// GetApplicationsForDeploy 获取可用于发布的应用列表
// @Summary 获取可用于发布的应用列表
// @Tags app-deploy-bindings
// @Produce json
// @Param deployType query string true "发布类型: jenkins, argocd"
// @Param environment query string false "环境: dev, test, qa, staging, prod"
// @Param keyword query string false "应用名称关键字"
// @Success 200 {object} model.Response
// @Router /api/app-deploy-bindings/applications [get]
func (h *AppDeployBindingHandler) GetApplicationsForDeploy(c *gin.Context) {
	var req model.GetApplicationsForDeployRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request parameters: " + err.Error(),
			Data:    nil,
		})
		return
	}

	apps, err := h.bindingRepo.GetApplicationsForDeploy(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch applications",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    apps,
	})
}

