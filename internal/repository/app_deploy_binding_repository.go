package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApplicationDeployBindingRepository struct {
	db *gorm.DB
}

func NewApplicationDeployBindingRepository(db *gorm.DB) *ApplicationDeployBindingRepository {
	return &ApplicationDeployBindingRepository{db: db}
}

// Create 创建应用-发布绑定
func (r *ApplicationDeployBindingRepository) Create(binding *model.ApplicationDeployBinding) error {
	return r.db.Create(binding).Error
}

// Update 更新应用-发布绑定
func (r *ApplicationDeployBindingRepository) Update(binding *model.ApplicationDeployBinding) error {
	return r.db.Model(&model.ApplicationDeployBinding{}).
		Where("id = ?", binding.ID).
		Omit("created_at").
		Updates(binding).Error
}

// Delete 删除应用-发布绑定
func (r *ApplicationDeployBindingRepository) Delete(id string) error {
	return r.db.Delete(&model.ApplicationDeployBinding{}, "id = ?", id).Error
}

// FindByID 根据ID查找应用-发布绑定
func (r *ApplicationDeployBindingRepository) FindByID(id string) (*model.ApplicationDeployBinding, error) {
	var binding model.ApplicationDeployBinding
	err := r.db.Where("id = ?", id).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// FindAll 查找所有应用-发布绑定（支持分页和筛选）
func (r *ApplicationDeployBindingRepository) FindAll(req *model.ListApplicationDeployBindingsRequest) ([]model.ApplicationDeployBindingInfo, int64, error) {
	var bindings []model.ApplicationDeployBinding
	var total int64

	query := r.db.Model(&model.ApplicationDeployBinding{})

	// 应用筛选
	if req.ApplicationID != "" {
		query = query.Where("application_id = ?", req.ApplicationID)
	}

	// 发布类型筛选
	if req.DeployType != "" {
		query = query.Where("deploy_type = ?", req.DeployType)
	}

	// 环境筛选
	if req.Environment != "" {
		query = query.Where("environment = ?", req.Environment)
	}

	// 启用状态筛选
	if req.Enabled != nil {
		query = query.Where("enabled = ?", *req.Enabled)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 查询数据
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&bindings).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为响应格式（包含应用名称）
	var result []model.ApplicationDeployBindingInfo
	for _, binding := range bindings {
		// 查询应用名称
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

// FindByApplicationAndDeployType 根据应用ID和发布类型查找绑定
func (r *ApplicationDeployBindingRepository) FindByApplicationAndDeployType(applicationID, deployType string) ([]model.ApplicationDeployBinding, error) {
	var bindings []model.ApplicationDeployBinding
	err := r.db.Where("application_id = ? AND deploy_type = ? AND enabled = ?", applicationID, deployType, true).
		Order("created_at DESC").Find(&bindings).Error
	return bindings, err
}

// FindByApplicationDeployTypeAndEnvironment 根据应用、发布类型、环境查找绑定（取第一个）
func (r *ApplicationDeployBindingRepository) FindByApplicationDeployTypeAndEnvironment(applicationID, deployType, environment string) (*model.ApplicationDeployBinding, error) {
	var binding model.ApplicationDeployBinding
	err := r.db.Where("application_id = ? AND deploy_type = ? AND enabled = ? AND environment = ?",
		applicationID, deployType, true, environment).
		Order("created_at DESC").First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetApplicationsForDeploy 获取可用于发布的应用列表
func (r *ApplicationDeployBindingRepository) GetApplicationsForDeploy(req *model.GetApplicationsForDeployRequest) ([]model.Application, error) {
	var apps []model.Application

	// 查询有绑定关系的应用
	query := r.db.Model(&model.Application{}).
		Joins("INNER JOIN application_deploy_bindings ON applications.id = application_deploy_bindings.application_id").
		Where("application_deploy_bindings.deploy_type = ?", req.DeployType).
		Where("application_deploy_bindings.enabled = ?", true)

	// 环境筛选
	if req.Environment != "" {
		query = query.Where("application_deploy_bindings.environment = ?", req.Environment)
	}

	// 关键字搜索
	if req.Keyword != "" {
		query = query.Where("applications.name LIKE ?", "%"+req.Keyword+"%")
	}

	// 去重并排序
	err := query.Group("applications.id").Order("applications.updated_at DESC").Find(&apps).Error
	return apps, err
}

// CheckBindingExists 检查绑定关系是否已存在
func (r *ApplicationDeployBindingRepository) CheckBindingExists(applicationID, deployType, deployConfigID, environment string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ApplicationDeployBinding{}).
		Where("application_id = ? AND deploy_type = ? AND deploy_config_id = ? AND environment = ?",
			applicationID, deployType, deployConfigID, environment).
		Count(&count).Error
	return count > 0, err
}

// FindByWebhookToken 根据 webhook token 查找绑定（用于 Git 推送校验）
func (r *ApplicationDeployBindingRepository) FindByWebhookToken(token string) (*model.ApplicationDeployBinding, error) {
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

