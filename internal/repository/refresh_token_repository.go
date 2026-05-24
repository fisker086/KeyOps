package repository

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type RefreshTokenRepository interface {
	Create(token *model.RefreshToken) error
	FindByJTI(jti string) (*model.RefreshToken, error)
	RevokeByJTI(jti string) error
	RevokeAllByUser(userID string) error
	CountActiveByUser(userID string) (int64, error)
	FindOldestActiveByUser(userID string, limit int) ([]model.RefreshToken, error)
	CleanupExpired() error
}

type refreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(token *model.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *refreshTokenRepository) FindByJTI(jti string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.Where("jti = ?", jti).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *refreshTokenRepository) RevokeByJTI(jti string) error {
	return r.db.Model(&model.RefreshToken{}).
		Where("jti = ?", jti).
		Update("revoked", true).Error
}

func (r *refreshTokenRepository) RevokeAllByUser(userID string) error {
	return r.db.Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Update("revoked", true).Error
}

func (r *refreshTokenRepository) CountActiveByUser(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked = ? AND expires_at > ?", userID, false, time.Now()).
		Count(&count).Error
	return count, err
}

func (r *refreshTokenRepository) FindOldestActiveByUser(userID string, limit int) ([]model.RefreshToken, error) {
	var tokens []model.RefreshToken
	err := r.db.Where("user_id = ? AND revoked = ? AND expires_at > ?", userID, false, time.Now()).
		Order("created_at ASC").
		Limit(limit).
		Find(&tokens).Error
	return tokens, err
}

func (r *refreshTokenRepository) CleanupExpired() error {
	return r.db.Where("expires_at <= ?", time.Now()).Delete(&model.RefreshToken{}).Error
}
