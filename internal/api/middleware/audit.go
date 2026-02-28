package middleware

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/gin-gonic/gin"
)

// OperationLogMiddleware 操作日志中间件
func OperationLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 结束时间
		endTime := time.Now()
		timeCost := endTime.Sub(startTime).Milliseconds()

		// 只记录非 GET 请求的操作日志
		if c.Request.Method == "GET" {
			return
		}

		// 获取当前用户
		userID, exists := c.Get("user_id")
		if !exists {
			return
		}

		// 获取用户名
		username := ""
		var user model.User
		if err := database.DB.First(&user, "id = ?", userID).Error; err == nil {
			username = user.Username
		}

		// 获取 API 描述（从路径推断或从数据库查询）
		desc := getAPIDescription(c.Request.Method, c.FullPath())

		// 创建操作日志
		operationLog := model.OperationLog{
			Username:  username,
			IP:        c.ClientIP(),
			Method:    c.Request.Method,
			Path:      c.FullPath(),
			Desc:      desc,
			Status:    c.Writer.Status(),
			StartTime: startTime,
			TimeCost:  timeCost,
			UserAgent: c.Request.UserAgent(),
		}

		// 异步保存操作日志
		go func() {
			if err := database.DB.Create(&operationLog).Error; err != nil {
				// 记录错误但不影响请求处理
				// 可以使用日志库记录错误
			}
		}()
	}
}

// getAPIDescription 根据方法和路径获取 API 描述
func getAPIDescription(method, path string) string {
	// 从数据库查询 API 描述
	var api struct {
		Description string
	}
	
	// 清理路径，移除路径参数（如 :id）
	cleanPath := path
	// 简单的路径匹配：移除 /api 前缀后查询
	queryPath := cleanPath
	if len(queryPath) > 4 && queryPath[:4] == "/api" {
		queryPath = queryPath[4:]
	}
	
	// 尝试精确匹配
	err := database.DB.Table("apis").
		Where("method = ? AND path = ?", method, queryPath).
		Select("description").
		First(&api).Error
	
	if err == nil && api.Description != "" {
		return api.Description
	}
	
	// 如果精确匹配失败，尝试模式匹配（移除路径参数）
	// 例如：/api/v1/kube/pod/:id -> /api/v1/kube/pod
	patternPath := queryPath
	for {
		lastSlash := len(patternPath) - 1
		for lastSlash >= 0 && patternPath[lastSlash] != '/' {
			lastSlash--
		}
		if lastSlash < 0 {
			break
		}
		patternPath = patternPath[:lastSlash+1] + "%"
		
		err := database.DB.Table("apis").
			Where("method = ? AND path LIKE ?", method, patternPath).
			Select("description").
			First(&api).Error
		
		if err == nil && api.Description != "" {
			return api.Description
		}
		break
	}
	
	// 默认描述映射（K8s 相关）
	descriptions := map[string]string{
		"POST /v1/kube/scale":     "扩缩容副本",
		"PUT /v1/kube/scale":      "扩缩容副本",
		"DELETE /v1/kube/pod":     "重启Pod",
		"POST /v1/kube/pod/exec":  "执行容器命令",
		"PUT /v1/kube/deployment": "更新Deployment",
		"DELETE /v1/kube/service": "删除Service",
	}

	key := method + " " + queryPath
	if desc, ok := descriptions[key]; ok {
		return desc
	}

	// 默认描述
	return method + " " + queryPath
}
