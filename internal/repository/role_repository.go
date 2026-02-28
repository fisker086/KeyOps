package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// Create 创建角色
func (r *RoleRepository) Create(role *model.Role) error {
	return r.db.Create(role).Error
}

// Update 更新角色
func (r *RoleRepository) Update(role *model.Role) error {
	// 使用 Updates 并排除 created_at 和 created_by 字段，避免零值覆盖
	return r.db.Model(&model.Role{}).
		Where("id = ?", role.ID).
		Omit("created_at", "created_by").
		Updates(role).Error
}

// Delete 删除角色
func (r *RoleRepository) Delete(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 删除角色成员关系
		if err := tx.Delete(&model.RoleMember{}, "role_id = ?", id).Error; err != nil {
			return err
		}
		// 删除角色
		return tx.Delete(&model.Role{}, "id = ?", id).Error
	})
}

// FindByID 根据ID查找角色
func (r *RoleRepository) FindByID(id string) (*model.Role, error) {
	var role model.Role
	err := r.db.Where("id = ?", id).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindAll 查找所有角色
func (r *RoleRepository) FindAll() ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Order("priority DESC, created_at DESC").Find(&roles).Error
	return roles, err
}

// FindByStatus 根据状态查找角色
func (r *RoleRepository) FindByStatus(status string) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Where("status = ?", status).Order("priority DESC, created_at DESC").Find(&roles).Error
	return roles, err
}

// FindAllWithMembers 查找所有角色及其成员数
func (r *RoleRepository) FindAllWithMembers() ([]model.RoleWithMembers, error) {
	var roles []model.RoleWithMembers

	err := r.db.Table("roles").
		Select(`
			roles.*,
			COUNT(DISTINCT role_members.user_id) as member_count
		`).
		Joins("LEFT JOIN role_members ON role_members.role_id = roles.id").
		Group("roles.id").
		Order("roles.priority DESC, roles.created_at DESC").
		Scan(&roles).Error

	return roles, err
}

// FindByIDWithMembers 根据ID查找角色及其成员
func (r *RoleRepository) FindByIDWithMembers(id string) (*model.RoleWithMembers, error) {
	var role model.RoleWithMembers

	// 查找角色
	err := r.db.Where("id = ?", id).First(&role.Role).Error
	if err != nil {
		return nil, err
	}

	// 查找成员
	err = r.db.Table("users").
		Select("users.*").
		Joins("INNER JOIN role_members ON role_members.user_id = users.id").
		Where("role_members.role_id = ?", id).
		Order("users.username").
		Scan(&role.Members).Error
	if err != nil {
		return nil, err
	}

	role.MemberCount = len(role.Members)
	return &role, nil
}

// AddMember 添加成员到角色
func (r *RoleRepository) AddMember(roleID, userID, addedBy string) error {
	member := &model.RoleMember{
		RoleID:  roleID,
		UserID:  userID,
		AddedBy: addedBy,
	}
	return r.db.Create(member).Error
}

// RemoveMember 从角色移除成员
func (r *RoleRepository) RemoveMember(roleID, userID string) error {
	return r.db.Delete(&model.RoleMember{}, "role_id = ? AND user_id = ?", roleID, userID).Error
}

// GetMembersByRoleID 获取角色的所有成员
func (r *RoleRepository) GetMembersByRoleID(roleID string) ([]model.User, error) {
	var users []model.User

	err := r.db.Table("users").
		Select("users.*").
		Joins("INNER JOIN role_members ON role_members.user_id = users.id").
		Where("role_members.role_id = ?", roleID).
		Order("users.username").
		Find(&users).Error

	return users, err
}

// GetRolesByUserID 获取用户所在的所有角色
func (r *RoleRepository) GetRolesByUserID(userID string) ([]model.Role, error) {
	var roles []model.Role

	err := r.db.Table("roles").
		Select("roles.*").
		Joins("INNER JOIN role_members ON role_members.role_id = roles.id").
		Where("role_members.user_id = ?", userID).
		Where("roles.status = ?", "active").
		Order("roles.priority DESC, roles.created_at DESC").
		Find(&roles).Error

	return roles, err
}

// IsMember 检查用户是否是角色成员
func (r *RoleRepository) IsMember(roleID, userID string) (bool, error) {
	var count int64
	err := r.db.Table("role_members").
		Where("role_id = ? AND user_id = ?", roleID, userID).
		Count(&count).Error
	return count > 0, err
}

// BatchAddMembers 批量添加成员
func (r *RoleRepository) BatchAddMembers(roleID string, userIDs []string, addedBy string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, userID := range userIDs {
			member := &model.RoleMember{
				RoleID:  roleID,
				UserID:  userID,
				AddedBy: addedBy,
			}
			if err := tx.Create(member).Error; err != nil {
				// 忽略重复键错误
				if err.Error() != "UNIQUE constraint failed" {
					return err
				}
			}
		}
		return nil
	})
}

// BatchRemoveMembers 批量移除成员
func (r *RoleRepository) BatchRemoveMembers(roleID string, userIDs []string) error {
	return r.db.Delete(&model.RoleMember{}, "role_id = ? AND user_id IN ?", roleID, userIDs).Error
}
