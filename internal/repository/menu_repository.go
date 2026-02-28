package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type MenuRepository struct {
	db *gorm.DB
}

func NewMenuRepository(db *gorm.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

// Create 创建菜单
func (r *MenuRepository) Create(menu *model.Menu) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 如果设置此菜单为默认菜单，需要先将其他菜单的 defaultMenu 设置为 false
		if menu.Meta.DefaultMenu {
			if err := tx.Model(&model.Menu{}).
				Update("default_menu", false).Error; err != nil {
				return err
			}
		}
		// 创建菜单
		return tx.Create(menu).Error
	})
}

// Update 更新菜单
func (r *MenuRepository) Update(menu *model.Menu) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 如果设置此菜单为默认菜单，需要先将其他菜单的 defaultMenu 设置为 false
		if menu.Meta.DefaultMenu {
			if err := tx.Model(&model.Menu{}).
				Where("id != ?", menu.ID).
				Update("default_menu", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Menu{}).
			Where("id = ?", menu.ID).
			Omit("created_at").
			Updates(menu).Error
	})
}

// Delete 删除菜单
func (r *MenuRepository) Delete(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 删除菜单权限关联
		if err := tx.Delete(&model.MenuPermission{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		// 删除菜单
		return tx.Delete(&model.Menu{}, "id = ?", id).Error
	})
}

// FindByID 根据ID查找菜单
func (r *MenuRepository) FindByID(id string) (*model.Menu, error) {
	var menu model.Menu
	err := r.db.Where("id = ?", id).First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

// FindAll 查找所有菜单
func (r *MenuRepository) FindAll() ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Order("sort ASC, created_at ASC").Find(&menus).Error
	return menus, err
}

// FindByParentID 根据父菜单ID查找子菜单
func (r *MenuRepository) FindByParentID(parentID string) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Where("parent_id = ?", parentID).Order("sort ASC").Find(&menus).Error
	return menus, err
}

// deepCopyMenu 递归复制菜单及其所有子菜单
func (r *MenuRepository) deepCopyMenu(menu model.Menu) model.Menu {
	menuCopy := menu
	// 确保 Children 字段被正确复制
	if len(menu.Children) > 0 {
		menuCopy.Children = make([]model.Menu, len(menu.Children))
		for i := range menu.Children {
			// 递归复制每个子菜单
			menuCopy.Children[i] = r.deepCopyMenu(menu.Children[i])
		}
	} else {
		// 如果没有子菜单，初始化为空数组
		menuCopy.Children = []model.Menu{}
	}
	return menuCopy
}

// BuildMenuTree 构建菜单树
func (r *MenuRepository) BuildMenuTree(menus []model.Menu) []model.Menu {
	if len(menus) == 0 {
		return []model.Menu{}
	}

	menuMap := make(map[string]*model.Menu)
	var rootMenus []model.Menu

	// 第一遍遍历，创建菜单映射并初始化Children
	// 注意：使用指针引用，确保修改的是同一个对象
	for i := range menus {
		menus[i].Children = []model.Menu{} // 初始化Children字段
		menuMap[menus[i].ID] = &menus[i]
	}

	// 第二遍遍历，构建树结构
	// 先处理所有子菜单，将它们添加到父菜单的Children中
	for i := range menus {
		parentID := menus[i].ParentID
		// 空字符串或"0"表示顶级菜单
		if parentID == "" || parentID == "0" {
			// 顶级菜单，稍后添加到rootMenus
			continue
		} else {
			// 查找父菜单并添加子菜单
			if parent, ok := menuMap[parentID]; ok {
				// 使用 menuMap 中的指针引用添加子菜单，确保修改的是同一个对象
				if childPtr, ok := menuMap[menus[i].ID]; ok {
					parent.Children = append(parent.Children, *childPtr)
				} else {
					parent.Children = append(parent.Children, menus[i])
				}
			}
			// 如果父菜单不在menuMap中，说明父菜单没有权限，子菜单会被忽略
		}
	}

	// 第三遍遍历：重建所有父菜单的 Children slice
	// 因为 Children slice 存储的是值副本，当子菜单的 Children 被修改后，
	// 父菜单 Children slice 中的副本不会自动更新
	// 解决方案：重建每个父菜单的 Children slice，从 menuMap 中获取最新的子菜单数据
	for i := range menus {
		parentID := menus[i].ParentID
		if parentID != "" && parentID != "0" {
			if parent, ok := menuMap[parentID]; ok {
				// 重建 parent.Children slice，确保包含所有子菜单的最新数据
				parent.Children = []model.Menu{}
				// 遍历所有菜单，找到属于此父菜单的子菜单
				for j := range menus {
					if menus[j].ParentID == parentID {
						if childPtr, ok := menuMap[menus[j].ID]; ok {
							parent.Children = append(parent.Children, *childPtr)
						} else {
							parent.Children = append(parent.Children, menus[j])
						}
					}
				}
			}
		}
	}

	// 第四遍遍历，收集所有顶级菜单
	// 注意：必须使用 menuMap 中的指针引用，而不是 menus 数组中的值
	// 因为前面的遍历修改的是 menuMap 中的指针指向的对象
	// 但是，当我们解引用 *menuPtr 时，会创建一个新的副本，Children slice 会被浅复制
	// 为了确保 Children 被正确复制，我们需要使用 deepCopyMenu 递归复制整个菜单树
	for i := range menus {
		parentID := menus[i].ParentID
		if parentID == "" || parentID == "0" {
			// 使用 menuMap 中的指针引用，确保包含已修改的 Children
			if menuPtr, ok := menuMap[menus[i].ID]; ok {
				// 使用 deepCopyMenu 递归复制整个菜单树，确保所有嵌套的 Children 都被正确复制
				menuCopy := r.deepCopyMenu(*menuPtr)
				rootMenus = append(rootMenus, menuCopy)
			} else {
				rootMenus = append(rootMenus, menus[i])
			}
		}
	}

	// 递归排序每个根菜单的子菜单
	for i := range rootMenus {
		r.sortMenuChildren(&rootMenus[i])
	}

	return rootMenus
}

// sortMenuChildren 递归排序菜单及其子菜单
func (r *MenuRepository) sortMenuChildren(menu *model.Menu) {
	if len(menu.Children) > 0 {
		// 对子菜单按sort排序
		for i := 0; i < len(menu.Children)-1; i++ {
			for j := i + 1; j < len(menu.Children); j++ {
				if menu.Children[i].Sort > menu.Children[j].Sort {
					menu.Children[i], menu.Children[j] = menu.Children[j], menu.Children[i]
				}
			}
		}
		// 递归排序子菜单的子菜单
		for i := range menu.Children {
			r.sortMenuChildren(&menu.Children[i])
		}
	}
}

// GetMenusByUserGroupID 获取用户组有权限的菜单
func (r *MenuRepository) GetMenusByUserGroupID(userGroupID string) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Table("menus").
		Select("menus.*").
		Joins("INNER JOIN menu_permissions ON menu_permissions.menu_id = menus.id").
		Where("menu_permissions.role_id = ?", userGroupID).
		Order("menus.sort ASC, menus.created_at ASC").
		Find(&menus).Error
	return menus, err
}

// GetMenusByUserID 获取用户有权限的菜单（通过用户组）
// 如果用户有子菜单的权限，也会自动包含父菜单（即使父菜单没有直接权限）
func (r *MenuRepository) GetMenusByUserID(userID string) ([]model.Menu, error) {
	var menus []model.Menu
	// 先获取有权限的菜单
	err := r.db.Table("menus").
		Select("DISTINCT menus.*").
		Joins("INNER JOIN menu_permissions ON menu_permissions.menu_id = menus.id").
		Joins("INNER JOIN role_members ON role_members.role_id = menu_permissions.role_id").
		Where("role_members.user_id = ?", userID).
		Order("menus.sort ASC, menus.created_at ASC").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}

	// 递归收集所有需要包含的父菜单ID（包括祖父菜单等）
	parentIDs := make(map[string]bool)
	menuMap := make(map[string]bool)
	for _, menu := range menus {
		menuMap[menu.ID] = true
		// 递归收集所有祖先菜单ID
		currentParentID := menu.ParentID
		for currentParentID != "" && currentParentID != "0" {
			if !parentIDs[currentParentID] {
				parentIDs[currentParentID] = true
			}
			// 检查当前父菜单是否已经在menus中，如果是，继续查找它的父菜单
			var parentMenu model.Menu
			if err := r.db.Where("id = ?", currentParentID).First(&parentMenu).Error; err == nil {
				currentParentID = parentMenu.ParentID
			} else {
				break
			}
		}
	}

	// 如果存在父菜单，查询父菜单（即使没有直接权限）
	if len(parentIDs) > 0 {
		var parentMenuIDs []string
		for id := range parentIDs {
			parentMenuIDs = append(parentMenuIDs, id)
		}

		var parentMenus []model.Menu
		err = r.db.Where("id IN (?)", parentMenuIDs).
			Order("sort ASC, created_at ASC").
			Find(&parentMenus).Error
		if err == nil {
			// 将父菜单添加到结果的开头（去重），确保父菜单在子菜单之前
			// 将父菜单插入到开头，保持顺序
			var newMenus []model.Menu
			for _, parent := range parentMenus {
				if !menuMap[parent.ID] {
					newMenus = append(newMenus, parent)
					menuMap[parent.ID] = true
				}
			}
			// 将父菜单放在前面，然后是原有菜单
			menus = append(newMenus, menus...)
		}
	}

	return menus, nil
}

// GetMenusByRole 获取角色有权限的菜单（通过role）
// 如果用户有子菜单的权限，也会自动包含父菜单（即使父菜单没有直接权限）
func (r *MenuRepository) GetMenusByRole(role string) ([]model.Menu, error) {
	var menus []model.Menu
	roleID := "role:" + role
	
	// 先获取有权限的菜单（排除特殊标记 __empty__）
	err := r.db.Table("menus").
		Select("DISTINCT menus.*").
		Joins("INNER JOIN menu_permissions ON menu_permissions.menu_id = menus.id").
		Where("menu_permissions.role_id = ? AND menu_permissions.menu_id != ?", roleID, "__empty__").
		Order("menus.sort ASC, menus.created_at ASC").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}

	// 递归收集所有需要包含的父菜单ID（包括祖父菜单等）
	parentIDs := make(map[string]bool)
	menuMap := make(map[string]bool)
	for _, menu := range menus {
		menuMap[menu.ID] = true
		// 递归收集所有祖先菜单ID
		currentParentID := menu.ParentID
		for currentParentID != "" && currentParentID != "0" {
			if !parentIDs[currentParentID] {
				parentIDs[currentParentID] = true
			}
			// 检查当前父菜单是否已经在menus中，如果是，继续查找它的父菜单
			var parentMenu model.Menu
			if err := r.db.Where("id = ?", currentParentID).First(&parentMenu).Error; err == nil {
				currentParentID = parentMenu.ParentID
			} else {
				break
			}
		}
	}

	// 如果存在父菜单，查询父菜单（即使没有直接权限）
	if len(parentIDs) > 0 {
		var parentMenuIDs []string
		for id := range parentIDs {
			parentMenuIDs = append(parentMenuIDs, id)
		}

		var parentMenus []model.Menu
		err = r.db.Where("id IN (?)", parentMenuIDs).
			Order("sort ASC, created_at ASC").
			Find(&parentMenus).Error
		if err == nil {
			// 将父菜单添加到结果的开头（去重），确保父菜单在子菜单之前
			// 将父菜单插入到开头，保持顺序
			var newMenus []model.Menu
			for _, parent := range parentMenus {
				if !menuMap[parent.ID] {
					newMenus = append(newMenus, parent)
					menuMap[parent.ID] = true
				}
			}
			// 将父菜单放在前面，然后是原有菜单
			menus = append(newMenus, menus...)
		}
	}

	return menus, nil
}

// AddMenuPermission 添加菜单权限
func (r *MenuRepository) AddMenuPermission(userGroupID, menuID, createdBy string) error {
	permission := &model.MenuPermission{
		RoleID:    userGroupID,
		MenuID:    menuID,
		CreatedBy: createdBy,
	}
	return r.db.Create(permission).Error
}

// RemoveMenuPermission 移除菜单权限
func (r *MenuRepository) RemoveMenuPermission(userGroupID, menuID string) error {
	return r.db.Delete(&model.MenuPermission{}, "role_id = ? AND menu_id = ?", userGroupID, menuID).Error
}

// RemoveAllMenuPermissions 移除用户组的所有菜单权限
func (r *MenuRepository) RemoveAllMenuPermissions(userGroupID string) error {
	return r.db.Delete(&model.MenuPermission{}, "role_id = ?", userGroupID).Error
}

// GetMenuPermissionsByUserGroupID 获取用户组的菜单权限列表（排除特殊标记）
func (r *MenuRepository) GetMenuPermissionsByUserGroupID(userGroupID string) ([]string, error) {
	var menuIDs []string
	err := r.db.Table("menu_permissions").
		Select("menu_id").
		Where("role_id = ? AND menu_id != ?", userGroupID, "__empty__").
		Pluck("menu_id", &menuIDs).Error
	return menuIDs, err
}

// HasMenuPermissions 检查角色是否有菜单权限配置记录
func (r *MenuRepository) HasMenuPermissions(roleID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.MenuPermission{}).
		Where("role_id = ?", roleID).
		Count(&count).Error
	return count > 0, err
}

// BatchAddMenuPermissions 批量添加菜单权限
// 如果 menuIDs 为空，会插入一个特殊标记记录（menu_id = '__empty__'），表示用户已经配置过权限（即使为空）
// 这样可以区分"从未配置过权限"和"配置过但清空了权限"
func (r *MenuRepository) BatchAddMenuPermissions(userGroupID string, menuIDs []string, createdBy string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 先删除所有现有权限（包括特殊标记）
		if err := tx.Delete(&model.MenuPermission{}, "role_id = ?", userGroupID).Error; err != nil {
			return err
		}
		// 批量添加新权限
		if len(menuIDs) > 0 {
			// 有菜单权限，正常添加
			for _, menuID := range menuIDs {
				permission := &model.MenuPermission{
					RoleID:    userGroupID,
					MenuID:    menuID,
					CreatedBy: createdBy,
				}
				if err := tx.Create(permission).Error; err != nil {
					return err
				}
			}
		} else {
			// 没有菜单权限，插入特殊标记记录，表示用户已经配置过权限（即使为空）
			// 这样 GetMenusByRole 会返回空数组，HasMenuPermissions 会返回 true
			// GetUserMenus 就不会fallback到所有菜单
			permission := &model.MenuPermission{
				RoleID:    userGroupID,
				MenuID:    "__empty__", // 特殊标记，表示已配置但为空
				CreatedBy: createdBy,
			}
			if err := tx.Create(permission).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchUpdateSortOrder 批量更新菜单排序
// updates: map[menuID]sortOrder
func (r *MenuRepository) BatchUpdateSortOrder(updates map[string]int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for menuID, sortOrder := range updates {
			if err := tx.Model(&model.Menu{}).
				Where("id = ?", menuID).
				Update("sort", sortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
