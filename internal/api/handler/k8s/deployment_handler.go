package k8s

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

type DeploymentHandler struct {
	deploymentService  *k8sService.DeploymentService
	permissionService  *k8sService.K8sPermissionService
	roleRepo           *repository.RoleRepository
}

func NewDeploymentHandler(deploymentService *k8sService.DeploymentService, permissionService *k8sService.K8sPermissionService, roleRepo *repository.RoleRepository) *DeploymentHandler {
	return &DeploymentHandler{
		deploymentService: deploymentService,
		permissionService: permissionService,
		roleRepo:          roleRepo,
	}
}

// checkK8sDeployPermission 检查当前用户是否有指定集群+命名空间的部署权限（write 或 admin），无权限时返回 false 并已 Abort+写 403
func (h *DeploymentHandler) checkK8sDeployPermission(c *gin.Context, clusterID, namespace string) bool {
	if clusterID == "" || namespace == "" {
		c.JSON(http.StatusBadRequest, model.Response{Code: http.StatusBadRequest, Message: "集群与命名空间不能为空"})
		c.Abort()
		return false
	}
	var userIDStr string
	if uid, exists := c.Get("userID"); exists {
		if s, ok := uid.(string); ok {
			userIDStr = s
		}
	}
	if userIDStr == "" {
		if uid, exists := c.Get("user_id"); exists {
			if s, ok := uid.(string); ok {
				userIDStr = s
			}
		}
	}
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, model.Response{Code: http.StatusUnauthorized, Message: "未登录"})
		c.Abort()
		return false
	}
	// 部署需要 write 权限（创建/更新资源）
	hasPermission, err := h.permissionService.CheckPermission(userIDStr, clusterID, namespace, k8sService.ResourceTypeNamespace, "", k8sService.ActionWrite)
	if err == nil && hasPermission {
		logger.Infof("部署权限通过: user=%s cluster=%s namespace=%s (用户或角色有 write 权限)", userIDStr, clusterID, namespace)
		return true
	}
	if h.roleRepo != nil {
		roles, err := h.roleRepo.GetRolesByUserID(userIDStr)
		if err == nil {
			for _, role := range roles {
				if role.ID == "role:admin" {
					logger.Infof("部署权限通过: user=%s cluster=%s namespace=%s (管理员默认有权限)", userIDStr, clusterID, namespace)
					return true
				}
				hasPermission, err = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sService.ResourceTypeNamespace, "", k8sService.ActionWrite)
				if err == nil && hasPermission {
					logger.Infof("部署权限通过: user=%s cluster=%s namespace=%s (角色 %s 有 write 权限)", userIDStr, clusterID, namespace, role.ID)
					return true
				}
				hasPermission, _ = h.permissionService.CheckPermission(role.ID, clusterID, namespace, k8sService.ResourceTypeNamespace, "", k8sService.ActionAdmin)
				if hasPermission {
					logger.Infof("部署权限通过: user=%s cluster=%s namespace=%s (角色 %s 有 admin 权限)", userIDStr, clusterID, namespace, role.ID)
					return true
				}
			}
		}
	}
	logger.Warnf("部署权限拒绝: user=%s cluster=%s namespace=%s 无该集群/命名空间的 write 或 admin 权限", userIDStr, clusterID, namespace)
	c.JSON(http.StatusForbidden, model.Response{
		Code:    http.StatusForbidden,
		Message: "无权限操作：您没有该集群/命名空间的部署权限，请在 K8s 管理-权限管理中配置",
	})
	c.Abort()
	return false
}

// CreateDeployment 创建部署记录
// @Summary 创建部署记录
// @Description 创建新的部署记录
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param deployment body CreateDeploymentRequest true "部署信息"
// @Success 200 {object} model.Response{data=model.Deployment}
// @Router /api/deployments [post]
func (h *DeploymentHandler) CreateDeployment(c *gin.Context) {
	var req CreateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证部署类型
	if req.DeployType != model.DeployTypeJenkins && req.DeployType != model.DeployTypeK8s {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "部署类型必须是 jenkins 或 k8s",
		})
		return
	}

	// 验证必填字段
	if req.ProjectName == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "项目名称不能为空",
		})
		return
	}

	// K8s 部署：校验当前用户对该集群+命名空间是否有部署权限（与 K8s 管理-权限管理一致）
	if req.DeployType == model.DeployTypeK8s {
		if req.ClusterID == "" || req.Namespace == "" {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "K8s 部署需填写集群与命名空间",
			})
			return
		}
		if !h.checkK8sDeployPermission(c, req.ClusterID, req.Namespace) {
			return
		}
	}

	// 获取当前用户
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	
	createdBy := ""
	createdByName := ""
	if userID != nil {
		createdBy = userID.(string)
	}
	if userName != nil {
		createdByName = userName.(string)
	}

	deployment, err := h.deploymentService.CreateDeployment(&k8sService.CreateDeploymentRequest{
		ProjectName:   req.ProjectName,
		ProjectID:     req.ProjectID,
		EnvID:         req.EnvID,
		EnvName:       req.EnvName,
		ClusterID:     req.ClusterID,
		ClusterName:   req.ClusterName,
		Namespace:     req.Namespace,
		DeployType:    req.DeployType,
		Version:       req.Version,
		ArtifactURL:   req.ArtifactURL,
		JenkinsJob:    req.JenkinsJob,
		K8sYAML:       req.K8sYAML,
		K8sKind:       req.K8sKind,
		VerifyEnabled: req.VerifyEnabled,
		VerifyTimeout: req.VerifyTimeout,
		Description:   req.Description,
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "创建成功",
		Data:    deployment,
	})
}

// GetDeployment 获取部署记录详情
// @Summary 获取部署记录详情
// @Description 根据ID获取部署记录详情
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param id path string true "部署ID"
// @Success 200 {object} model.Response{data=model.Deployment}
// @Router /api/deployments/{id} [get]
func (h *DeploymentHandler) GetDeployment(c *gin.Context) {
	id := c.Param("id")
	
	deployment, err := h.deploymentService.GetDeployment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "部署记录不存在",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "获取成功",
		Data:    deployment,
	})
}

// ListDeployments 查询部署记录列表
// @Summary 查询部署记录列表
// @Description 查询部署记录列表，支持分页和筛选
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID"
// @Param project_name query string false "项目名称"
// @Param env_id query string false "环境ID"
// @Param cluster_id query string false "集群ID"
// @Param deploy_type query string false "部署类型: jenkins, k8s"
// @Param status query string false "状态: pending, running, success, failed, cancelled"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.Response{data=DeploymentListResponse}
// @Router /api/deployments [get]
func (h *DeploymentHandler) ListDeployments(c *gin.Context) {
	params := &repository.DeploymentListParams{
		ProjectID:   c.Query("project_id"),
		ProjectName: c.Query("project_name"),
		EnvID:       c.Query("env_id"),
		ClusterID:   c.Query("cluster_id"),
		DeployType:  c.Query("deploy_type"),
		Status:      c.Query("status"),
		Page:        1,
		PageSize:    20,
		OrderBy:     "created_at DESC",
	}

	// 解析分页参数
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			params.Page = page
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			params.PageSize = pageSize
		}
	}

	// 解析时间范围（支持多种时间格式）
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		var startTime time.Time
		var err error
		// 尝试 ISO 8601 格式
		if startTime, err = time.Parse(time.RFC3339, startTimeStr); err != nil {
			// 尝试标准格式
			if startTime, err = time.Parse("2006-01-02 15:04:05", startTimeStr); err != nil {
				// 尝试日期格式
				if startTime, err = time.Parse("2006-01-02", startTimeStr); err == nil {
					// 日期格式，设置为当天的开始时间
					startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
				}
			}
		}
		if err == nil {
			params.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		var endTime time.Time
		var err error
		// 尝试 ISO 8601 格式
		if endTime, err = time.Parse(time.RFC3339, endTimeStr); err != nil {
			// 尝试标准格式
			if endTime, err = time.Parse("2006-01-02 15:04:05", endTimeStr); err != nil {
				// 尝试日期格式
				if endTime, err = time.Parse("2006-01-02", endTimeStr); err == nil {
					// 日期格式，设置为当天的结束时间
					endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
				}
			}
		}
		if err == nil {
			params.EndTime = &endTime
		}
	}

	// 如果提供了created_by，只查询当前用户的部署记录
	userID, exists := c.Get("user_id")
	if exists && c.Query("my_deployments") == "true" {
		params.CreatedBy = userID.(string)
	}

	deployments, total, err := h.deploymentService.ListDeployments(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "查询成功",
		Data: DeploymentListResponse{
			Items: deployments,
			Total: total,
			Page:  params.Page,
			PageSize: params.PageSize,
		},
	})
}

// UpdateDeploymentStatus 更新部署状态
// @Summary 更新部署状态
// @Description 更新部署记录的状态
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param id path string true "部署ID"
// @Param status body UpdateDeploymentStatusRequest true "状态信息"
// @Success 200 {object} model.Response
// @Router /api/deployments/{id}/status [put]
func (h *DeploymentHandler) UpdateDeploymentStatus(c *gin.Context) {
	id := c.Param("id")
	
	var req UpdateDeploymentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证状态值
	validStatuses := map[string]bool{
		model.DeploymentStatusPending:   true,
		model.DeploymentStatusRunning:    true,
		model.DeploymentStatusSuccess:    true,
		model.DeploymentStatusFailed:     true,
		model.DeploymentStatusCancelled:  true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "无效的状态值，必须是: pending, running, success, failed, cancelled",
		})
		return
	}

	var duration *int
	if req.Duration > 0 {
		duration = &req.Duration
	}

	err := h.deploymentService.UpdateDeploymentStatus(id, req.Status, duration, req.LogPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "更新成功",
	})
}

// DeleteDeployment 删除部署记录
// @Summary 删除部署记录
// @Description 删除部署记录
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param id path string true "部署ID"
// @Success 200 {object} model.Response
// @Router /api/deployments/{id} [delete]
func (h *DeploymentHandler) DeleteDeployment(c *gin.Context) {
	id := c.Param("id")
	
	err := h.deploymentService.DeleteDeployment(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "删除成功",
	})
}

// ExecuteK8sDeployment 执行 K8s 容器部署（包含 kubedog 监听）
// @Summary 执行 K8s 容器部署
// @Description 执行 K8s 容器部署，如果启用了验证，会使用 kubedog 监听部署状态
// @Tags 发布管理
// @Accept json
// @Produce json
// @Param id path string true "部署ID"
// @Success 200 {object} model.Response{data=model.Deployment}
// @Router /api/deployments/{id}/execute [post]
func (h *DeploymentHandler) ExecuteK8sDeployment(c *gin.Context) {
	id := c.Param("id")
	deployment, err := h.deploymentService.GetDeployment(id)
	if err != nil || deployment == nil {
		c.JSON(http.StatusNotFound, model.Response{Code: http.StatusNotFound, Message: "部署记录不存在"})
		return
	}
	// 执行前校验：当前用户对该集群+命名空间是否有部署权限
	if !h.checkK8sDeployPermission(c, deployment.ClusterID, deployment.Namespace) {
		return
	}
	// 在 goroutine 中执行部署，避免阻塞 API 响应
	go func() {
		if err := h.deploymentService.ExecuteK8sDeployment(id); err != nil {
			return
		}
	}()

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "部署任务已启动，正在后台执行",
	})
}

// CreateDeploymentRequest 创建部署请求
type CreateDeploymentRequest struct {
	ProjectName string `json:"project_name" binding:"required"`
	ProjectID   string `json:"project_id"`
	EnvID       string `json:"env_id"`
	EnvName     string `json:"env_name"`
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Namespace   string `json:"namespace"`
	DeployType  string `json:"deploy_type" binding:"required"` // jenkins, k8s
	Version     string `json:"version"`
	ArtifactURL string `json:"artifact_url"`
	JenkinsJob    string `json:"jenkins_job"`
	K8sYAML       string `json:"k8s_yaml"`
	K8sKind       string `json:"k8s_kind"`
	VerifyEnabled bool   `json:"verify_enabled"`
	VerifyTimeout int    `json:"verify_timeout"`
	Description   string `json:"description"`
}

// UpdateDeploymentStatusRequest 更新部署状态请求
type UpdateDeploymentStatusRequest struct {
	Status   string `json:"status" binding:"required"`
	Duration int    `json:"duration"`
	LogPath  string `json:"log_path"`
}

// DeploymentListResponse 部署列表响应
type DeploymentListResponse struct {
	Items    []*model.Deployment `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

