package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware JWT认证中间件
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket 升级请求特殊处理：允许通过 query 参数传递 token
		if strings.Contains(c.Request.URL.Path, "/ws/") {
			fmt.Printf("[AuthMiddleware] WebSocket 请求: %s %s\n", c.Request.Method, c.Request.URL.Path)
			log.Printf("[AuthMiddleware] WebSocket 请求: %s %s", c.Request.Method, c.Request.URL.Path)
			tokenString := c.Query("token")
			if tokenString == "" {
				fmt.Printf("[AuthMiddleware] WebSocket请求缺少token参数\n")
				log.Printf("[AuthMiddleware] WebSocket请求缺少token参数")
				c.JSON(http.StatusUnauthorized, model.Error(401, "WebSocket请求缺少token参数"))
				c.Abort()
				return
			}
			// 验证Token
			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				fmt.Printf("[AuthMiddleware] Token验证失败: %v\n", err)
				log.Printf("[AuthMiddleware] Token验证失败: %v", err)
				c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
				c.Abort()
				return
			}
			fmt.Printf("[AuthMiddleware] Token验证成功，用户ID: %s\n", claims.UserID)
			log.Printf("[AuthMiddleware] Token验证成功，用户ID: %s", claims.UserID)
			// 将用户信息保存到上下文（同时设置两个键名以保持兼容性）
			c.Set("user_id", claims.UserID)
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Next()
			return
		}

		// 支持从 Header 或 query 参数获取 token，便于静态文件/录制回放请求
		authHeader := c.GetHeader("Authorization")
		tokenString := ""
		if authHeader != "" {
			// 移除 "Bearer " 前缀
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				c.JSON(http.StatusUnauthorized, model.Error(401, "Token格式错误：Authorization header 必须以 'Bearer ' 开头"))
				c.Abort()
				return
			}
		} else {
			// 兼容静态资源访问：允许通过 query 传递 token
			tokenString = c.Query("token")
			if tokenString == "" {
				c.JSON(http.StatusUnauthorized, model.Error(401, "缺少Authorization Header或token参数"))
				c.Abort()
				return
			}
		}

		// 验证Token
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
			c.Abort()
			return
		}

		// 将用户信息保存到上下文（同时设置两个键名以保持兼容性）
		c.Set("user_id", claims.UserID)
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// AdminMiddleware 管理员权限中间件
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求路径用于调试
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
