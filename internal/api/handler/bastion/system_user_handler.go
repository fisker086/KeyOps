package bastion

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SystemUserHandler struct {
	repo *repository.SystemUserRepository
}

func NewSystemUserHandler(repo *repository.SystemUserRepository) *SystemUserHandler {
	return &SystemUserHandler{repo: repo}
}

// ListSystemUsers 获取系统用户列表
// @Summary 获取系统用户列表
// @Tags system-users
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/system-users [get]
func (h *SystemUserHandler) ListSystemUsers(c *gin.Context) {
	status := c.Query("status")

	var systemUsers []model.SystemUser
	var err error

	if status != "" {
		systemUsers, err = h.repo.FindByStatus(status)
	} else {
		systemUsers, err = h.repo.FindAll()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch system users",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    systemUsers,
	})
}

// GetSystemUser 获取单个系统用户
// @Summary 获取单个系统用户
// @Tags system-users
// @Produce json
// @Param id path string true "System User ID"
// @Success 200 {object} model.Response
// @Router /api/system-users/{id} [get]
func (h *SystemUserHandler) GetSystemUser(c *gin.Context) {
	id := c.Param("id")

	systemUser, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "System user not found",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    systemUser,
	})
}

// CreateSystemUser 创建系统用户
// @Summary 创建系统用户
// @Tags system-users
// @Accept json
// @Produce json
// @Param systemUser body model.SystemUser true "System User"
// @Success 200 {object} model.Response
// @Router /api/system-users [post]
func (h *SystemUserHandler) CreateSystemUser(c *gin.Context) {
	var systemUser model.SystemUser
	if err := c.ShouldBindJSON(&systemUser); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 验证认证方式和对应的认证信息
	if systemUser.AuthType != "password" && systemUser.AuthType != "key" && systemUser.AuthType != "both" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid auth type. Must be 'password', 'key', or 'both'",
			Data:    nil,
		})
		return
	}

	if systemUser.AuthType == "password" && systemUser.Password == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Password is required when auth type is 'password'",
			Data:    nil,
		})
		return
	}
	
	if systemUser.AuthType == "both" && systemUser.Password == "" && systemUser.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "At least password or private key is required when auth type is 'both'",
			Data:    nil,
		})
		return
	}

	if systemUser.AuthType == "key" && systemUser.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Private key is required when auth type is 'key'",
			Data:    nil,
		})
		return
	}

	// 生成ID
	systemUser.ID = uuid.New().String()

	// 获取当前用户ID
	if userID, exists := c.Get("userID"); exists {
		systemUser.CreatedBy = userID.(string)
	}

	if err := h.repo.Create(&systemUser); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create system user",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "System user created successfully",
		Data:    systemUser,
	})
}

// UpdateSystemUser 更新系统用户
// @Summary 更新系统用户
// @Tags system-users
// @Accept json
// @Produce json
// @Param id path string true "System User ID"
// @Param systemUser body model.SystemUser true "System User"
// @Success 200 {object} model.Response
// @Router /api/system-users/{id} [put]
func (h *SystemUserHandler) UpdateSystemUser(c *gin.Context) {
	id := c.Param("id")

	var systemUser model.SystemUser
	if err := c.ShouldBindJSON(&systemUser); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 验证认证方式和对应的认证信息
	if systemUser.AuthType != "password" && systemUser.AuthType != "key" && systemUser.AuthType != "both" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid auth type. Must be 'password', 'key', or 'both'",
			Data:    nil,
		})
		return
	}

	if systemUser.AuthType == "password" && systemUser.Password == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Password is required when auth type is 'password'",
			Data:    nil,
		})
		return
	}
	
	if systemUser.AuthType == "both" && systemUser.Password == "" && systemUser.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "At least password or private key is required when auth type is 'both'",
			Data:    nil,
		})
		return
	}

	if systemUser.AuthType == "key" && systemUser.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Private key is required when auth type is 'key'",
			Data:    nil,
		})
		return
	}

	systemUser.ID = id

	if err := h.repo.Update(&systemUser); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update system user",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "System user updated successfully",
		Data:    systemUser,
	})
}

// DeleteSystemUser 删除系统用户
// @Summary 删除系统用户
// @Tags system-users
// @Produce json
// @Param id path string true "System User ID"
// @Success 200 {object} model.Response
// @Router /api/system-users/{id} [delete]
func (h *SystemUserHandler) DeleteSystemUser(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete system user",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "System user deleted successfully",
		Data:    nil,
	})
}

// GetAvailableSystemUsers 获取用户可用的系统用户列表（用于登录前选择）
// @Summary 获取用户可用的系统用户列表
// @Tags system-users
// @Produce json
// @Param hostId query string true "Host ID"
// @Success 200 {object} model.Response
// @Router /api/system-users/available [get]
func (h *SystemUserHandler) GetAvailableSystemUsers(c *gin.Context) {
	hostID := c.Query("hostId")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Missing hostId parameter",
			Data:    nil,
		})
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Data:    nil,
		})
		return
	}

	systemUsers, err := h.repo.GetAvailableSystemUsersForUser(userID.(string), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch available system users",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    systemUsers,
	})
}

// CheckPermission 检查用户是否有权限使用指定系统用户
// @Summary 检查用户权限
// @Tags system-users
// @Produce json
// @Param hostId query string true "Host ID"
// @Param systemUserId query string true "System User ID"
// @Success 200 {object} model.Response
// @Router /api/system-users/check-permission [get]
func (h *SystemUserHandler) CheckPermission(c *gin.Context) {
	hostID := c.Query("hostId")
	systemUserID := c.Query("systemUserId")

	if hostID == "" || systemUserID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Missing parameters",
			Data:    nil,
		})
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, model.Response{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Data:    nil,
		})
		return
	}

	hasPermission, err := h.repo.CheckUserHasPermission(userID.(string), hostID, systemUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to check permission",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data: map[string]interface{}{
			"hasPermission": hasPermission,
		},
	})
}
