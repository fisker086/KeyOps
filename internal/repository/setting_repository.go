package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// GetAll 获取所有设置
func (r *SettingRepository) GetAll() ([]model.Setting, error) {
	var settings []model.Setting
	err := r.db.Order("category ASC").
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}, Desc: false}).
		Find(&settings).Error
	return settings, err
}

// GetByCategory 根据分类获取设置
func (r *SettingRepository) GetByCategory(category string) ([]model.Setting, error) {
	var settings []model.Setting
	err := r.db.Where("category = ?", category).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}, Desc: false}).
		Find(&settings).Error
	return settings, err
}

// Get 根据key获取设置值（返回字符串）
func (r *SettingRepository) Get(key string) (string, error) {
	setting, err := r.GetByKey(key)
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil
	}
	return setting.Value, nil
}

// GetByKey 根据key获取设置
func (r *SettingRepository) GetByKey(key string) (*model.Setting, error) {
	var setting model.Setting
	err := r.db.Where(&model.Setting{Key: key}).First(&setting).Error
	if err != nil {
		// 如果记录不存在，返回 nil, nil（不报错）
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// GetSettingByKey 根据key获取设置（别名方法，保持兼容性）
func (r *SettingRepository) GetSettingByKey(key string) (*model.Setting, error) {
	return r.GetByKey(key)
}

// SetSetting 设置配置（别名方法，保持兼容性）
func (r *SettingRepository) SetSetting(setting *model.Setting) error {
	return r.Upsert(setting)
}

// Upsert 更新或插入设置
func (r *SettingRepository) Upsert(setting *model.Setting) error {
	// 先尝试查找
	var existing model.Setting
	err := r.db.Where(&model.Setting{Key: setting.Key}).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// 不存在，创建
		return r.db.Create(setting).Error
	} else if err != nil {
		return err
	}

	// 存在，更新
	return r.db.Model(&existing).Updates(map[string]interface{}{
		"value":    setting.Value,
		"category": setting.Category,
		"type":     setting.Type,
	}).Error
}

// BatchUpsert 批量更新或插入设置
func (r *SettingRepository) BatchUpsert(settings []model.Setting) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, setting := range settings {
			var existing model.Setting
			err := tx.Where(&model.Setting{Key: setting.Key}).First(&existing).Error

			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(&setting).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				if err := tx.Model(&existing).Updates(map[string]interface{}{
					"value":    setting.Value,
					"category": setting.Category,
					"type":     setting.Type,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Delete 删除设置
func (r *SettingRepository) Delete(key string) error {
	return r.db.Where(&model.Setting{Key: key}).Delete(&model.Setting{}).Error
}
