package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type AssetSyncRepository struct {
	db *gorm.DB
}

func NewAssetSyncRepository(db *gorm.DB) *AssetSyncRepository {
	return &AssetSyncRepository{db: db}
}

// GetAll 获取所有同步配置
func (r *AssetSyncRepository) GetAll() ([]model.AssetSyncConfig, error) {
	var configs []model.AssetSyncConfig
	err := r.db.Order("created_at DESC").Find(&configs).Error
	return configs, err
}

// GetByID 根据ID获取配置
func (r *AssetSyncRepository) GetByID(id string) (*model.AssetSyncConfig, error) {
	var config model.AssetSyncConfig
	err := r.db.Where("id = ?", id).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetEnabledConfigs 获取所有启用的配置
func (r *AssetSyncRepository) GetEnabledConfigs() ([]model.AssetSyncConfig, error) {
	var configs []model.AssetSyncConfig
	err := r.db.Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

// Create 创建配置
func (r *AssetSyncRepository) Create(config *model.AssetSyncConfig) error {
	return r.db.Create(config).Error
}

// Update 更新配置
func (r *AssetSyncRepository) Update(config *model.AssetSyncConfig) error {
	return r.db.Save(config).Error
}

// Delete 删除配置
func (r *AssetSyncRepository) Delete(id string) error {
	return r.db.Delete(&model.AssetSyncConfig{}, "id = ?", id).Error
}

// UpdateSyncStatus 更新同步状态
func (r *AssetSyncRepository) UpdateSyncStatus(id string, status string, syncedCount int, errorMsg string) error {
	updates := map[string]interface{}{
		"last_sync_status": status,
		"synced_count":     syncedCount,
		"error_message":    errorMsg,
	}
	return r.db.Model(&model.AssetSyncConfig{}).Where("id = ?", id).Updates(updates).Error
}

// CreateLog 创建同步日志
func (r *AssetSyncRepository) CreateLog(log *model.AssetSyncLog) error {
	return r.db.Create(log).Error
}

// GetLogs 获取同步日志
func (r *AssetSyncRepository) GetLogs(configID string, limit int) ([]model.AssetSyncLog, error) {
	var logs []model.AssetSyncLog
	query := r.db.Order("created_at DESC")
	if configID != "" {
		query = query.Where("config_id = ?", configID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}
