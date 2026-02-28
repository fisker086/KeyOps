package bastion

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PermissionRuleHandler struct {
	repo *repository.PermissionRuleRepository
}

func NewPermissionRuleHandler(repo *repository.PermissionRuleRepository) *PermissionRuleHandler {
	return &PermissionRuleHandler{repo: repo}
}

// CreatePermissionRuleRequest 创建授权规则请求（支持多个系统用户和主机组）
type CreatePermissionRuleRequest struct {
	model.PermissionRule
	UserGroupID   string   `json:"userGroupId"`   // 用户组ID（前端字段名，映射到 RoleID）
	SystemUserIDs []string `json:"systemUserIds"`  // 多个系统用户ID
	HostGroupIDs  []string `json:"hostGroupIds"`   // 多个主机组ID
}

// ListPermissionRules 获取授权规则列表
// @Summary 获取授权规则列表
// @Tags permission-rules
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/permission-rules [get]
func (h *PermissionRuleHandler) ListPermissionRules(c *gin.Context) {
	rules, err := h.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch permission rules",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    rules,
	})
}

// GetPermissionRule 获取单个授权规则
// @Summary 获取单个授权规则
// @Tags permission-rules
// @Produce json
// @Param id path string true "Permission Rule ID"
// @Success 200 {object} model.Response
// @Router /api/permission-rules/{id} [get]
func (h *PermissionRuleHandler) GetPermissionRule(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "Permission rule not found",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    rule,
	})
}

// CreatePermissionRule 创建授权规则（支持多个系统用户和主机组）
// @Summary 创建授权规则
// @Tags permission-rules
// @Accept json
// @Produce json
// @Param rule body CreatePermissionRuleRequest true "Permission Rule"
// @Success 200 {object} model.Response
// @Router /api/permission-rules [post]
func (h *PermissionRuleHandler) CreatePermissionRule(c *gin.Context) {
	var req CreatePermissionRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	// 生成ID
	req.PermissionRule.ID = uuid.New().String()

	// 获取当前用户ID
	if userID, exists := c.Get("userID"); exists {
		req.PermissionRule.CreatedBy = userID.(string)
	}

	// 处理 userGroupId 映射到 RoleID（前端使用 userGroupId，后端使用 RoleID）
	if req.UserGroupID != "" {
		req.PermissionRule.RoleID = req.UserGroupID
	}
	
	// 验证 RoleID 不能为空
	if req.PermissionRule.RoleID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "用户组（角色）不能为空",
			Data:    nil,
		})
		return
	}

	// 处理空字符串指针：将指向空字符串的指针转换为 nil
	if req.PermissionRule.HostGroupID != nil && *req.PermissionRule.HostGroupID == "" {
		req.PermissionRule.HostGroupID = nil
	}
	if req.PermissionRule.SystemUserID != nil && *req.PermissionRule.SystemUserID == "" {
		req.PermissionRule.SystemUserID = nil
	}

	// 调用 repository 创建授权规则（支持多对多关系）
	if err := h.repo.CreateWithRelations(&req.PermissionRule, req.SystemUserIDs, req.HostGroupIDs); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create permission rule",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Permission rule created successfully",
		Data:    req.PermissionRule,
	})
}

// UpdatePermissionRule 更新授权规则（支持多个系统用户和主机组）
// @Summary 更新授权规则
// @Tags permission-rules
// @Accept json
// @Produce json
// @Param id path string true "Permission Rule ID"
// @Param rule body CreatePermissionRuleRequest true "Permission Rule"
// @Success 200 {object} model.Response
// @Router /api/permission-rules/{id} [put]
func (h *PermissionRuleHandler) UpdatePermissionRule(c *gin.Context) {
	id := c.Param("id")

	var req CreatePermissionRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Data:    nil,
		})
		return
	}

	req.PermissionRule.ID = id

	// 处理 userGroupId 映射到 RoleID（前端使用 userGroupId，后端使用 RoleID）
	if req.UserGroupID != "" {
		req.PermissionRule.RoleID = req.UserGroupID
	}
	
	// 验证 RoleID 不能为空
	if req.PermissionRule.RoleID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "用户组（角色）不能为空",
			Data:    nil,
		})
		return
	}

	// 处理空字符串指针：将指向空字符串的指针转换为 nil
	if req.PermissionRule.HostGroupID != nil && *req.PermissionRule.HostGroupID == "" {
		req.PermissionRule.HostGroupID = nil
	}
	if req.PermissionRule.SystemUserID != nil && *req.PermissionRule.SystemUserID == "" {
		req.PermissionRule.SystemUserID = nil
	}

	// 调用 repository 更新授权规则（支持多对多关系）
	if err := h.repo.UpdateWithRelations(&req.PermissionRule, req.SystemUserIDs, req.HostGroupIDs); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update permission rule",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Permission rule updated successfully",
		Data:    req.PermissionRule,
	})
}

// DeletePermissionRule 删除授权规则
// @Summary 删除授权规则
// @Tags permission-rules
// @Produce json
// @Param id path string true "Permission Rule ID"
// @Success 200 {object} model.Response
// @Router /api/permission-rules/{id} [delete]
func (h *PermissionRuleHandler) DeletePermissionRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete permission rule",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Permission rule deleted successfully",
		Data:    nil,
	})
}

// GetPermissionRulesByRole 获取角色的授权规则
// @Summary 获取角色的授权规则
// @Tags permission-rules
// @Produce json
// @Param roleId query string true "Role ID"
// @Success 200 {object} model.Response
// @Router /api/permission-rules/by-role [get]
func (h *PermissionRuleHandler) GetPermissionRulesByRole(c *gin.Context) {
	roleID := c.Query("roleId")
	if roleID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Missing roleId parameter",
			Data:    nil,
		})
		return
	}

	rules, err := h.repo.FindByRole(roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch permission rules",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    rules,
	})
}

// GetPermissionRulesByHostGroup 获取主机组的授权规则
// @Summary 获取主机组的授权规则
// @Tags permission-rules
// @Produce json
// @Param hostGroupId query string true "Host Group ID"
// @Success 200 {object} model.Response
// @Router /api/permission-rules/by-host-group [get]
func (h *PermissionRuleHandler) GetPermissionRulesByHostGroup(c *gin.Context) {
	hostGroupID := c.Query("hostGroupId")
	if hostGroupID == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Missing hostGroupId parameter",
			Data:    nil,
		})
		return
	}

	rules, err := h.repo.FindByHostGroup(hostGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch permission rules",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    rules,
	})
}
