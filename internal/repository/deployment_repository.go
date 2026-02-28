package repository

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type DeploymentRepository struct {
	db *gorm.DB
}

func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

// Create 创建部署记录
func (r *DeploymentRepository) Create(deployment *model.Deployment) error {
	return r.db.Create(deployment).Error
}

// GetByID 根据ID获取部署记录
func (r *DeploymentRepository) GetByID(id string) (*model.Deployment, error) {
	var deployment model.Deployment
	err := r.db.Where("id = ?", id).First(&deployment).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// Update 更新部署记录
func (r *DeploymentRepository) Update(deployment *model.Deployment) error {
	return r.db.Save(deployment).Error
}

// UpdateStatus 更新部署状态
func (r *DeploymentRepository) UpdateStatus(id string, status string, duration *int, logPath string) error {
	// 先获取当前记录，检查状态变化
	var current model.Deployment
	if err := r.db.Where("id = ?", id).First(&current).Error; err != nil {
		return err
	}
	
	updates := map[string]interface{}{
		"status": status,
	}
	
	if duration != nil {
		updates["duration"] = *duration
	}
	if logPath != "" {
		updates["log_path"] = logPath
	}
	
	now := time.Now()
	// 只在状态从非 running 变为 running 时设置 started_at
	if status == model.DeploymentStatusRunning && current.Status != model.DeploymentStatusRunning {
		updates["started_at"] = &now
	}
	
	// 在状态变为最终状态时设置 completed_at
	if status == model.DeploymentStatusSuccess || status == model.DeploymentStatusFailed || status == model.DeploymentStatusCancelled {
		updates["completed_at"] = &now
		// 如果还没有 started_at，且提供了 duration，则计算 started_at
		if current.StartedAt == nil && duration != nil && *duration > 0 {
			startTime := now.Add(-time.Duration(*duration) * time.Second)
			updates["started_at"] = &startTime
		}
	}
	
	return r.db.Model(&model.Deployment{}).Where("id = ?", id).Updates(updates).Error
}

// List 查询部署记录列表
func (r *DeploymentRepository) List(params *DeploymentListParams) ([]*model.Deployment, int64, error) {
	var deployments []*model.Deployment
	var total int64
	
	query := r.db.Model(&model.Deployment{})
	
	// 构建查询条件
	if params.ProjectID != "" {
		query = query.Where("project_id = ?", params.ProjectID)
	}
	if params.ProjectName != "" {
		query = query.Where("project_name LIKE ?", "%"+params.ProjectName+"%")
	}
	if params.EnvID != "" {
		query = query.Where("env_id = ?", params.EnvID)
	}
	if params.ClusterID != "" {
		query = query.Where("cluster_id = ?", params.ClusterID)
	}
	if params.DeployType != "" {
		query = query.Where("deploy_type = ?", params.DeployType)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.CreatedBy != "" {
		query = query.Where("created_by = ?", params.CreatedBy)
	}
	if params.StartTime != nil {
		query = query.Where("created_at >= ?", *params.StartTime)
	}
	if params.EndTime != nil {
		query = query.Where("created_at <= ?", *params.EndTime)
	}
	
	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// 排序和分页
	// 验证分页参数
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100 // 限制最大页面大小
	}
	
	// 安全的排序字段验证（防止SQL注入）
	orderBy := "created_at DESC"
	if params.OrderBy != "" {
		// 只允许特定的排序字段
		allowedOrderFields := map[string]bool{
			"created_at DESC": true,
			"created_at ASC":  true,
			"updated_at DESC": true,
			"updated_at ASC":  true,
			"status DESC":     true,
			"status ASC":      true,
		}
		if allowedOrderFields[params.OrderBy] {
			orderBy = params.OrderBy
		}
	}
	
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order(orderBy).Limit(params.PageSize).Offset(offset).Find(&deployments).Error; err != nil {
		return nil, 0, err
	}
	
	return deployments, total, nil
}

// DeploymentListParams 部署记录查询参数
type DeploymentListParams struct {
	ProjectID   string
	ProjectName string
	EnvID       string
	ClusterID   string
	DeployType  string
	Status      string
	CreatedBy   string
	StartTime   *time.Time
	EndTime     *time.Time
	Page        int
	PageSize    int
	OrderBy     string
}

// Delete 删除部署记录
func (r *DeploymentRepository) Delete(id string) error {
	return r.db.Delete(&model.Deployment{}, "id = ?", id).Error
}

// SaveBuildLog 保存Jenkins构建日志
func (r *DeploymentRepository) SaveBuildLog(deploymentID string, buildLog string) error {
	return r.db.Model(&model.Deployment{}).Where("id = ?", deploymentID).Update("build_log", buildLog).Error
}

// GetBuildLog 获取Jenkins构建日志
func (r *DeploymentRepository) GetBuildLog(deploymentID string) (string, error) {
	var deployment model.Deployment
	err := r.db.Select("build_log").Where("id = ?", deploymentID).First(&deployment).Error
	if err != nil {
		return "", err
	}
	return deployment.BuildLog, nil
}

// FindByJenkinsBuild 根据Jenkins Job和Build Number查找部署记录
func (r *DeploymentRepository) FindByJenkinsBuild(jobName string, buildNumber int) (*model.Deployment, error) {
	var deployment model.Deployment
	err := r.db.Where("jenkins_job = ? AND jenkins_build_number = ?", jobName, buildNumber).First(&deployment).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

