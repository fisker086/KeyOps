package middleware

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service"
	apiKeyService "github.com/fisker086/keyops/internal/service/api_key"
	"github.com/gin-gonic/gin"
)

// McpAuthMiddleware 同时支持 X-API-Key 和 Authorization: Bearer 两种认证方式
func McpAuthMiddleware(authService *service.AuthService, apiKeySvc *apiKeyService.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryApiKeyAuth(c, apiKeySvc) {
			c.Next()
			return
		}

		if tryBearerAuth(c, authService) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, model.Error(401, "missing or invalid credentials"))
	}
}

func tryApiKeyAuth(c *gin.Context, svc *apiKeyService.ApiKeyService) bool {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		return false
	}

	ak, err := svc.ValidateKey(apiKey)
	if err != nil {
		return false
	}

	setUser(c, ak.User.ID, ak.User.Username, ak.User.Role)
	if len(ak.Permissions) > 0 {
		c.Set("mcp_permissions", ak.Permissions)
	}
	return true
}

func tryBearerAuth(c *gin.Context, authService *service.AuthService) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return false
	}

	claims, err := authService.ValidateToken(tokenString)
	if err != nil {
		return false
	}

	setUser(c, claims.UserID, claims.Username, claims.Role)
	return true
}
