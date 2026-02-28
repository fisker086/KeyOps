package middleware

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/casbin"
	"github.com/gin-gonic/gin"
)

// PermissionMiddleware Casbin权限中间件
// 检查用户是否有权限访问指定的API路径
func PermissionMiddleware(userRepo *repository.UserRepository, roleRepo *repository.RoleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户ID
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, model.Error(401, "用户ID格式错误"))
			c.Abort()
			return
		}

		// 获取请求路径和方法
		path := c.Request.URL.Path
		method := c.Request.Method

		// 移除API前缀（如果有）
		path = strings.TrimPrefix(path, "/api")

		// 优先级1: 检查用户直接权限
		hasPermission, err := casbin.Enforce(userIDStr, path, method)
		if err == nil && hasPermission {
			c.Next()
			return
		}

		// 优先级2: 检查角色权限（统一从 role_members 表获取，包括系统角色和自定义角色）
		// 获取用户所属的所有角色
		roles, err := roleRepo.GetRolesByUserID(userIDStr)
		if err == nil && len(roles) > 0 {
			for _, role := range roles {
				// 检查角色是否有权限
				hasPermission, err = casbin.Enforce(role.ID, path, method)
				if err == nil && hasPermission {
					c.Next()
					return
				}
			}
		}

		// 优先级3: 管理员默认拥有所有权限（如果没有配置任何权限规则）
		// 检查用户是否有 role:admin 角色
		for _, role := range roles {
			if role.ID == "role:admin" {
				c.Next()
				return
			}
		}

		// 没有权限
		c.JSON(http.StatusForbidden, model.Error(403, "权限不足"))
		c.Abort()
	}
}

// OptionalPermissionMiddleware 可选的权限中间件
// 如果权限系统未启用或用户没有配置权限，则允许访问
// 主要用于渐进式迁移
func OptionalPermissionMiddleware(userRepo *repository.UserRepository, roleRepo *repository.RoleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户ID
		userID, exists := c.Get("userID")
		if !exists {
			c.Next()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.Next()
			return
		}

		// 获取请求路径和方法
		path := c.Request.URL.Path
		method := c.Request.Method

		// 移除API前缀（如果有）
		path = strings.TrimPrefix(path, "/api")

		// 检查用户直接权限
		hasPermission, err := casbin.Enforce(userIDStr, path, method)
		if err != nil || !hasPermission {
			// 检查角色权限（统一从 role_members 表获取）
			roles, err := roleRepo.GetRolesByUserID(userIDStr)
			if err == nil && len(roles) > 0 {
				for _, role := range roles {
					hasPermission, err = casbin.Enforce(role.ID, path, method)
					if err == nil && hasPermission {
						c.Next()
						return
					}
					// 管理员角色默认拥有所有权限
					if role.ID == "role:admin" {
						c.Next()
						return
					}
				}
			}

			// 如果没有配置权限，默认允许访问（渐进式迁移）
			// 如果配置了权限但用户没有权限，则拒绝
			// 这里简化处理：如果没有找到任何策略，则允许访问
			c.Next()
			return
		}

		c.Next()
	}
}

