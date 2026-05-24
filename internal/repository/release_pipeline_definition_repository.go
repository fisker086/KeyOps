package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ReleasePipelineDefinitionRepository interface {
	GetByID(id string) (*model.ReleasePipelineDefinition, error)
	Save(def *model.ReleasePipelineDefinition) error
	ListAll() ([]model.ReleasePipelineDefinition, error)
	Delete(id string) error
}

type releasePipelineDefinitionRepository struct {
	db *gorm.DB
}

func NewReleasePipelineDefinitionRepository(db *gorm.DB) ReleasePipelineDefinitionRepository {
	return &releasePipelineDefinitionRepository{db: db}
}

func (r *releasePipelineDefinitionRepository) GetByID(id string) (*model.ReleasePipelineDefinition, error) {
	var def model.ReleasePipelineDefinition
	err := r.db.Where("id = ?", id).First(&def).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &def, nil
}

func (r *releasePipelineDefinitionRepository) Save(def *model.ReleasePipelineDefinition) error {
	return r.db.Save(def).Error
}

func (r *releasePipelineDefinitionRepository) ListAll() ([]model.ReleasePipelineDefinition, error) {
	var list []model.ReleasePipelineDefinition
	err := r.db.Order("updated_at DESC").Find(&list).Error
	return list, err
}

func (r *releasePipelineDefinitionRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.ReleasePipelineDefinition{}).Error
}
