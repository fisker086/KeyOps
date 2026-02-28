package repository

import (
	"encoding/json"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type SystemUserRepository struct {
	db *gorm.DB
}

func NewSystemUserRepository(db *gorm.DB) *SystemUserRepository {
	return &SystemUserRepository{db: db}
}

// getJSONContainsQuery 根据数据库类型返回 JSON 包含查询语句
func (r *SystemUserRepository) getJSONContainsQuery(column, value string) string {
	if r.db.Dialector.Name() == "postgres" {
		// PostgreSQL: 使用 JSONB @> 操作符
		return fmt.Sprintf("%s::jsonb @> jsonb_build_array(%s)", column, value)
	}
	// MySQL: 使用 JSON_CONTAINS 和 JSON_QUOTE
	return fmt.Sprintf("JSON_CONTAINS(%s, JSON_QUOTE(%s))", column, value)
}

// Create 创建系统用户
func (r *SystemUserRepository) Create(systemUser *model.SystemUser) error {
	return r.db.Create(systemUser).Error
}

// Update 更新系统用户
func (r *SystemUserRepository) Update(systemUser *model.SystemUser) error {
	// 使用 Updates 并排除 created_at 字段，避免零值覆盖
	return r.db.Model(&model.SystemUser{}).
		Where("id = ?", systemUser.ID).
		Omit("created_at").
		Updates(systemUser).Error
}

// Delete 删除系统用户
func (r *SystemUserRepository) Delete(id string) error {
	return r.db.Delete(&model.SystemUser{}, "id = ?", id).Error
}

// FindByID 根据ID查找系统用户
func (r *SystemUserRepository) FindByID(id string) (*model.SystemUser, error) {
	var systemUser model.SystemUser
	err := r.db.Where("id = ?", id).First(&systemUser).Error
	if err != nil {
		return nil, err
	}
	return &systemUser, nil
}

// FindAll 查找所有系统用户
func (r *SystemUserRepository) FindAll() ([]model.SystemUser, error) {
	var systemUsers []model.SystemUser
	err := r.db.Order("priority DESC, created_at DESC").Find(&systemUsers).Error
	return systemUsers, err
}

// FindByStatus 根据状态查找系统用户
func (r *SystemUserRepository) FindByStatus(status string) ([]model.SystemUser, error) {
	var systemUsers []model.SystemUser
	err := r.db.Where("status = ?", status).Order("priority DESC, created_at DESC").Find(&systemUsers).Error
	return systemUsers, err
}

// FindByProtocol 根据协议查找系统用户
func (r *SystemUserRepository) FindByProtocol(protocol string) ([]model.SystemUser, error) {
	var systemUsers []model.SystemUser
	err := r.db.Where("protocol = ? AND status = ?", protocol, "active").
		Order("priority DESC, created_at DESC").
		Find(&systemUsers).Error
	return systemUsers, err
}

// GetAvailableSystemUsersForUser 获取用户可用的系统用户列表（通过用户组和主机）
// GetAvailableSystemUsersForUser 获取用户可用的系统用户列表（新权限架构：多对多关系）
func (r *SystemUserRepository) GetAvailableSystemUsersForUser(userID, hostID string) ([]model.SystemUser, error) {
	var systemUsers []model.SystemUser

	now := time.Now()

	// 新查询逻辑（多对多关系）：
	// 1. 用户 → role_members → roles
	// 2. roles → permission_rules
	// 3. permission_rules → permission_rule_system_users → system_users
	// 4. permission_rules → permission_rule_host_groups → host_group_members → hosts
	query := r.db.Table("system_users").
		Select("DISTINCT system_users.*").
		// 连接：授权规则-系统用户关联表
		Joins("INNER JOIN permission_rule_system_users ON permission_rule_system_users.system_user_id = system_users.id").
		// 连接：授权规则
		Joins("INNER JOIN permission_rules ON permission_rules.id = permission_rule_system_users.permission_rule_id").
		// 连接：用户组（角色）
		Joins("INNER JOIN roles ON roles.id = permission_rules.role_id").
		// 连接：角色成员
		Joins("INNER JOIN role_members ON role_members.role_id = roles.id").
		// 用户过滤
		Where("role_members.user_id = ?", userID).
		// 状态过滤
		Where("system_users.status = ?", "active").
		Where("permission_rules.enabled = ?", true).
		Where("roles.status = ?", "active")

	// 时间范围检查
	query = query.Where("(permission_rules.valid_from IS NULL OR permission_rules.valid_from <= ?)", now)
	query = query.Where("(permission_rules.valid_to IS NULL OR permission_rules.valid_to >= ?)", now)

	// 主机范围检查（新逻辑：通过 permission_rule_host_groups 表）
	// 检查主机是否在授权规则关联的任意一个主机组中
	query = query.Where(`
		EXISTS (
			SELECT 1 FROM permission_rule_host_groups
			INNER JOIN host_group_members ON host_group_members.group_id = permission_rule_host_groups.host_group_id
			WHERE permission_rule_host_groups.permission_rule_id = permission_rules.id
			AND host_group_members.host_id = ?
		) OR
		(permission_rules.host_ids IS NOT NULL 
		 AND permission_rules.host_ids != '' 
		 AND %s)
	`, r.getJSONContainsQuery("permission_rules.host_ids", "?"), hostID, hostID)

	err := query.Order("system_users.priority DESC, system_users.created_at DESC").Find(&systemUsers).Error
	return systemUsers, err
}

// CheckUserHasPermission 检查用户是否有权限使用指定的系统用户访问主机（新权限架构：多对多关系）
func (r *SystemUserRepository) CheckUserHasPermission(userID, hostID, systemUserID string) (bool, error) {
	now := time.Now()

	var count int64
	// 新查询逻辑（多对多关系）：
	// 1. 用户 → role_members → roles
	// 2. roles → permission_rules
	// 3. permission_rules → permission_rule_system_users → system_users
	// 4. permission_rules → permission_rule_host_groups → host_group_members → hosts
	err := r.db.Table("permission_rules").
		// 连接：授权规则-系统用户关联表
		Joins("INNER JOIN permission_rule_system_users ON permission_rule_system_users.permission_rule_id = permission_rules.id").
		// 连接：用户组（角色）
		Joins("INNER JOIN roles ON roles.id = permission_rules.role_id").
		// 连接：角色成员
		Joins("INNER JOIN role_members ON role_members.role_id = roles.id").
		// 过滤条件
		Where("role_members.user_id = ?", userID).
		Where("permission_rule_system_users.system_user_id = ?", systemUserID).
		Where("permission_rules.enabled = ?", true).
		Where("roles.status = ?", "active").
		Where("(permission_rules.valid_from IS NULL OR permission_rules.valid_from <= ?)", now).
		Where("(permission_rules.valid_to IS NULL OR permission_rules.valid_to >= ?)", now).
		// 主机范围检查（新逻辑：通过 permission_rule_host_groups 表）
		Where(`
			EXISTS (
				SELECT 1 FROM permission_rule_host_groups
				INNER JOIN host_group_members ON host_group_members.group_id = permission_rule_host_groups.host_group_id
				WHERE permission_rule_host_groups.permission_rule_id = permission_rules.id
				AND host_group_members.host_id = ?
			) OR
			(permission_rules.host_ids IS NOT NULL 
			 AND permission_rules.host_ids != '' 
			 AND `+r.getJSONContainsQuery("permission_rules.host_ids", "?")+`)
		`, hostID, hostID).
		Count(&count).Error

	return count > 0, err
}

// PermissionRuleRepository 授权规则仓储
type PermissionRuleRepository struct {
	db *gorm.DB
}

func NewPermissionRuleRepository(db *gorm.DB) *PermissionRuleRepository {
	return &PermissionRuleRepository{db: db}
}

// Create 创建授权规则（支持多对多关系）
func (r *PermissionRuleRepository) Create(rule *model.PermissionRule) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 创建授权规则
		if err := tx.Create(rule).Error; err != nil {
			return err
		}

		// 2. 如果有系统用户关联，创建关联记录（从 system_user_id 字段）
		if rule.SystemUserID != nil && *rule.SystemUserID != "" {
			relation := &model.PermissionRuleSystemUser{
				PermissionRuleID: rule.ID,
				SystemUserID:     *rule.SystemUserID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		// 3. 如果有主机组关联，创建关联记录（从 host_group_id 字段）
		if rule.HostGroupID != nil && *rule.HostGroupID != "" {
			relation := &model.PermissionRuleHostGroup{
				PermissionRuleID: rule.ID,
				HostGroupID:      *rule.HostGroupID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// Update 更新授权规则（支持多对多关系）
func (r *PermissionRuleRepository) Update(rule *model.PermissionRule) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新授权规则基本信息
		if err := tx.Model(&model.PermissionRule{}).
			Where("id = ?", rule.ID).
			Omit("created_at", "created_by").
			Updates(rule).Error; err != nil {
			return err
		}

		// 2. 删除旧的系统用户关联
		if err := tx.Where("permission_rule_id = ?", rule.ID).
			Delete(&model.PermissionRuleSystemUser{}).Error; err != nil {
			return err
		}

		// 3. 创建新的系统用户关联
		if rule.SystemUserID != nil && *rule.SystemUserID != "" {
			relation := &model.PermissionRuleSystemUser{
				PermissionRuleID: rule.ID,
				SystemUserID:     *rule.SystemUserID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		// 4. 删除旧的主机组关联
		if err := tx.Where("permission_rule_id = ?", rule.ID).
			Delete(&model.PermissionRuleHostGroup{}).Error; err != nil {
			return err
		}

		// 5. 创建新的主机组关联
		if rule.HostGroupID != nil && *rule.HostGroupID != "" {
			relation := &model.PermissionRuleHostGroup{
				PermissionRuleID: rule.ID,
				HostGroupID:      *rule.HostGroupID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// CreateWithRelations 创建授权规则（支持多个系统用户和主机组）
func (r *PermissionRuleRepository) CreateWithRelations(rule *model.PermissionRule, systemUserIDs, hostGroupIDs []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 创建授权规则
		if err := tx.Create(rule).Error; err != nil {
			return err
		}

		// 2. 创建系统用户关联
		for _, systemUserID := range systemUserIDs {
			if systemUserID == "" {
				continue
			}
			relation := &model.PermissionRuleSystemUser{
				PermissionRuleID: rule.ID,
				SystemUserID:     systemUserID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		// 3. 创建主机组关联
		for _, hostGroupID := range hostGroupIDs {
			if hostGroupID == "" {
				continue
			}
			relation := &model.PermissionRuleHostGroup{
				PermissionRuleID: rule.ID,
				HostGroupID:      hostGroupID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateWithRelations 更新授权规则（支持多个系统用户和主机组）
func (r *PermissionRuleRepository) UpdateWithRelations(rule *model.PermissionRule, systemUserIDs, hostGroupIDs []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 更新授权规则基本信息
		if err := tx.Model(&model.PermissionRule{}).
			Where("id = ?", rule.ID).
			Omit("created_at", "created_by").
			Updates(rule).Error; err != nil {
			return err
		}

		// 2. 删除旧的系统用户关联
		if err := tx.Where("permission_rule_id = ?", rule.ID).
			Delete(&model.PermissionRuleSystemUser{}).Error; err != nil {
			return err
		}

		// 3. 创建新的系统用户关联
		for _, systemUserID := range systemUserIDs {
			if systemUserID == "" {
				continue
			}
			relation := &model.PermissionRuleSystemUser{
				PermissionRuleID: rule.ID,
				SystemUserID:     systemUserID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		// 4. 删除旧的主机组关联
		if err := tx.Where("permission_rule_id = ?", rule.ID).
			Delete(&model.PermissionRuleHostGroup{}).Error; err != nil {
			return err
		}

		// 5. 创建新的主机组关联
		for _, hostGroupID := range hostGroupIDs {
			if hostGroupID == "" {
				continue
			}
			relation := &model.PermissionRuleHostGroup{
				PermissionRuleID: rule.ID,
				HostGroupID:      hostGroupID,
			}
			if err := tx.Create(relation).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// Delete 删除授权规则
func (r *PermissionRuleRepository) Delete(id string) error {
	return r.db.Delete(&model.PermissionRule{}, "id = ?", id).Error
}

// FindByID 根据ID查找授权规则
func (r *PermissionRuleRepository) FindByID(id string) (*model.PermissionRule, error) {
	var rule model.PermissionRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// FindAll 查找所有授权规则（支持多对多关系）
func (r *PermissionRuleRepository) FindAll() ([]model.PermissionRuleDetail, error) {
	// 首先获取所有授权规则
	var rules []model.PermissionRule
	err := r.db.Order("priority DESC, created_at DESC").Find(&rules).Error
	if err != nil {
		return nil, err
	}

	// 构建详细信息
	details := make([]model.PermissionRuleDetail, 0, len(rules))
	for _, rule := range rules {
		detail := model.PermissionRuleDetail{
			PermissionRule:  rule,
			SystemUserIDs:   []string{},
			HostGroupIDs:    []string{},
			SystemUserNames: []string{},
			HostGroupNames:  []string{},
			SystemUsers:     []model.SystemUser{},
			HostGroups:      []model.HostGroup{},
		}

		// 获取用户组名称
		var role model.Role
		if err := r.db.Where("id = ?", rule.RoleID).First(&role).Error; err == nil {
			detail.RoleName = role.Name
		} else {
			// 如果角色查询失败，记录日志但不影响其他数据的返回
			// RoleName 保持为空，前端会回退显示 roleId
			logger.Warnf("[PermissionRuleRepository] Failed to find role with id %s: %v", rule.RoleID, err)
		}

		// 获取关联的系统用户（多对多）
		var systemUserRelations []model.PermissionRuleSystemUser
		if err := r.db.Where("permission_rule_id = ?", rule.ID).Find(&systemUserRelations).Error; err == nil {
			for _, rel := range systemUserRelations {
				detail.SystemUserIDs = append(detail.SystemUserIDs, rel.SystemUserID)

				var systemUser model.SystemUser
				if err := r.db.Where("id = ?", rel.SystemUserID).First(&systemUser).Error; err == nil {
					detail.SystemUsers = append(detail.SystemUsers, systemUser)
					detail.SystemUserNames = append(detail.SystemUserNames, systemUser.Name)

					// 兼容旧字段：设置第一个系统用户为默认
					if detail.SystemUserName == "" {
						detail.SystemUserName = systemUser.Name
					}
				}
			}
		}

		// 获取关联的主机组（多对多）
		var hostGroupRelations []model.PermissionRuleHostGroup
		if err := r.db.Where("permission_rule_id = ?", rule.ID).Find(&hostGroupRelations).Error; err == nil {
			for _, rel := range hostGroupRelations {
				detail.HostGroupIDs = append(detail.HostGroupIDs, rel.HostGroupID)

				var hostGroup model.HostGroup
				if err := r.db.Where("id = ?", rel.HostGroupID).First(&hostGroup).Error; err == nil {
					detail.HostGroups = append(detail.HostGroups, hostGroup)
					detail.HostGroupNames = append(detail.HostGroupNames, hostGroup.Name)

					// 兼容旧字段：设置第一个主机组为默认
					if detail.HostGroupName == "" {
						detail.HostGroupName = hostGroup.Name
					}
				}
			}
		}

		details = append(details, detail)
	}

	return details, nil
}

// FindByRole 根据角色查找授权规则
func (r *PermissionRuleRepository) FindByRole(roleID string) ([]model.PermissionRule, error) {
	var rules []model.PermissionRule
	err := r.db.Where("role_id = ?", roleID).
		Order("priority DESC, created_at DESC").
		Find(&rules).Error
	return rules, err
}

// FindByHostGroup 根据主机组查找授权规则
func (r *PermissionRuleRepository) FindByHostGroup(hostGroupID string) ([]model.PermissionRule, error) {
	var rules []model.PermissionRule
	err := r.db.Where("host_group_id = ?", hostGroupID).
		Order("priority DESC, created_at DESC").
		Find(&rules).Error
	return rules, err
}

// ValidateHostInRule 验证主机是否在授权规则范围内
func (r *PermissionRuleRepository) ValidateHostInRule(ruleID, hostID string) (bool, error) {
	var rule model.PermissionRule
	err := r.db.Where("id = ?", ruleID).First(&rule).Error
	if err != nil {
		return false, err
	}

	// 如果 host_group_id 为空，表示所有主机
	if (rule.HostGroupID == nil || *rule.HostGroupID == "") && rule.HostIDs == "" {
		return true, nil
	}

	// 检查主机是否在主机组中
	if rule.HostGroupID != nil && *rule.HostGroupID != "" {
		var count int64
		err = r.db.Table("host_group_members").
			Where("group_id = ? AND host_id = ?", *rule.HostGroupID, hostID).
			Count(&count).Error
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}

	// 检查主机是否在 host_ids 列表中
	if rule.HostIDs != "" {
		var hostIDs []string
		if err := json.Unmarshal([]byte(rule.HostIDs), &hostIDs); err == nil {
			for _, id := range hostIDs {
				if id == hostID {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
