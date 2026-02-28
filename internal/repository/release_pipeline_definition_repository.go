package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ReleasePipelineDefinitionRepository struct {
	db *gorm.DB
}

func NewReleasePipelineDefinitionRepository(db *gorm.DB) *ReleasePipelineDefinitionRepository {
	return &ReleasePipelineDefinitionRepository{db: db}
}

func (r *ReleasePipelineDefinitionRepository) GetByID(id string) (*model.ReleasePipelineDefinition, error) {
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

func (r *ReleasePipelineDefinitionRepository) Save(def *model.ReleasePipelineDefinition) error {
	return r.db.Save(def).Error
}

func (r *ReleasePipelineDefinitionRepository) ListAll() ([]model.ReleasePipelineDefinition, error) {
	var list []model.ReleasePipelineDefinition
	err := r.db.Order("updated_at DESC").Find(&list).Error
	return list, err
}

func (r *ReleasePipelineDefinitionRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.ReleasePipelineDefinition{}).Error
}
