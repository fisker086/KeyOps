package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type MonitorRepository struct {
	db *gorm.DB
}

func NewMonitorRepository(db *gorm.DB) *MonitorRepository {
	return &MonitorRepository{db: db}
}

// Create 创建监控查询语句
func (r *MonitorRepository) Create(monitor *model.Monitor) error {
	return r.db.Create(monitor).Error
}

// Update 更新监控查询语句
func (r *MonitorRepository) Update(monitor *model.Monitor) error {
	return r.db.Save(monitor).Error
}

// Delete 删除监控查询语句
func (r *MonitorRepository) Delete(id uint) error {
	return r.db.Delete(&model.Monitor{}, "id = ?", id).Error
}

// FindByID 根据ID查找
func (r *MonitorRepository) FindByID(id uint) (*model.Monitor, error) {
	var monitor model.Monitor
	err := r.db.Where("id = ?", id).First(&monitor).Error
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

// FindByName 根据名称查找
func (r *MonitorRepository) FindByName(name string) (*model.Monitor, error) {
	var monitor model.Monitor
	err := r.db.Where("name = ?", name).First(&monitor).Error
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

// List 获取监控列表（支持分页和搜索）
func (r *MonitorRepository) List(name string, page, pageSize int) (total int64, monitors []model.Monitor, err error) {
	query := r.db.Model(&model.Monitor{})

	// 名称搜索（模糊匹配）
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 先查询总数
	err = query.Count(&total).Error
	if err != nil {
		return
	}

	// 如果总数为0，直接返回空列表
	if total == 0 {
		return 0, []model.Monitor{}, nil
	}

	// 分页查询
	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	// 按创建时间倒序排列
	query = query.Order("created_at DESC")

	err = query.Find(&monitors).Error
	return
}

// Count 统计总数（用于分页）
func (r *MonitorRepository) Count(name string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Monitor{})

	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

