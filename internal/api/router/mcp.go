package router

import (
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func registerMCP(r *gin.Engine, api, authenticated *gin.RouterGroup, d Deps) {
	h := d.Handlers
	s := d.Services

	authenticated.GET("/mcp/tools", h.Mcp.ListTools)

	mcpAuth := middleware.McpAuthMiddleware(s.Auth, s.ApiKey)
	api.GET("/mcp", mcpAuth, h.Mcp.HandleMCPGet)
	api.POST("/mcp", mcpAuth, h.Mcp.HandleMCP)
	r.GET("/api/v1/mcp", mcpAuth, h.Mcp.HandleMCPGet)
	r.POST("/api/v1/mcp", mcpAuth, h.Mcp.HandleMCP)
}
