package model

import (
	"fmt"

	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{
		Code:    0,
		Message: "success",
		Data:    data,
	}
}

func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

// HandleError 统一错误处理函数，记录详细日志并返回错误响应
func HandleError(c *gin.Context, code int, err error, context ...string) {
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

	// 构建错误消息
	errorMsg := err.Error()
	if len(context) > 0 {
		errorMsg = fmt.Sprintf("%s: %v", context[0], err)
	}

	// 打印详细的错误日志
	logger.Errorf(
		"Request error [%d]: %v\n"+
			"  Request: %s %s\n"+
			"  Client IP: %s\n"+
			"  User-Agent: %s\n"+
			"  User ID: %s\n"+
			"  Username: %s",
		code,
		errorMsg,
		requestMethod,
		fullURL,
		clientIP,
		userAgent,
		userID,
		username,
	)

	// 返回错误响应
	c.JSON(code, Error(code, errorMsg))
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type DashboardStats struct {
	TotalHosts   int `json:"totalHosts"`
	OnlineHosts  int `json:"onlineHosts"`
	OfflineHosts int `json:"offlineHosts"`
	RecentLogins int `json:"recentLogins"`
}

type HostsResponse struct {
	Hosts []Host `json:"hosts"`
	Total int64  `json:"total"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
	WSUrl     string `json:"wsUrl"`
	Token     string `json:"token"` // 临时令牌，用于 Proxy 验证
}

type TestConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
