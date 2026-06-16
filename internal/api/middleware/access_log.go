package middleware

import (
	"time"

	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

// AccessLogMiddleware 记录 API 请求（release 模式下 gin.Logger 默认关闭，账单等接口仍需可观测）。
func AccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) < 5 || path[:5] != "/api/" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		// 慢请求或账单相关路径用 info，其余成功快请求用 debug 减少噪音
		msg := "[HTTP] %s %s %d %s ip=%s"
		args := []interface{}{method, path, status, latency.Round(time.Millisecond), clientIP}
		if latency >= 500*time.Millisecond || status >= 400 || isBillPath(path) {
			logger.Infof(msg, args...)
		} else {
			logger.Debugf(msg, args...)
		}
	}
}

func isBillPath(path string) bool {
	return len(path) >= 9 && path[:9] == "/api/bill"
}
