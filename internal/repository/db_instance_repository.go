package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type DBInstanceRepository struct {
	db *gorm.DB
}

func NewDBInstanceRepository(db *gorm.DB) *DBInstanceRepository {
	return &DBInstanceRepository{db: db}
}

// Create 创建数据库实例
func (r *DBInstanceRepository) Create(instance *model.DBInstance) error {
	return r.db.Create(instance).Error
}

// Update 更新数据库实例
func (r *DBInstanceRepository) Update(instance *model.DBInstance) error {
	return r.db.Model(instance).Omit("created_at").Updates(instance).Error
}

// Delete 删除数据库实例
func (r *DBInstanceRepository) Delete(id uint) error {
	return r.db.Delete(&model.DBInstance{}, "id = ?", id).Error
}

// GetByID 根据ID获取实例
func (r *DBInstanceRepository) GetByID(id uint) (*model.DBInstance, error) {
	var instance model.DBInstance
	err := r.db.Where("id = ?", id).First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// ExistsByName 检查名称是否已存在。excludeID 不为 nil 时排除该 ID（用于更新时允许原名）
func (r *DBInstanceRepository) ExistsByName(name string, excludeID *uint) (bool, error) {
	var count int64
	query := r.db.Model(&model.DBInstance{}).Where("name = ?", name)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// List 获取实例列表
func (r *DBInstanceRepository) List(offset, limit int, filters map[string]interface{}) ([]model.DBInstance, int64, error) {
	var instances []model.DBInstance
	var total int64

	query := r.db.Model(&model.DBInstance{})

	// 应用过滤条件
	if dbType, ok := filters["db_type"].(string); ok && dbType != "" {
		query = query.Where("db_type = ?", dbType)
	}
	if isEnabled, ok := filters["is_enabled"].(bool); ok {
		query = query.Where("is_enabled = ?", isEnabled)
	}
	if name, ok := filters["name"].(string); ok && name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取列表
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&instances).Error
	return instances, total, err
}

// TestConnection 测试连接（不保存到数据库）
func (r *DBInstanceRepository) TestConnection(instance *model.DBInstance) error {
	// 这里只做基本验证，实际连接测试在 service 层实现
	if instance.Host == "" {
		return gorm.ErrRecordNotFound
	}
	return nil
}
