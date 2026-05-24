package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type AssetSyncRepository interface {
	GetAll() ([]model.AssetSyncConfig, error)
	GetByID(id string) (*model.AssetSyncConfig, error)
	GetEnabledConfigs() ([]model.AssetSyncConfig, error)
	Create(config *model.AssetSyncConfig) error
	Update(config *model.AssetSyncConfig) error
	Delete(id string) error
	UpdateSyncStatus(id string, status string, syncedCount int, errorMsg string) error
	CreateLog(log *model.AssetSyncLog) error
	GetLogs(configID string, limit int) ([]model.AssetSyncLog, error)
}

type assetSyncRepository struct {
	db *gorm.DB
}

func NewAssetSyncRepository(db *gorm.DB) AssetSyncRepository {
	return &assetSyncRepository{db: db}
}

func (r *assetSyncRepository) GetAll() ([]model.AssetSyncConfig, error) {
	var configs []model.AssetSyncConfig
	err := r.db.Order("created_at DESC").Find(&configs).Error
	return configs, err
}

func (r *assetSyncRepository) GetByID(id string) (*model.AssetSyncConfig, error) {
	var config model.AssetSyncConfig
	err := r.db.Where("id = ?", id).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *assetSyncRepository) GetEnabledConfigs() ([]model.AssetSyncConfig, error) {
	var configs []model.AssetSyncConfig
	err := r.db.Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

func (r *assetSyncRepository) Create(config *model.AssetSyncConfig) error {
	return r.db.Create(config).Error
}

func (r *assetSyncRepository) Update(config *model.AssetSyncConfig) error {
	return r.db.Save(config).Error
}

func (r *assetSyncRepository) Delete(id string) error {
	return r.db.Delete(&model.AssetSyncConfig{}, "id = ?", id).Error
}

func (r *assetSyncRepository) UpdateSyncStatus(id string, status string, syncedCount int, errorMsg string) error {
	updates := map[string]interface{}{
		"last_sync_status": status,
		"synced_count":     syncedCount,
		"error_message":    errorMsg,
	}
	return r.db.Model(&model.AssetSyncConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *assetSyncRepository) CreateLog(log *model.AssetSyncLog) error {
	return r.db.Create(log).Error
}

func (r *assetSyncRepository) GetLogs(configID string, limit int) ([]model.AssetSyncLog, error) {
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
