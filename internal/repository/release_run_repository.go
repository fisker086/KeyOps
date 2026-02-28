package repository

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ReleaseRunRepository struct {
	db *gorm.DB
}

func NewReleaseRunRepository(db *gorm.DB) *ReleaseRunRepository {
	return &ReleaseRunRepository{db: db}
}

func (r *ReleaseRunRepository) Create(run *model.ReleaseRun) error {
	return r.db.Create(run).Error
}

func (r *ReleaseRunRepository) GetByID(id string) (*model.ReleaseRun, error) {
	var run model.ReleaseRun
	err := r.db.Where("id = ?", id).First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *ReleaseRunRepository) Update(run *model.ReleaseRun) error {
	return r.db.Save(run).Error
}

func (r *ReleaseRunRepository) UpdateStatus(id string, status string, startedAt, completedAt *time.Time) error {
	updates := map[string]interface{}{"status": status}
	if startedAt != nil {
		updates["started_at"] = startedAt
	}
	if completedAt != nil {
		updates["completed_at"] = completedAt
	}
	return r.db.Model(&model.ReleaseRun{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateStatusAndDeployedEnv 更新状态并记录部署环境（执行时调用）
func (r *ReleaseRunRepository) UpdateStatusAndDeployedEnv(id string, status string, deployedEnv string, startedAt, completedAt *time.Time) error {
	updates := map[string]interface{}{"status": status}
	if deployedEnv != "" {
		updates["deployed_environment"] = deployedEnv
	}
	if startedAt != nil {
		updates["started_at"] = startedAt
	}
	if completedAt != nil {
		updates["completed_at"] = completedAt
	}
	return r.db.Model(&model.ReleaseRun{}).Where("id = ?", id).Updates(updates).Error
}

// GetLastSuccessfulProdRun 查询某应用最近一次 prod 部署成功的 run（用于回滚源）
func (r *ReleaseRunRepository) GetLastSuccessfulProdRun(applicationID string) (*model.ReleaseRun, error) {
	var run model.ReleaseRun
	err := r.db.Where("application_id = ? AND status = ? AND deployed_environment = ?",
		applicationID, model.ReleaseRunStatusSuccess, "prod").
		Order("completed_at DESC").Limit(1).First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// List 分页列表，支持按 repo_url、branch、status 筛选
func (r *ReleaseRunRepository) List(repoURL, branch, status string, page, pageSize int) ([]model.ReleaseRun, int64, error) {
	var list []model.ReleaseRun
	q := r.db.Model(&model.ReleaseRun{})
	if repoURL != "" {
		q = q.Where("repo_url = ?", repoURL)
	}
	if branch != "" {
		q = q.Where("branch = ?", branch)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}
