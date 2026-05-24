package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApiKeyRepository interface {
	Create(key *model.ApiKey) error
	FindByKey(key string) (*model.ApiKey, error)
	ListByUser(userID string) ([]model.ApiKey, error)
	FindByID(id string) (*model.ApiKey, error)
	Update(key *model.ApiKey) error
	Delete(id string) error
	UpdateLastUsed(id string) error
}

type apiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) ApiKeyRepository {
	return &apiKeyRepository{db: db}
}

func (r *apiKeyRepository) Create(key *model.ApiKey) error {
	return r.db.Create(key).Error
}

func (r *apiKeyRepository) FindByKey(key string) (*model.ApiKey, error) {
	var apiKey model.ApiKey
	err := r.db.Where("`key` = ?", key).Preload("User").First(&apiKey).Error
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *apiKeyRepository) ListByUser(userID string) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *apiKeyRepository) FindByID(id string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := r.db.Where("id = ?", id).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepository) Update(key *model.ApiKey) error {
	return r.db.Save(key).Error
}

func (r *apiKeyRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.ApiKey{}).Error
}

func (r *apiKeyRepository) UpdateLastUsed(id string) error {
	return r.db.Model(&model.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_at", gorm.Expr("NOW()")).Error
}
