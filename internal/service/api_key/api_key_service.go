package api_key

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ApiKeyService struct {
	repo repository.ApiKeyRepository
}

func NewApiKeyService(repo repository.ApiKeyRepository) *ApiKeyService {
	return &ApiKeyService{repo: repo}
}

func (s *ApiKeyService) ListByUser(userID string) ([]model.ApiKey, error) {
	return s.repo.ListByUser(userID)
}

func (s *ApiKeyService) Create(userID string, name string, expiresAt *time.Time, permissions []string) (*model.ApiKey, string, error) {
	rawKey := generateRawKey()
	keyPrefix := rawKey[:12]

	apiKey := &model.ApiKey{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         rawKey,
		KeyPrefix:   keyPrefix,
		UserID:      userID,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := s.repo.Create(apiKey); err != nil {
		return nil, "", err
	}

	return apiKey, rawKey, nil
}

func (s *ApiKeyService) Revoke(userID string, id string) error {
	key, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if key.UserID != userID {
		return errors.New("cannot revoke another user's API key")
	}
	key.IsActive = false
	return s.repo.Update(key)
}

func (s *ApiKeyService) ValidateKey(rawKey string) (*model.ApiKey, error) {
	apiKey, err := s.repo.FindByKey(rawKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid API key")
		}
		return nil, err
	}

	if !apiKey.IsActive {
		return nil, errors.New("API key is disabled")
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("API key has expired")
	}

	_ = s.repo.UpdateLastUsed(apiKey.ID)

	return apiKey, nil
}

func generateRawKey() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return "k8s_mcp_" + base64.RawURLEncoding.EncodeToString(buf)
}
