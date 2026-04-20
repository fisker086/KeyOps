package permission

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoleHandler struct {
	repo *repository.RoleRepository
}

func NewRoleHandler(repo *repository.RoleRepository) *RoleHandler {
	return &RoleHandler{repo: repo}
}

// ListRoles 获取角色列表（统一从 roles 表获取，包括系统角色和自定义角色）
// @Summary 获取角色列表
// @Tags roles
// @Produce json
// @Param withMembers query boolean false "Include members"
// @Success 200 {object} model.Response
// @Router /api/roles [get]
func (h *RoleHandler) ListRoles(c *gin.Context) {
	withMembers := c.Query("withMembers") == "true"

	if withMembers {
		// 返回 RoleWithMembers，保留 memberCount（勿只取内嵌 Role，否则会丢失统计）
		rolesWithMembers, err := h.repo.FindAllWithMembers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{
				Code:    http.StatusInternalServerError,
				Message: "Failed to fetch roles",
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusOK,
			Message: "Success",
			Data:    rolesWithMembers,
		})
		return
	}

	roles, err := h.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch roles",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    roles,
	})
}

// GetRole 获取单个角色
// @Summary 获取单个角色
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Param withMembers query boolean false "Include members"
// @Success 200 {object} model.Response
// @Router /api/roles/{id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	id := c.Param("id")
	withMembers := c.Query("withMembers") == "true"

	if withMembers {
		role, err := h.repo.FindByIDWithMembers(id)
		if err != nil {
			c.JSON(http.StatusNotFound, model.Response{
				Code:    http.StatusNotFound,
				Message: "Role not found",
				Data:    nil,
			})
			return
		}

		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusOK,
			Message: "Success",
			Data:    role,
		})
	} else {
		role, err := h.repo.FindByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, model.Response{
				Code:    http.StatusNotFound,
				Message: "Role not found",
				Data:    nil,
			})
			return
		}

		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusOK,
			Message: "Success",
			Data:    role,
		})
	}
}

// CreateRole 创建角色
// @Summary 创建角色
// @Tags roles
// @Accept json
// @Produce json
// @Param role body model.Role true "Role"
// @Success 200 {object} model.Response
// @Router /api/roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 生成ID
	role.ID = uuid.New().String()

	// 获取当前用户ID
	if userID, exists := c.Get("userID"); exists {
		role.CreatedBy = userID.(string)
	}

	if err := h.repo.Create(&role); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create role",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Role created successfully",
		Data:    role,
	})
}

// UpdateRole 更新角色
// @Summary 更新角色
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param role body model.Role true "Role"
// @Success 200 {object} model.Response
// @Router /api/roles/{id} [put]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	id := c.Param("id")

	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	role.ID = id

	if err := h.repo.Update(&role); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update role",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Role updated successfully",
		Data:    role,
	})
}

// DeleteRole 删除角色
// @Summary 删除角色
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} model.Response
// @Router /api/roles/{id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete role",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Role deleted successfully",
		Data:    nil,
	})
}

// GetRoleMembers 获取角色成员
// @Summary 获取角色成员
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} model.Response
// @Router /api/roles/{id}/members [get]
func (h *RoleHandler) GetRoleMembers(c *gin.Context) {
	id := c.Param("id")

	members, err := h.repo.GetMembersByRoleID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch members",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    members,
	})
}

// AddRoleMember 添加用户到角色
// @Summary 添加用户到角色
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body map[string]string true "Request body with userId"
// @Success 200 {object} model.Response
// @Router /api/roles/{id}/members [post]
func (h *RoleHandler) AddRoleMember(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserID string `json:"userId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 获取当前用户ID（作为操作人）
	addedBy := ""
	if userID, exists := c.Get("userID"); exists {
		addedBy = userID.(string)
	}

	if err := h.repo.AddMember(id, req.UserID, addedBy); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to add member",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Member added successfully",
		Data:    nil,
	})
}

// RemoveRoleMember 从角色移除用户
// @Summary 从角色移除用户
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Param userId path string true "User ID"
// @Success 200 {object} model.Response
// @Router /api/roles/{id}/members/{userId} [delete]
func (h *RoleHandler) RemoveRoleMember(c *gin.Context) {
	id := c.Param("id")
	userID := c.Param("userId")

	if err := h.repo.RemoveMember(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to remove member",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Member removed successfully",
		Data:    nil,
	})
}

// BatchAddMembers 批量添加成员
// @Summary 批量添加成员
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body map[string][]string true "Request body with userIds"
// @Success 200 {object} model.Response
// @Router /api/roles/{id}/members/batch [post]
func (h *RoleHandler) BatchAddMembers(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 获取当前用户ID（作为操作人）
	addedBy := ""
	if userID, exists := c.Get("userID"); exists {
		addedBy = userID.(string)
	}

	if err := h.repo.BatchAddMembers(id, req.UserIDs, addedBy); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to add members",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Members added successfully",
		Data:    nil,
	})
}

// GetRoles 获取用户所在的所有角色
// @Summary 获取用户所在的所有角色
// @Tags roles
// @Produce json
// @Param userId query string true "User ID"
// @Success 200 {object} model.Response
// @Router /api/roles/by-user [get]
func (h *RoleHandler) GetRoles(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Missing userId parameter",
			Data:    nil,
		})
		return
	}

	roles, err := h.repo.GetRolesByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch roles",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    roles,
	})
}
