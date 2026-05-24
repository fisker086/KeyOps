package api_key

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAPIKeyNotFound       = errors.New("api key not found")
	ErrAPIKeyForbidden      = errors.New("cannot revoke another user's API key")
	ErrInvalidAPIKey        = errors.New("invalid API key")
	ErrAPIKeyDisabled       = errors.New("API key is disabled")
	ErrAPIKeyExpired        = errors.New("API key has expired")
	ErrInvalidAPIKeyName    = errors.New("api key name cannot be empty")
	ErrAPIKeyNameTooLong    = errors.New("api key name is too long")
	ErrInvalidAPIKeyExpires = errors.New("expires_at must be in the future")
	ErrTooManyPermissions   = errors.New("too many permissions")
	ErrInvalidPermission    = errors.New("permission contains invalid value")
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
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", ErrInvalidAPIKeyName
	}
	if len(name) > 100 {
		return nil, "", ErrAPIKeyNameTooLong
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, "", ErrInvalidAPIKeyExpires
	}

	normalizedPermissions, err := normalizePermissions(permissions)
	if err != nil {
		return nil, "", err
	}

	rawKey := generateRawKey()
	keyPrefix := rawKey[:12]

	apiKey := &model.ApiKey{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         rawKey,
		KeyPrefix:   keyPrefix,
		UserID:      userID,
		Permissions: normalizedPermissions,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAPIKeyNotFound
		}
		return err
	}
	if key.UserID != userID {
		return ErrAPIKeyForbidden
	}
	key.IsActive = false
	return s.repo.Update(key)
}

func (s *ApiKeyService) ValidateKey(rawKey string) (*model.ApiKey, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return nil, ErrInvalidAPIKey
	}

	apiKey, err := s.repo.FindByKey(rawKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	if !apiKey.IsActive {
		return nil, ErrAPIKeyDisabled
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}

	_ = s.repo.UpdateLastUsed(apiKey.ID)

	return apiKey, nil
}

func generateRawKey() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return "k8s_mcp_" + base64.RawURLEncoding.EncodeToString(buf)
}

func normalizePermissions(permissions []string) ([]string, error) {
	if len(permissions) == 0 {
		return nil, nil
	}
	if len(permissions) > 256 {
		return nil, ErrTooManyPermissions
	}

	uniq := make(map[string]struct{}, len(permissions))
	result := make([]string, 0, len(permissions))
	for _, p := range permissions {
		pp := strings.TrimSpace(p)
		if pp == "" || len(pp) > 128 {
			return nil, ErrInvalidPermission
		}
		if _, ok := uniq[pp]; ok {
			continue
		}
		uniq[pp] = struct{}{}
		result = append(result, pp)
	}
	return result, nil
}
