package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type APIRepository interface {
	Create(api *model.API) error
	Update(api *model.API) error
	Delete(id uint) error
	FindByID(id uint) (*model.API, error)
	FindAll() ([]model.API, error)
	FindByGroup(group string) ([]model.API, error)
	FindByPathAndMethod(path, method string) (*model.API, error)
	GetGroups() ([]string, error)
}

type apiRepository struct {
	db *gorm.DB
}

func NewAPIRepository(db *gorm.DB) APIRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) Create(api *model.API) error {
	return r.db.Create(api).Error
}

func (r *apiRepository) Update(api *model.API) error {
	return r.db.Model(&model.API{}).
		Where("id = ?", api.ID).
		Omit("created_at").
		Updates(api).Error
}

func (r *apiRepository) Delete(id uint) error {
	return r.db.Delete(&model.API{}, "id = ?", id).Error
}

func (r *apiRepository) FindByID(id uint) (*model.API, error) {
	var api model.API
	err := r.db.Where("id = ?", id).First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) FindAll() ([]model.API, error) {
	var apis []model.API
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Order(groupColumn + " ASC, path ASC, method ASC").Find(&apis).Error
	return apis, err
}

func (r *apiRepository) FindByGroup(group string) ([]model.API, error) {
	var apis []model.API
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Where(groupColumn+" = ?", group).Order("path ASC, method ASC").Find(&apis).Error
	return apis, err
}

func (r *apiRepository) FindByPathAndMethod(path, method string) (*model.API, error) {
	var api model.API
	err := r.db.Where("path = ? AND method = ?", path, method).First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) GetGroups() ([]string, error) {
	var groups []string
	groupColumn := "`group`"
	if r.db.Dialector.Name() == "postgres" {
		groupColumn = "\"group\""
	}
	err := r.db.Model(&model.API{}).
		Select("DISTINCT "+groupColumn).
		Order(groupColumn+" ASC").
		Pluck(groupColumn, &groups).Error
	return groups, err
}
