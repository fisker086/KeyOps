package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApplicationDeployBindingRepository interface {
	Create(binding *model.ApplicationDeployBinding) error
	Update(binding *model.ApplicationDeployBinding) error
	Delete(id string) error
	FindByID(id string) (*model.ApplicationDeployBinding, error)
	FindAll(req *model.ListApplicationDeployBindingsRequest) ([]model.ApplicationDeployBindingInfo, int64, error)
	FindByApplicationAndDeployType(applicationID, deployType string) ([]model.ApplicationDeployBinding, error)
	FindByApplicationDeployTypeAndEnvironment(applicationID, deployType, environment string) (*model.ApplicationDeployBinding, error)
	GetApplicationsForDeploy(req *model.GetApplicationsForDeployRequest) ([]model.Application, error)
	CheckBindingExists(applicationID, deployType, deployConfigID, environment string) (bool, error)
	FindByWebhookToken(token string) (*model.ApplicationDeployBinding, error)
}

type applicationDeployBindingRepository struct {
	db *gorm.DB
}

func NewApplicationDeployBindingRepository(db *gorm.DB) ApplicationDeployBindingRepository {
	return &applicationDeployBindingRepository{db: db}
}

func (r *applicationDeployBindingRepository) Create(binding *model.ApplicationDeployBinding) error {
	return r.db.Create(binding).Error
}

func (r *applicationDeployBindingRepository) Update(binding *model.ApplicationDeployBinding) error {
	return r.db.Model(&model.ApplicationDeployBinding{}).
		Where("id = ?", binding.ID).
		Omit("created_at").
		Updates(binding).Error
}

func (r *applicationDeployBindingRepository) Delete(id string) error {
	return r.db.Delete(&model.ApplicationDeployBinding{}, "id = ?", id).Error
}

func (r *applicationDeployBindingRepository) FindByID(id string) (*model.ApplicationDeployBinding, error) {
	var binding model.ApplicationDeployBinding
	err := r.db.Where("id = ?", id).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *applicationDeployBindingRepository) FindAll(req *model.ListApplicationDeployBindingsRequest) ([]model.ApplicationDeployBindingInfo, int64, error) {
	var bindings []model.ApplicationDeployBinding
	var total int64

	query := r.db.Model(&model.ApplicationDeployBinding{})

	if req.ApplicationID != "" {
		query = query.Where("application_id = ?", req.ApplicationID)
	}

	if req.DeployType != "" {
		query = query.Where("deploy_type = ?", req.DeployType)
	}

	if req.Environment != "" {
		query = query.Where("environment = ?", req.Environment)
	}

	if req.Enabled != nil {
		query = query.Where("enabled = ?", *req.Enabled)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&bindings).Error
	if err != nil {
		return nil, 0, err
	}

	var result []model.ApplicationDeployBindingInfo
	for _, binding := range bindings {
		var app model.Application
		r.db.Select("name").Where("id = ?", binding.ApplicationID).First(&app)

		result = append(result, model.ApplicationDeployBindingInfo{
			ID:                binding.ID,
			ApplicationID:     binding.ApplicationID,
			ApplicationName:   app.Name,
			DeployType:        binding.DeployType,
			DeployConfigID:    binding.DeployConfigID,
			DeployConfigName:  binding.DeployConfigName,
			Environment:       binding.Environment,
			JenkinsJob:        binding.JenkinsJob,
			ArgoCDApplication: binding.ArgoCDApplication,
			DeployStrategy:    binding.DeployStrategy,
			StrategyOptions:   binding.StrategyOptions,
			PipelineID:        binding.PipelineID,
			WebhookToken:      binding.WebhookToken,
			Enabled:           binding.Enabled,
			Description:       binding.Description,
			CreatedBy:         binding.CreatedBy,
			CreatedAt:         binding.CreatedAt,
			UpdatedAt:         binding.UpdatedAt,
		})
	}

	return result, total, nil
}

func (r *applicationDeployBindingRepository) FindByApplicationAndDeployType(applicationID, deployType string) ([]model.ApplicationDeployBinding, error) {
	var bindings []model.ApplicationDeployBinding
	err := r.db.Where("application_id = ? AND deploy_type = ? AND enabled = ?", applicationID, deployType, true).
		Order("created_at DESC").Find(&bindings).Error
	return bindings, err
}

func (r *applicationDeployBindingRepository) FindByApplicationDeployTypeAndEnvironment(applicationID, deployType, environment string) (*model.ApplicationDeployBinding, error) {
	var binding model.ApplicationDeployBinding
	err := r.db.Where("application_id = ? AND deploy_type = ? AND enabled = ? AND environment = ?",
		applicationID, deployType, true, environment).
		Order("created_at DESC").First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *applicationDeployBindingRepository) GetApplicationsForDeploy(req *model.GetApplicationsForDeployRequest) ([]model.Application, error) {
	var apps []model.Application

	query := r.db.Model(&model.Application{}).
		Joins("INNER JOIN application_deploy_bindings ON applications.id = application_deploy_bindings.application_id").
		Where("application_deploy_bindings.deploy_type = ?", req.DeployType).
		Where("application_deploy_bindings.enabled = ?", true)

	if req.Environment != "" {
		query = query.Where("application_deploy_bindings.environment = ?", req.Environment)
	}

	if req.Keyword != "" {
		query = query.Where("applications.name LIKE ?", "%"+req.Keyword+"%")
	}

	err := query.Group("applications.id").Order("applications.updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationDeployBindingRepository) CheckBindingExists(applicationID, deployType, deployConfigID, environment string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ApplicationDeployBinding{}).
		Where("application_id = ? AND deploy_type = ? AND deploy_config_id = ? AND environment = ?",
			applicationID, deployType, deployConfigID, environment).
		Count(&count).Error
	return count > 0, err
}

func (r *applicationDeployBindingRepository) FindByWebhookToken(token string) (*model.ApplicationDeployBinding, error) {
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var b model.ApplicationDeployBinding
	err := r.db.Where("webhook_token = ? AND enabled = ?", token, true).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}
