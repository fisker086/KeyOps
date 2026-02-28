package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type APIRepository struct {
	db *gorm.DB
}

func NewAPIRepository(db *gorm.DB) *APIRepository {
	return &APIRepository{db: db}
}

// Create 创建API
func (r *APIRepository) Create(api *model.API) error {
	return r.db.Create(api).Error
}

// Update 更新API
func (r *APIRepository) Update(api *model.API) error {
	return r.db.Model(&model.API{}).
		Where("id = ?", api.ID).
		Omit("created_at").
		Updates(api).Error
}

// Delete 删除API
func (r *APIRepository) Delete(id uint) error {
	return r.db.Delete(&model.API{}, "id = ?", id).Error
}

// FindByID 根据ID查找API
func (r *APIRepository) FindByID(id uint) (*model.API, error) {
	var api model.API
	err := r.db.Where("id = ?", id).First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

// FindAll 查找所有API
func (r *APIRepository) FindAll() ([]model.API, error) {
	var apis []model.API
	// 根据数据库类型使用正确的引号
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Order(groupColumn + " ASC, path ASC, method ASC").Find(&apis).Error
	return apis, err
}

// FindByGroup 根据分组查找API
func (r *APIRepository) FindByGroup(group string) ([]model.API, error) {
	var apis []model.API
	// 根据数据库类型使用正确的引号
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Where(groupColumn+" = ?", group).Order("path ASC, method ASC").Find(&apis).Error
	return apis, err
}

// FindByPathAndMethod 根据路径和方法查找API
func (r *APIRepository) FindByPathAndMethod(path, method string) (*model.API, error) {
	var api model.API
	err := r.db.Where("path = ? AND method = ?", path, method).First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

// GetGroups 获取所有API分组
func (r *APIRepository) GetGroups() ([]string, error) {
	var groups []string
	// 根据数据库类型使用正确的引号
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Model(&model.API{}).
		Select("DISTINCT " + groupColumn).
		Order(groupColumn + " ASC").
		Pluck(groupColumn, &groups).Error
	return groups, err
}

