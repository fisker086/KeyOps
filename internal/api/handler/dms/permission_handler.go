package dms

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service/dms"
	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permissionService *dms.PermissionService
}

func NewPermissionHandler(permissionService *dms.PermissionService) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
	}
}

// GrantPermission 分配权限
// @Summary 分配数据库权限
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.GrantPermissionRequest true "权限信息"
// @Success 200 {object} model.Response
// @Router /api/dms/permissions [post]
func (h *PermissionHandler) GrantPermission(c *gin.Context) {
	var req dms.GrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	req.GrantedBy = userID.(string)

	if err := h.permissionService.GrantPermission(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "分配权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "权限分配成功"}))
}

// RevokePermission 回收权限
// @Summary 回收数据库权限
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.RevokePermissionRequest true "权限信息"
// @Success 200 {object} model.Response
// @Router /api/dms/permissions [delete]
func (h *PermissionHandler) RevokePermission(c *gin.Context) {
	var req dms.RevokePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.permissionService.RevokePermission(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "回收权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "权限回收成功"}))
}

// BatchGrantPermissions 批量分配权限
// @Summary 批量分配数据库权限
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.BatchGrantPermissionRequest true "批量权限信息"
// @Success 200 {object} model.Response
// @Router /api/dms/permissions/batch [post]
func (h *PermissionHandler) BatchGrantPermissions(c *gin.Context) {
	var req dms.BatchGrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if len(req.Permissions) == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "权限列表不能为空"))
		return
	}

	userID, _ := c.Get("userID")
	grantedBy := userID.(string)

	// 构建批量授权请求（简化：只支持实例级别和数据库级别，不再支持表级别）
	var grantReqs []*dms.GrantPermissionRequest
	for _, perm := range req.Permissions {
		grantReq := &dms.GrantPermissionRequest{
			UserID:         req.UserID,
			InstanceID:     req.InstanceID,
			DatabaseName:   perm.DatabaseName, // 留空表示实例级别权限
			TableName:      "",                 // 不再支持表级别权限
			PermissionType: req.PermissionType,
			GrantedBy:      grantedBy,
			ExpiresAt:      req.ExpiresAt,
			Description:    req.Description,
		}
		grantReqs = append(grantReqs, grantReq)
	}

	if err := h.permissionService.BatchGrantPermissions(grantReqs); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "批量分配权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "批量权限分配成功", "count": len(grantReqs)}))
}

// UpdatePermission 更新权限
// @Summary 更新数据库权限（只更新元数据：过期时间、描述）
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.UpdatePermissionRequest true "权限信息"
// @Success 200 {object} model.Response
// @Router /api/dms/permissions [put]
func (h *PermissionHandler) UpdatePermission(c *gin.Context) {
	var req dms.UpdatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.permissionService.UpdatePermission(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "权限更新成功"}))
}

// UpdatePermissionResource 更新权限的资源路径（数据库、权限类型）
// @Summary 更新权限的资源路径（先添加新权限，再删除旧权限）
// @Tags DMS
// @Accept json
// @Produce json
// @Param request body dms.UpdatePermissionResourceRequest true "权限信息"
// @Success 200 {object} model.Response
// @Router /api/dms/permissions/resource [put]
func (h *PermissionHandler) UpdatePermissionResource(c *gin.Context) {
	var req dms.UpdatePermissionResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	req.GrantedBy = userID.(string)

	if err := h.permissionService.UpdatePermissionResource(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新权限资源失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"message": "权限资源更新成功"}))
}

// GetUserPermissions 获取用户权限列表
// @Summary 获取用户权限列表
// @Tags DMS
// @Accept json
// @Produce json
// @Param user_id query string false "用户ID（为空则获取所有用户的权限）"
// @Param instance_id query int false "实例ID"
// @Param page query int false "页码，从1开始"
// @Param page_size query int false "每页数量，默认20"
// @Success 200 {object} model.Response{data=model.PaginatedResponse{data=[]dms.PermissionInfo}}
// @Router /api/dms/permissions [get]
func (h *PermissionHandler) GetUserPermissions(c *gin.Context) {
	userID := c.Query("user_id")
	// 如果指定了user_id，只获取该用户的权限；如果为空，获取所有用户的权限

	filters := make(map[string]interface{})
	if instanceIDStr := c.Query("instance_id"); instanceIDStr != "" {
		if instanceID, err := strconv.ParseUint(instanceIDStr, 10, 32); err == nil {
			filters["instance_id"] = uint(instanceID)
		}
	}

	// 分页参数
	page := 1
	pageSize := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	permissions, total, err := h.permissionService.GetUserPermissionsWithPagination(userID, filters, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取权限列表失败: "+err.Error()))
		return
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	c.JSON(http.StatusOK, model.Success(model.PaginatedResponse{
		Data:       permissions,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}))
}

// GetMyPermissions 获取我的权限
// @Summary 获取当前用户的权限
// @Tags DMS
// @Accept json
// @Produce json
// @Success 200 {object} model.Response{data=[]dms.PermissionInfo}
// @Router /api/dms/permissions/my [get]
func (h *PermissionHandler) GetMyPermissions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	permissions, err := h.permissionService.GetUserPermissions(userIDStr, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取权限列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(permissions))
}
