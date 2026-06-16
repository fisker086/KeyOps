package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware JWT认证中间件
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果已经有用户信息（API key 认证已通过），跳过
		if _, exists := c.Get("user_id"); exists {
			c.Next()
			return
		}

		// WebSocket 升级请求特殊处理：允许通过 query 参数传递 token
		if strings.Contains(c.Request.URL.Path, "/ws/") {
			logger.Debugf("[AuthMiddleware] WebSocket request: %s %s", c.Request.Method, c.Request.URL.Path)
			tokenString := c.Query("token")
			if tokenString == "" {
				logger.Warnf("[Auth] 401 %s %s: missing token query", c.Request.Method, c.Request.URL.Path)
				c.JSON(http.StatusUnauthorized, model.Error(401, "WebSocket请求缺少token参数"))
				c.Abort()
				return
			}
			// 验证Token
			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				logAuthFailure(c, err)
				c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
				c.Abort()
				return
			}
			logger.Debugf("[AuthMiddleware] token validated, user_id=%s", claims.UserID)
			setUser(c, claims.UserID, claims.Username, claims.Role)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		tokenString := ""
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				logger.Warnf("[Auth] 401 %s %s: malformed Authorization header", c.Request.Method, c.Request.URL.Path)
				c.JSON(http.StatusUnauthorized, model.Error(401, "Token格式错误：Authorization header 必须以 'Bearer ' 开头"))
				c.Abort()
				return
			}
		} else {
			logger.Warnf("[Auth] 401 %s %s: missing Authorization header", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, model.Error(401, "缺少Authorization Header"))
			c.Abort()
			return
		}

		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			logAuthFailure(c, err)
			c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
			c.Abort()
			return
		}

		setUser(c, claims.UserID, claims.Username, claims.Role)
		c.Next()
	}
}

func logAuthFailure(c *gin.Context, err error) {
	method := c.Request.Method
	path := c.Request.URL.Path
	if errors.Is(err, jwt.ErrTokenExpired) {
		logger.Debugf("[Auth] 401 %s %s: token expired", method, path)
		return
	}
	logger.Warnf("[Auth] 401 %s %s: invalid token (%v)", method, path, err)
}

func setUser(c *gin.Context, userID, username, role string) {
	c.Set("user_id", userID)
	c.Set("userID", userID)
	c.Set("username", username)
	c.Set("role", role)
}

// AdminMiddleware 管理员权限中间件
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/api/sessions/recordings" ||
			strings.Contains(c.Request.URL.Path, "/api/sessions/recordings/") {
			c.Request.Header.Set("X-Debug-Path", c.Request.URL.Path)
		}

		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, model.Error(403, "需要管理员权限"))
			c.Abort()
			return
		}
		c.Next()
	}
}
