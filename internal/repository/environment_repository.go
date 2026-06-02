package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type EnvironmentRepository interface {
	Create(env *model.Environment) error
	Update(env *model.Environment) error
	Delete(id string) error
	FindByID(id string) (*model.Environment, error)
	FindByCode(code string) (*model.Environment, error)
	FindAll() ([]model.Environment, error)
	CheckCodeExists(code string, excludeID string) (bool, error)
}

type environmentRepository struct {
	db *gorm.DB
}

func NewEnvironmentRepository(db *gorm.DB) EnvironmentRepository {
	return &environmentRepository{db: db}
}

func (r *environmentRepository) Create(env *model.Environment) error {
	return r.db.Create(env).Error
}

func (r *environmentRepository) Update(env *model.Environment) error {
	return r.db.Model(&model.Environment{}).
		Where("id = ?", env.ID).
		Omit("created_at").
		Updates(env).Error
}

func (r *environmentRepository) Delete(id string) error {
	return r.db.Delete(&model.Environment{}, "id = ?", id).Error
}

func (r *environmentRepository) FindByID(id string) (*model.Environment, error) {
	var env model.Environment
	if err := r.db.Where("id = ?", id).First(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *environmentRepository) FindByCode(code string) (*model.Environment, error) {
	var env model.Environment
	if err := r.db.Where("code = ?", code).First(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *environmentRepository) FindAll() ([]model.Environment, error) {
	var envs []model.Environment
	err := r.db.Order("sort_order ASC, created_at ASC").Find(&envs).Error
	return envs, err
}

func (r *environmentRepository) CheckCodeExists(code string, excludeID string) (bool, error) {
	var count int64
	query := r.db.Model(&model.Environment{}).Where("code = ?", code)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
