package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository interface {
	CreateUser(user *model.User) error
	FindUserByUsername(username string) (*model.User, error)
	FindUserByEmail(email string) (*model.User, error)
	FindUserByID(id string) (*model.User, error)
	UpdateUser(user *model.User) error
	UpdateUserLastLogin(userID string, loginTime time.Time, loginIP string) error
	FindAllUsers() ([]model.User, error)
	CreatePlatformLoginRecord(record *model.PlatformLoginRecord) error
	FindPlatformLoginRecords(page, pageSize int, userID string) ([]model.PlatformLoginRecord, int64, error)
	UpdatePlatformLoginRecordLogout(recordID string) error
	UpdatePlatformLoginRecordLogoutByUser(userID string) error
	GetDB() *gorm.DB
	FindAllUsersWithPagination(page, pageSize int, keyword string) ([]model.User, int64, error)
	DeleteUser(userID string) error
	UpdateUserRole(userID, role string) error
	UpdateUserStatus(userID, status string) error
	AssignRolesToUser(userID string, roleIDs []string, createdBy string) error
	GetUserRoles(userID string) ([]string, error)
	GetUserWithGroups(userID string) (*model.UserWithGroups, error)
	FindAllUsersWithGroups(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error)
	RemoveUserFromGroup(userID, groupID string) error
	AddUserToGroup(userID, groupID, createdBy string) error
	GetUsersInGroup(groupID string) ([]model.User, error)
	AssignHostsToUser(userID string, hostIDs []string, createdBy string) error
	GetUserHosts(userID string) ([]string, error)
	GetUserHostGroupIDs(userID string) ([]string, error)
	AddUserToHost(userID, hostID, createdBy string) error
	RemoveUserFromHost(userID, hostID string) error
	GetUserWithGroupsAndHosts(userID string) (*model.UserWithGroups, error)
	FindAllUsersWithGroupsAndHosts(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error)
}

const directGrantRulePrefix = "_direct_grant_"

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// getUserPrimaryRole 获取用户的主角色（第一个角色的ID）
func (r *userRepository) getUserPrimaryRole(userID string) (string, error) {
	var member model.RoleMember
	if err := r.db.Where("user_id = ?", userID).Order("id ASC").First(&member).Error; err != nil {
		return "", err
	}
	return member.RoleID, nil
}

// ===== User Methods =====

func (r *userRepository) CreateUser(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindUserByUsername(username string) (*model.User, error) {
	var users []model.User
	result := r.db.Where("username = ?", username).Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	if len(users) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &users[0], nil
}

func (r *userRepository) FindUserByEmail(email string) (*model.User, error) {
	var users []model.User
	result := r.db.Where("email = ?", email).Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	if len(users) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &users[0], nil
}

func (r *userRepository) FindUserByID(id string) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) UpdateUser(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) UpdateUserLastLogin(userID string, loginTime time.Time, loginIP string) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"last_login_time": loginTime,
			"last_login_ip":   loginIP,
		}).Error
}

func (r *userRepository) FindAllUsers() ([]model.User, error) {
	var users []model.User
	err := r.db.Select("id, username, email, full_name, role, status, created_at").
		Where("status = ?", "active").
		Order("username ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// ===== Platform Login Record Methods =====

func (r *userRepository) CreatePlatformLoginRecord(record *model.PlatformLoginRecord) error {
	return r.db.Create(record).Error
}

func (r *userRepository) FindPlatformLoginRecords(page, pageSize int, userID string) ([]model.PlatformLoginRecord, int64, error) {
	var records []model.PlatformLoginRecord
	var total int64

	query := r.db.Model(&model.PlatformLoginRecord{})

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("login_time DESC").Find(&records).Error

	return records, total, err
}

func (r *userRepository) UpdatePlatformLoginRecordLogout(recordID string) error {
	return r.db.Model(&model.PlatformLoginRecord{}).
		Where("id = ? AND status = ?", recordID, "active").
		Updates(map[string]interface{}{
			"status": "logged_out",
		}).Error
}

func (r *userRepository) UpdatePlatformLoginRecordLogoutByUser(userID string) error {
	// 更新该用户最近的活跃登录记录
	return r.db.Model(&model.PlatformLoginRecord{}).
		Where("user_id = ? AND status = ?", userID, "active").
		Order("login_time DESC").
		Limit(1).
		Updates(map[string]interface{}{
			"status": "logged_out",
		}).Error
}

func (r *userRepository) GetDB() *gorm.DB {
	return r.db
}

// ===== User Management Methods =====

// FindAllUsersWithPagination 分页获取所有用户
func (r *userRepository) FindAllUsersWithPagination(page, pageSize int, keyword string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.Model(&model.User{})

	// 关键字搜索
	if keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ? OR full_name LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&users).Error

	return users, total, err
}

// DeleteUser 删除用户（软删除，设置status为inactive）
func (r *userRepository) DeleteUser(userID string) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Update("status", "inactive").Error
}

// UpdateUserRole 更新用户角色
func (r *userRepository) UpdateUserRole(userID, role string) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Update("role", role).Error
}

// UpdateUserStatus 更新用户状态
func (r *userRepository) UpdateUserStatus(userID, status string) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Update("status", status).Error
}

// ===== User-Group Permission Methods =====

// AssignRolesToUser 给用户分配角色（统一使用 role_members 表管理）
// 系统角色和自定义角色都统一在 roles 表中，通过 role_members 表关联
func (r *userRepository) AssignRolesToUser(userID string, roleIDs []string, createdBy string) error {
	// 先删除该用户现有的所有角色（包括系统角色和自定义角色）
	if err := r.db.Where("user_id = ?", userID).Delete(&model.RoleMember{}).Error; err != nil {
		return fmt.Errorf("删除现有角色失败: %w", err)
	}

	// 如果没有角色要分配，直接返回
	if len(roleIDs) == 0 {
		// 同时更新 users.role 字段为 'user'（向后兼容）
		if err := r.UpdateUserRole(userID, "user"); err != nil {
			return fmt.Errorf("更新用户角色失败: %w", err)
		}
		return nil
	}

	// 批量插入新角色成员关系（统一使用 role_members 表）
	members := make([]model.RoleMember, 0, len(roleIDs))
	hasAdminRole := false
	for _, roleID := range roleIDs {
		members = append(members, model.RoleMember{
			RoleID:  roleID,
			UserID:  userID,
			AddedBy: createdBy,
		})
		if roleID == "role:admin" {
			hasAdminRole = true
		}
	}

	if err := r.db.Create(&members).Error; err != nil {
		return fmt.Errorf("分配角色失败: %w", err)
	}

	// 同步更新 users.role 字段（向后兼容，用于快速查询）
	// 如果用户有 role:admin，则 users.role = 'admin'，否则为 'user'
	userRole := "user"
	if hasAdminRole {
		userRole = "admin"
	}
	if err := r.UpdateUserRole(userID, userRole); err != nil {
		return fmt.Errorf("更新用户角色失败: %w", err)
	}

	return nil
}

// GetUserRoles 获取用户有权限访问的角色ID列表（统一从 role_members 表获取）
// 系统角色和自定义角色都统一在 roles 表中，通过 role_members 表关联
func (r *userRepository) GetUserRoles(userID string) ([]string, error) {
	var roleMembers []model.RoleMember
	err := r.db.Where("user_id = ?", userID).Find(&roleMembers).Error
	if err != nil {
		return nil, err
	}

	roleIDs := make([]string, 0, len(roleMembers))
	for _, member := range roleMembers {
		roleIDs = append(roleIDs, member.RoleID)
	}

	return roleIDs, nil
}

// GetUserWithGroups 获取用户及其分组信息
func (r *userRepository) GetUserWithGroups(userID string) (*model.UserWithGroups, error) {
	// 获取用户信息
	user, err := r.FindUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 获取用户分组
	roleIDs, err := r.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	return &model.UserWithGroups{
		User:     *user,
		GroupIDs: roleIDs,
	}, nil
}

// FindAllUsersWithGroups 获取所有用户及其分组信息（分页）
func (r *userRepository) FindAllUsersWithGroups(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	// 获取用户列表
	users, total, err := r.FindAllUsersWithPagination(page, pageSize, keyword)
	if err != nil {
		return nil, 0, err
	}

	// 获取所有用户的分组信息
	usersWithGroups := make([]model.UserWithGroups, 0, len(users))
	for _, user := range users {
		roleIDs, err := r.GetUserRoles(user.ID)
		if err != nil {
			return nil, 0, err
		}

		usersWithGroups = append(usersWithGroups, model.UserWithGroups{
			User:     user,
			GroupIDs: roleIDs,
		})
	}

	return usersWithGroups, total, nil
}

// RemoveUserFromGroup 从分组中移除用户
func (r *userRepository) RemoveUserFromGroup(userID, groupID string) error {
	// 旧表
	if err := r.db.Where("user_id = ? AND group_id = ?", userID, groupID).
		Delete(&model.UserGroupPermission{}).Error; err != nil {
		return err
	}

	// 新表：删除由此迁移逻辑创建的授权规则
	roleID, err := r.getUserPrimaryRole(userID)
	if err == nil {
		ruleName := directGrantRulePrefix + userID + "_group_" + groupID
		r.db.Where("role_id = ? AND name = ?", roleID, ruleName).
			Delete(&model.PermissionRule{})
	}

	return nil
}

// AddUserToGroup 将用户添加到分组
func (r *userRepository) AddUserToGroup(userID, groupID, createdBy string) error {
	// 旧表
	permission := model.UserGroupPermission{
		UserID:    userID,
		GroupID:   groupID,
		CreatedBy: createdBy,
	}
	if err := r.db.Create(&permission).Error; err != nil {
		return err
	}

	// 新表：为用户的主角色创建授权规则（同时写入关联表）
	roleID, err := r.getUserPrimaryRole(userID)
	if err != nil {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		rule := &model.PermissionRule{
			ID:          uuid.New().String(),
			Name:        directGrantRulePrefix + userID + "_group_" + groupID,
			RoleID:      roleID,
			HostGroupID: &groupID,
			Enabled:     true,
			Description: fmt.Sprintf("Auto-migrated from user_group_permissions (user=%s, group=%s)", userID, groupID),
			CreatedBy:   createdBy,
		}
		if err := tx.Create(rule).Error; err != nil {
			return err
		}
		relation := &model.PermissionRuleHostGroup{
			PermissionRuleID: rule.ID,
			HostGroupID:      groupID,
		}
		return tx.Create(relation).Error
	})
}

// GetUsersInGroup 获取有权限访问某个分组的所有用户
func (r *userRepository) GetUsersInGroup(groupID string) ([]model.User, error) {
	userMap := make(map[string]model.User)

	// 1. 新架构：角色 → 授权规则 → 主机组（关联表 + 直列）
	type userIDResult struct {
		UserID string
	}
	var fromRules []userIDResult
	r.db.Table("permission_rules").
		Select("DISTINCT role_members.user_id").
		Joins("JOIN role_members ON permission_rules.role_id = role_members.role_id").
		Where("permission_rules.enabled = ? AND permission_rules.host_group_id = ?", true, groupID).
		Scan(&fromRules)

	var fromJoinTable []userIDResult
	r.db.Table("permission_rule_host_groups").
		Select("DISTINCT role_members.user_id").
		Joins("JOIN permission_rules ON permission_rule_host_groups.permission_rule_id = permission_rules.id").
		Joins("JOIN role_members ON permission_rules.role_id = role_members.role_id").
		Where("permission_rule_host_groups.host_group_id = ?", groupID).
		Where("permission_rules.enabled = ?", true).
		Scan(&fromJoinTable)

	fromRules = append(fromRules, fromJoinTable...)
	for _, row := range fromRules {
		if row.UserID != "" {
			user, err := r.FindUserByID(row.UserID)
			if err == nil {
				userMap[user.ID] = *user
			}
		}
	}

	// 2. 旧表 user_group_permissions
	var legacyUsers []model.User
	r.db.
		Joins("JOIN user_group_permissions ON users.id = user_group_permissions.user_id").
		Where("user_group_permissions.group_id = ?", groupID).
		Find(&legacyUsers)

	for _, u := range legacyUsers {
		userMap[u.ID] = u
	}

	out := make([]model.User, 0, len(userMap))
	for _, u := range userMap {
		out = append(out, u)
	}
	return out, nil
}

// ===== User-Host Permission Methods =====

// AssignHostsToUser 给用户分配单个主机权限
func (r *userRepository) AssignHostsToUser(userID string, hostIDs []string, createdBy string) error {
	// 旧表
	if err := r.db.Where("user_id = ?", userID).Delete(&model.UserHostPermission{}).Error; err != nil {
		return err
	}
	if len(hostIDs) > 0 {
		permissions := make([]model.UserHostPermission, 0, len(hostIDs))
		for _, hostID := range hostIDs {
			permissions = append(permissions, model.UserHostPermission{
				UserID:    userID,
				HostID:    hostID,
				CreatedBy: createdBy,
			})
		}
		if err := r.db.Create(&permissions).Error; err != nil {
			return err
		}
	}

	// 新表：为用户的主角色创建/更新授权规则
	roleID, err := r.getUserPrimaryRole(userID)
	if err != nil {
		return nil
	}

	ruleName := directGrantRulePrefix + userID + "_hosts"
	var existing model.PermissionRule
	if err := r.db.Where("role_id = ? AND name = ?", roleID, ruleName).First(&existing).Error; err == nil {
		if len(hostIDs) == 0 {
			return r.db.Delete(&existing).Error
		}
		hostIDsJSON, _ := json.Marshal(hostIDs)
		return r.db.Model(&existing).Update("host_ids", string(hostIDsJSON)).Error
	}

	if len(hostIDs) == 0 {
		return nil
	}

	hostIDsJSON, _ := json.Marshal(hostIDs)
	rule := &model.PermissionRule{
		ID:          uuid.New().String(),
		Name:        ruleName,
		RoleID:      roleID,
		HostIDs:     string(hostIDsJSON),
		Enabled:     true,
		Description: fmt.Sprintf("Auto-migrated from user_host_permissions (user=%s)", userID),
		CreatedBy:   createdBy,
	}
	return r.db.Create(rule).Error
}

// GetUserHosts 获取用户有权限访问的主机ID列表（单独授权的）
func (r *userRepository) GetUserHosts(userID string) ([]string, error) {
	hostIDMap := make(map[string]bool)

	// 1. 新架构：从 PermissionRule 的 HostIDs 字段读取
	roleID, err := r.getUserPrimaryRole(userID)
	if err == nil {
		ruleName := directGrantRulePrefix + userID + "_hosts"
		var rule model.PermissionRule
		if err := r.db.Where("role_id = ? AND name = ?", roleID, ruleName).First(&rule).Error; err == nil && rule.HostIDs != "" {
			var ids []string
			if err := json.Unmarshal([]byte(rule.HostIDs), &ids); err == nil {
				for _, id := range ids {
					hostIDMap[id] = true
				}
			}
		}
	}

	// 2. 旧表
	var permissions []model.UserHostPermission
	if err := r.db.Where("user_id = ?", userID).Find(&permissions).Error; err == nil {
		for _, p := range permissions {
			hostIDMap[p.HostID] = true
		}
	}

	out := make([]string, 0, len(hostIDMap))
	for id := range hostIDMap {
		out = append(out, id)
	}
	return out, nil
}

// GetUserHostGroupIDs 获取用户可访问的主机组 ID：授权规则（permission_rule_host_groups）+ 旧表 user_group_permissions
func (r *userRepository) GetUserHostGroupIDs(userID string) ([]string, error) {
	hostGroupIDMap := make(map[string]bool)

	// 1. 角色 → 有效授权规则 → permission_rule_host_groups
	var roleMembers []model.RoleMember
	err := r.db.Where("user_id = ?", userID).Find(&roleMembers).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	if len(roleMembers) > 0 {
		roleIDs := make([]string, 0, len(roleMembers))
		for _, member := range roleMembers {
			roleIDs = append(roleIDs, member.RoleID)
		}

		var permissionRules []struct {
			ID string `gorm:"column:id"`
		}
		now := time.Now()
		err = r.db.Table("permission_rules").
			Select("id").
			Where("role_id IN (?) AND enabled = ?", roleIDs, true).
			Where("(valid_from IS NULL OR valid_from <= ?) AND (valid_to IS NULL OR valid_to >= ?)", now, now).
			Find(&permissionRules).Error
		if err != nil {
			return nil, fmt.Errorf("failed to get permission rules: %w", err)
		}

		if len(permissionRules) > 0 {
			ruleIDs := make([]string, 0, len(permissionRules))
			for _, rule := range permissionRules {
				ruleIDs = append(ruleIDs, rule.ID)
			}

			// a) 从 permission_rule_host_groups 关联表读取
			var hostGroupRelations []struct {
				HostGroupID string `gorm:"column:host_group_id"`
			}
			err = r.db.Table("permission_rule_host_groups").
				Select("DISTINCT host_group_id").
				Where("permission_rule_id IN (?)", ruleIDs).
				Find(&hostGroupRelations).Error
			if err != nil {
				return nil, fmt.Errorf("failed to get host groups from rules: %w", err)
			}
			for _, rel := range hostGroupRelations {
				if rel.HostGroupID != "" {
					hostGroupIDMap[rel.HostGroupID] = true
				}
			}

			// b) 从 permission_rules.host_group_id 直列读取
			var directGroups []struct {
				HostGroupID string `gorm:"column:host_group_id"`
			}
			r.db.Table("permission_rules").
				Select("DISTINCT host_group_id").
				Where("id IN (?) AND host_group_id IS NOT NULL AND host_group_id != ''", ruleIDs).
				Scan(&directGroups)
			for _, dg := range directGroups {
				if dg.HostGroupID != "" {
					hostGroupIDMap[dg.HostGroupID] = true
				}
			}
		}
	}

	// 2. 旧版 user_group_permissions（无角色或未配规则时仍可能仅有此项）
	var legacyRows []model.UserGroupPermission
	if err := r.db.Where("user_id = ?", userID).Find(&legacyRows).Error; err != nil {
		return nil, fmt.Errorf("failed to get legacy user group permissions: %w", err)
	}
	for _, row := range legacyRows {
		if row.GroupID != "" {
			hostGroupIDMap[row.GroupID] = true
		}
	}

	out := make([]string, 0, len(hostGroupIDMap))
	for id := range hostGroupIDMap {
		out = append(out, id)
	}
	return out, nil
}

// AddUserToHost 将用户添加到单个主机权限
func (r *userRepository) AddUserToHost(userID, hostID, createdBy string) error {
	// 旧表
	permission := model.UserHostPermission{
		UserID:    userID,
		HostID:    hostID,
		CreatedBy: createdBy,
	}
	if err := r.db.Create(&permission).Error; err != nil {
		return err
	}

	// 新表：更新用户的个人主机授权规则（追加hostID）
	roleID, err := r.getUserPrimaryRole(userID)
	if err != nil {
		return nil
	}

	ruleName := directGrantRulePrefix + userID + "_hosts"
	var existing model.PermissionRule
	if err := r.db.Where("role_id = ? AND name = ?", roleID, ruleName).First(&existing).Error; err == nil {
		var ids []string
		if existing.HostIDs != "" {
			json.Unmarshal([]byte(existing.HostIDs), &ids)
		}
		for _, id := range ids {
			if id == hostID {
				return nil
			}
		}
		ids = append(ids, hostID)
		hostIDsJSON, _ := json.Marshal(ids)
		return r.db.Model(&existing).Update("host_ids", string(hostIDsJSON)).Error
	}

	hostIDsJSON, _ := json.Marshal([]string{hostID})
	rule := &model.PermissionRule{
		ID:          uuid.New().String(),
		Name:        ruleName,
		RoleID:      roleID,
		HostIDs:     string(hostIDsJSON),
		Enabled:     true,
		Description: fmt.Sprintf("Auto-migrated from user_host_permissions (user=%s)", userID),
		CreatedBy:   createdBy,
	}
	return r.db.Create(rule).Error
}

// RemoveUserFromHost 从主机移除用户权限
func (r *userRepository) RemoveUserFromHost(userID, hostID string) error {
	// 旧表
	if err := r.db.Where("user_id = ? AND host_id = ?", userID, hostID).
		Delete(&model.UserHostPermission{}).Error; err != nil {
		return err
	}

	// 新表：从个人主机授权规则中移除hostID
	roleID, err := r.getUserPrimaryRole(userID)
	if err != nil {
		return nil
	}

	ruleName := directGrantRulePrefix + userID + "_hosts"
	var existing model.PermissionRule
	if err := r.db.Where("role_id = ? AND name = ?", roleID, ruleName).First(&existing).Error; err != nil {
		return nil
	}

	var ids []string
	if existing.HostIDs != "" {
		json.Unmarshal([]byte(existing.HostIDs), &ids)
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != hostID {
			filtered = append(filtered, id)
		}
	}

	if len(filtered) == 0 {
		return r.db.Delete(&existing).Error
	}
	hostIDsJSON, _ := json.Marshal(filtered)
	return r.db.Model(&existing).Update("host_ids", string(hostIDsJSON)).Error
}

// GetUserWithGroupsAndHosts 获取用户及其分组和主机权限信息
func (r *userRepository) GetUserWithGroupsAndHosts(userID string) (*model.UserWithGroups, error) {
	// 获取用户信息
	user, err := r.FindUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 获取用户分组
	roleIDs, err := r.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	// 获取用户单独授权的主机
	hostIDs, err := r.GetUserHosts(userID)
	if err != nil {
		return nil, err
	}

	// Web 终端等与 SSH 一致：可访问的主机组来自授权规则（非 role id）
	hostGroupIDs, err := r.GetUserHostGroupIDs(userID)
	if err != nil {
		return nil, err
	}

	return &model.UserWithGroups{
		User:         *user,
		GroupIDs:     roleIDs,
		HostGroupIDs: hostGroupIDs,
		HostIDs:      hostIDs,
	}, nil
}

// FindAllUsersWithGroupsAndHosts 获取所有用户及其分组和主机信息（分页）
func (r *userRepository) FindAllUsersWithGroupsAndHosts(page, pageSize int, keyword string) ([]model.UserWithGroups, int64, error) {
	// 获取用户列表
	users, total, err := r.FindAllUsersWithPagination(page, pageSize, keyword)
	if err != nil {
		return nil, 0, err
	}

	// 获取所有用户的分组和主机信息
	usersWithPermissions := make([]model.UserWithGroups, 0, len(users))
	for _, user := range users {
		roleIDs, err := r.GetUserRoles(user.ID)
		if err != nil {
			return nil, 0, err
		}

		hostIDs, err := r.GetUserHosts(user.ID)
		if err != nil {
			return nil, 0, err
		}

		usersWithPermissions = append(usersWithPermissions, model.UserWithGroups{
			User:     user,
			GroupIDs: roleIDs,
			HostIDs:  hostIDs,
		})
	}

	return usersWithPermissions, total, nil
}
