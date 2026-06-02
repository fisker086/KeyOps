package api_key

import (
	"errors"
	"net/http"
	"strings"
	"time"

	apiKeyService "github.com/fisker086/keyops/internal/service/api_key"
	"github.com/gin-gonic/gin"

	"github.com/fisker086/keyops/internal/model"
)

type ApiKeyHandler struct {
	service *apiKeyService.ApiKeyService
}

func NewApiKeyHandler(service *apiKeyService.ApiKeyService) *ApiKeyHandler {
	return &ApiKeyHandler{service: service}
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	keys, err := h.service.ListByUser(userID)
	if err != nil {
		model.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	safeKeys := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		safeKeys = append(safeKeys, gin.H{
			"id":           k.ID,
			"name":         k.Name,
			"key_prefix":   k.KeyPrefix,
			"permissions":  k.Permissions,
			"is_active":    k.IsActive,
			"expires_at":   k.ExpiresAt,
			"last_used_at": k.LastUsedAt,
			"created_at":   k.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, model.Success(safeKeys))
}

type createRequest struct {
	Name        string     `json:"name" binding:"required"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Permissions []string   `json:"permissions"`
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	key, rawKey, err := h.service.Create(userID, req.Name, req.ExpiresAt, req.Permissions)
	if err != nil {
		switch {
		case errors.Is(err, apiKeyService.ErrInvalidAPIKeyName),
			errors.Is(err, apiKeyService.ErrAPIKeyNameTooLong),
			errors.Is(err, apiKeyService.ErrInvalidAPIKeyExpires),
			errors.Is(err, apiKeyService.ErrInvalidPermission),
			errors.Is(err, apiKeyService.ErrTooManyPermissions):
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
			return
		}
		model.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"full_key":   rawKey,
		"key_prefix": key.KeyPrefix,
		"expires_at": key.ExpiresAt,
		"created_at": key.CreatedAt,
	}))
}

func (h *ApiKeyHandler) Revoke(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	if err := h.service.Revoke(userID, id); err != nil {
		switch {
		case errors.Is(err, apiKeyService.ErrAPIKeyNotFound):
			c.JSON(http.StatusNotFound, model.Error(404, err.Error()))
			return
		case errors.Is(err, apiKeyService.ErrAPIKeyForbidden):
			c.JSON(http.StatusForbidden, model.Error(403, err.Error()))
			return
		default:
			model.HandleError(c, http.StatusInternalServerError, err)
			return
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"success": true}))
}
