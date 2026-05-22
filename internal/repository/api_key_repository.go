package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) *ApiKeyRepository {
	return &ApiKeyRepository{db: db}
}

func (r *ApiKeyRepository) Create(key *model.ApiKey) error {
	return r.db.Create(key).Error
}

func (r *ApiKeyRepository) FindByKey(key string) (*model.ApiKey, error) {
	var apiKey model.ApiKey
	err := r.db.Where("`key` = ?", key).Preload("User").First(&apiKey).Error
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *ApiKeyRepository) ListByUser(userID string) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *ApiKeyRepository) FindByID(id string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := r.db.Where("id = ?", id).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *ApiKeyRepository) Update(key *model.ApiKey) error {
	return r.db.Save(key).Error
}

func (r *ApiKeyRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.ApiKey{}).Error
}

func (r *ApiKeyRepository) UpdateLastUsed(id string) error {
	return r.db.Model(&model.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_at", gorm.Expr("NOW()")).Error
}
