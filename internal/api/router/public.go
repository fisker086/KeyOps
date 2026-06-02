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
}
