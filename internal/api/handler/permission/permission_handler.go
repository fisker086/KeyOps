package permission

import (
	"fmt"
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/casbin"
	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	menuRepo *repository.MenuRepository
	apiRepo  *repository.APIRepository
	roleRepo *repository.RoleRepository
}

func NewPermissionHandler(
	menuRepo *repository.MenuRepository,
	apiRepo *repository.APIRepository,
	roleRepo *repository.RoleRepository,
) *PermissionHandler {
	return &PermissionHandler{
		menuRepo: menuRepo,
		apiRepo:  apiRepo,
		roleRepo: roleRepo,
	}
}

// ==================== 菜单管理 ====================

// ListMenus 获取所有菜单（树形结构）
func (h *PermissionHandler) ListMenus(c *gin.Context) {
	menus, err := h.menuRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取菜单列表失败: "+err.Error()))
		return
	}

	menuTree := h.menuRepo.BuildMenuTree(menus)
	c.JSON(http.StatusOK, model.Success(menuTree))
}

// CreateMenu 创建菜单
func (h *PermissionHandler) CreateMenu(c *gin.Context) {
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.menuRepo.Create(&menu); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "创建菜单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(menu))
}

// UpdateMenu 更新菜单
func (h *PermissionHandler) UpdateMenu(c *gin.Context) {
	id := c.Param("id")
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	menu.ID = id
	if err := h.menuRepo.Update(&menu); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新菜单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(menu))
}

// DeleteMenu 删除菜单
func (h *PermissionHandler) DeleteMenu(c *gin.Context) {
	id := c.Param("id")
	if err := h.menuRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "删除菜单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetUserMenus 获取当前用户的菜单（根据role、用户组权限）
func (h *PermissionHandler) GetUserMenus(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	role, _ := c.Get("role")
	roleStr := role.(string)

	// 获取所有菜单
	allMenus, err := h.menuRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取菜单列表失败: "+err.Error()))
		return
	}

	// 检查是否有role级别的菜单权限配置
	// 先检查数据库中是否有该role的权限配置记录
	hasPermissions, err := h.menuRepo.HasMenuPermissions("role:" + roleStr)
	if err == nil && hasPermissions {
		// 如果数据库中有该role的权限配置记录，使用role权限
		// 即使现在为空数组（用户取消了所有菜单），也返回空菜单，不fallback到所有菜单
		roleMenus, err := h.menuRepo.GetMenusByRole(roleStr)
		if err == nil {
			menuTree := h.menuRepo.BuildMenuTree(roleMenus)
			c.JSON(http.StatusOK, model.Success(menuTree))
			return
		}
	}

	// 如果没有role级别的配置，检查用户组权限
	userGroupMenus, err := h.menuRepo.GetMenusByUserID(userIDStr)
	if err == nil && len(userGroupMenus) > 0 {
		// 有用户组级别的菜单权限配置，使用用户组权限
		menuTree := h.menuRepo.BuildMenuTree(userGroupMenus)
		c.JSON(http.StatusOK, model.Success(menuTree))
		return
	}

	// 如果既没有role配置也没有用户组配置，根据角色返回默认菜单
	if roleStr == "admin" {
		// 管理员默认返回所有菜单
		menuTree := h.menuRepo.BuildMenuTree(allMenus)
		c.JSON(http.StatusOK, model.Success(menuTree))
		return
	}

	// 普通用户且没有配置权限，返回空菜单
	// 前端会显示授权提示页面
	c.JSON(http.StatusOK, model.Success([]model.Menu{}))
}

// GetMenuPermissionsByUserGroup 获取用户组的菜单权限（也支持role，如role:admin）
func (h *PermissionHandler) GetMenuPermissionsByUserGroup(c *gin.Context) {
	userGroupID := c.Param("id")

	menuIDs, err := h.menuRepo.GetMenuPermissionsByUserGroupID(userGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取菜单权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"menuIds": menuIDs}))
}

// GetMenuPermissionsByRole 获取角色的菜单权限
func (h *PermissionHandler) GetMenuPermissionsByRole(c *gin.Context) {
	role := c.Param("role")
	roleSubject := "role:" + role

	menuIDs, err := h.menuRepo.GetMenuPermissionsByUserGroupID(roleSubject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取菜单权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"menuIds": menuIDs}))
}

// UpdateMenuPermissionsByRole 更新角色的菜单权限
func (h *PermissionHandler) UpdateMenuPermissionsByRole(c *gin.Context) {
	role := c.Param("role")
	roleSubject := "role:" + role

	var req struct {
		MenuIDs []string `json:"menuIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	createdBy := userID.(string)

	if err := h.menuRepo.BatchAddMenuPermissions(roleSubject, req.MenuIDs, createdBy); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新菜单权限失败: "+err.Error()))
		return
	}

	// 重新加载Casbin策略
	if err := casbin.ReloadPolicy(); err != nil {
		// 记录警告但不影响返回结果
		_ = err
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// BatchUpdateMenuSortOrder 批量更新菜单排序
func (h *PermissionHandler) BatchUpdateMenuSortOrder(c *gin.Context) {
	var req struct {
		Updates map[string]int `json:"updates" binding:"required"` // map[menuID]sortOrder
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.menuRepo.BatchUpdateSortOrder(req.Updates); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新菜单排序失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// UpdateMenuPermissions 更新用户组的菜单权限
func (h *PermissionHandler) UpdateMenuPermissions(c *gin.Context) {
	userGroupID := c.Param("id")

	var req struct {
		MenuIDs []string `json:"menuIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	userID, _ := c.Get("userID")
	createdBy := userID.(string)

	if err := h.menuRepo.BatchAddMenuPermissions(userGroupID, req.MenuIDs, createdBy); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新菜单权限失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ==================== API管理 ====================

// ListAPIs 获取所有API
func (h *PermissionHandler) ListAPIs(c *gin.Context) {
	apis, err := h.apiRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取API列表失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(apis))
}

// CreateAPI 创建API
func (h *PermissionHandler) CreateAPI(c *gin.Context) {
	var api model.API
	if err := c.ShouldBindJSON(&api); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	if err := h.apiRepo.Create(&api); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "创建API失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(api))
}

// UpdateAPI 更新API
func (h *PermissionHandler) UpdateAPI(c *gin.Context) {
	id := c.Param("id")
	var api model.API
	if err := c.ShouldBindJSON(&api); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	var apiID uint
	if _, err := fmt.Sscanf(id, "%d", &apiID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的API ID"))
		return
	}

	api.ID = apiID
	if err := h.apiRepo.Update(&api); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "更新API失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(api))
}

// DeleteAPI 删除API
func (h *PermissionHandler) DeleteAPI(c *gin.Context) {
	id := c.Param("id")
	var apiID uint
	if _, err := fmt.Sscanf(id, "%d", &apiID); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "无效的API ID"))
		return
	}

	if err := h.apiRepo.Delete(apiID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "删除API失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetAPIGroups 获取所有API分组
func (h *PermissionHandler) GetAPIGroups(c *gin.Context) {
	groups, err := h.apiRepo.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取API分组失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(groups))
}

// ==================== API权限管理（Casbin） ====================

// GetAPIPermissionsByUserGroup 获取用户组的API权限（也支持role，如role:admin）
func (h *PermissionHandler) GetAPIPermissionsByUserGroup(c *gin.Context) {
	userGroupID := c.Param("id")

	// 从Casbin获取该用户组的策略
	policies, err := casbin.GetFilteredPolicy(0, userGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取API权限失败: "+err.Error()))
		return
	}

	// 转换为API权限列表
	var permissions []gin.H
	for _, policy := range policies {
		if len(policy) >= 3 {
			permissions = append(permissions, gin.H{
				"path":   policy[1],
				"method": policy[2],
			})
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"permissions": permissions}))
}

// GetAPIPermissionsByRole 获取角色的API权限
// 如果策略中包含通配符规则（/* 和 .*），前端需要特殊处理显示所有API
func (h *PermissionHandler) GetAPIPermissionsByRole(c *gin.Context) {
	role := c.Param("role")
	roleSubject := "role:" + role

	// 从Casbin获取该角色的策略
	policies, err := casbin.GetFilteredPolicy(0, roleSubject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取API权限失败: "+err.Error()))
		return
	}

	// 转换为API权限列表（直接返回策略中的路径和方法，包括通配符）
	var permissions []gin.H
	for _, policy := range policies {
		if len(policy) >= 3 {
			permissions = append(permissions, gin.H{
				"path":   policy[1],
				"method": policy[2],
			})
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"permissions": permissions}))
}

// UpdateAPIPermissionsByRole 更新角色的API权限
func (h *PermissionHandler) UpdateAPIPermissionsByRole(c *gin.Context) {
	role := c.Param("role")
	roleSubject := "role:" + role

	var req struct {
		Permissions []struct {
			Path   string `json:"path" binding:"required"`
			Method string `json:"method" binding:"required"`
		} `json:"permissions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	// 先删除该角色的所有现有策略
	_, err := casbin.RemoveFilteredPolicy(0, roleSubject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "清除旧权限失败: "+err.Error()))
		return
	}

	// 如果没有权限，直接返回
	if len(req.Permissions) == 0 {
		if err := casbin.ReloadPolicy(); err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "刷新权限缓存失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, model.Success(nil))
		return
	}

	// 获取所有API，判断是否选择了所有API
	allAPIs, err := h.apiRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "获取API列表失败: "+err.Error()))
		return
	}

	// 检查是否选择了所有API（用于判断是否使用通配符规则）
	selectedCount := len(req.Permissions)
	totalAPICount := len(allAPIs)

	// 如果选择的API数量等于所有API数量，使用通配符规则
	if selectedCount == totalAPICount {
		// 使用通配符规则：/* 匹配所有路径，.* 匹配所有方法
		rules := [][]string{
			{roleSubject, "/*", ".*"},
		}
		_, err = casbin.AddPolicies(rules)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "添加权限失败: "+err.Error()))
			return
		}
	} else {
		// 使用具体规则
		var rules [][]string
		for _, perm := range req.Permissions {
			rules = append(rules, []string{roleSubject, perm.Path, perm.Method})
		}

		// 批量添加新策略
		if len(rules) > 0 {
			_, err = casbin.AddPolicies(rules)
			if err != nil {
				c.JSON(http.StatusInternalServerError, model.Error(500, "添加权限失败: "+err.Error()))
				return
			}
		}
	}

	// 重新加载策略
	if err := casbin.ReloadPolicy(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "刷新权限缓存失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// UpdateAPIPermissions 更新用户组的API权限
func (h *PermissionHandler) UpdateAPIPermissions(c *gin.Context) {
	userGroupID := c.Param("id")

	var req struct {
		Permissions []struct {
			Path   string `json:"path" binding:"required"`
			Method string `json:"method" binding:"required"`
		} `json:"permissions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "参数错误: "+err.Error()))
		return
	}

	// 先删除该用户组的所有现有策略
	_, err := casbin.RemoveFilteredPolicy(0, userGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "清除旧权限失败: "+err.Error()))
		return
	}

	// 构建新的策略列表
	var rules [][]string
	for _, perm := range req.Permissions {
		rules = append(rules, []string{userGroupID, perm.Path, perm.Method})
	}

	// 批量添加新策略
	if len(rules) > 0 {
		_, err = casbin.AddPolicies(rules)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "添加权限失败: "+err.Error()))
			return
		}
	}

	// 重新加载策略
	if err := casbin.ReloadPolicy(); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "刷新权限缓存失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}
