package middleware

import (
	"errors"
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
		if c.IsAborted() {
			return
		}

		if tryBearerAuth(c, authService) {
			c.Next()
			return
		}
		if c.IsAborted() {
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, model.Error(401, "missing or invalid credentials"))
	}
}

func tryApiKeyAuth(c *gin.Context, svc *apiKeyService.ApiKeyService) bool {
	apiKey := extractAPIKey(c)
	if apiKey == "" {
		return false
	}

	ak, err := svc.ValidateKey(apiKey)
	if err != nil {
		// 若 Bearer 里不是 API Key，允许继续走 JWT 鉴权分支
		if errors.Is(err, apiKeyService.ErrInvalidAPIKey) {
			return false
		}
		// 其他错误（禁用/过期）视为 API Key 认证失败并终止
		c.AbortWithStatusJSON(http.StatusUnauthorized, model.Error(401, err.Error()))
		return false
	}

	setUser(c, ak.User.ID, ak.User.Username, ak.User.Role)
	if len(ak.Permissions) > 0 {
		c.Set("mcp_permissions", ak.Permissions)
	}
	return true
}

func extractAPIKey(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-API-Key")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.Query("api_key")); v != "" {
		return v
	}
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		return ""
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
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
