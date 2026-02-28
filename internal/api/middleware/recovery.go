package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware 自定义错误恢复中间件，打印详细的错误信息
func RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// 获取错误信息
		err, ok := recovered.(error)
		if !ok {
			err = fmt.Errorf("%v", recovered)
		}

		// 获取请求信息
		requestMethod := c.Request.Method
		requestPath := c.Request.URL.Path
		requestQuery := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// 获取用户信息（如果有）
		userID := ""
		username := ""
		if uid, exists := c.Get("user_id"); exists {
			userID = fmt.Sprintf("%v", uid)
		}
		if uname, exists := c.Get("username"); exists {
			username = fmt.Sprintf("%v", uname)
		}

		// 构建完整的请求URL
		fullURL := requestPath
		if requestQuery != "" {
			fullURL = fmt.Sprintf("%s?%s", requestPath, requestQuery)
		}

		// 获取堆栈跟踪
		stack := string(debug.Stack())

		// 打印详细的错误日志
		logger.Errorf(
			"Panic recovered: %v\n"+
				"  Request: %s %s\n"+
				"  Client IP: %s\n"+
				"  User-Agent: %s\n"+
				"  User ID: %s\n"+
				"  Username: %s\n"+
				"  Stack Trace:\n%s",
			err,
			requestMethod,
			fullURL,
			clientIP,
			userAgent,
			userID,
			username,
			stack,
		)

		// 返回500错误响应
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		c.Abort()
	})
}
