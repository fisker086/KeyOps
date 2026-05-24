package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func registerPublic(
	api *gin.RouterGroup,
	authHandler *handler.AuthHandler,
	settingHandler *handler.SettingHandler,
	releaseHandler *handler.ReleaseHandler,
	proxyHandler *handler.ProxyHandler,
	blacklistHandler *handler.BlacklistHandler,
	sessionHandler *handler.SessionHandler,
) {
	auth := api.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
		auth.GET("/method", authHandler.GetAuthMethod)
		auth.GET("/sso/config", authHandler.GetSSOConfig)
		auth.GET("/sso/initiate", authHandler.InitiateSSO)
		auth.GET("/sso/callback", authHandler.SSOCallback)
	}

	api.GET("/settings/public", settingHandler.GetPublicSettings)
	api.GET("/auth/methods", settingHandler.GetAuthMethods)

	api.POST("/release/webhook", releaseHandler.HandleWebhook)
	api.POST("/release/webhook/push", releaseHandler.HandleWebhookPush)

	proxy := api.Group("/proxy")
	{
		proxy.POST("/register", proxyHandler.RegisterProxy)
		proxy.POST("/unregister", proxyHandler.Unregister)
		proxy.POST("/heartbeat", proxyHandler.Heartbeat)
		proxy.POST("/sessions", proxyHandler.ReportSession)
		proxy.POST("/sessions/:session_id/close", proxyHandler.CloseSession)
		proxy.POST("/commands", proxyHandler.ReportCommand)
		proxy.POST("/sessions/batch", proxyHandler.SyncSessions)
		proxy.POST("/commands/batch", proxyHandler.SyncCommands)
		proxy.GET("/blacklist", blacklistHandler.GetActiveCommands)
		proxy.GET("/validate-token", sessionHandler.ValidateToken)
	}
}
