package middleware

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	apiKeyService "github.com/fisker086/keyops/internal/service/api_key"
	"github.com/gin-gonic/gin"
)

// ApiKeyAuthMiddleware 验证 X-API-Key header
// 如果 X-API-Key 存在则校验，不存在则跳过（留给 JWT 中间件兜底）
func ApiKeyAuthMiddleware(service *apiKeyService.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.Next()
			return
		}

		ak, err := service.ValidateKey(apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.Error(401, err.Error()))
			return
		}

		setUser(c, ak.User.ID, ak.User.Username, ak.User.Role)
		c.Next()
	}
}
