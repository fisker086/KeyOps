package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func registerAuthCore(
	authenticated *gin.RouterGroup,
	apiKeyHandler *handler.ApiKeyHandler,
	authHandler *handler.AuthHandler,
	settingHandler *handler.SettingHandler,
	twoFactorHandler *handler.TwoFactorHandler,
) {
	authenticated.POST("/auth/logout", authHandler.Logout)
	authenticated.POST("/auth/session/establish", authHandler.EstablishSession)
	authenticated.GET("/auth/me", authHandler.GetCurrentUser)
	authenticated.GET("/auth/me/permissions", authHandler.GetMyPermissions)
	authenticated.GET("/auth/login-records", authHandler.GetPlatformLoginRecords)
	authenticated.GET("/users", authHandler.GetUsers)

	twoFactor := authenticated.Group("/two-factor")
	{
		twoFactor.GET("/status", twoFactorHandler.GetUserStatus)
		twoFactor.GET("/global-status", twoFactorHandler.GetGlobalStatus)
		twoFactor.POST("/setup", twoFactorHandler.SetupTwoFactor)
		twoFactor.POST("/verify", twoFactorHandler.VerifyTwoFactor)
		twoFactor.POST("/disable", twoFactorHandler.DisableTwoFactor)
		twoFactor.POST("/verify-code", twoFactorHandler.VerifyCode)
		twoFactor.GET("/backup-codes", twoFactorHandler.GetBackupCodes)
		twoFactor.POST("/regenerate-backup-codes", twoFactorHandler.RegenerateBackupCodes)
	}

	apiKeys := authenticated.Group("/api-keys")
	{
		apiKeys.GET("", apiKeyHandler.List)
		apiKeys.POST("", apiKeyHandler.Create)
		apiKeys.DELETE("/:id", apiKeyHandler.Revoke)
	}

	userManagement := authenticated.Group("/user-management")
	{
		userManagement.POST("/users/:id/ssh-key/generate", authHandler.GenerateSSHKey)
		userManagement.DELETE("/users/:id/ssh-key", authHandler.DeleteSSHKey)
		userManagement.GET("/users/:id/ssh-key/download", authHandler.DownloadSSHPrivateKey)
		userManagement.PUT("/users/:id/auth-method", authHandler.UpdateUserAuthMethod)

		userAdmin := userManagement.Group("")
		userAdmin.Use(middleware.AdminMiddleware())
		{
			userAdmin.GET("/users", authHandler.GetUsersWithPagination)
			userAdmin.GET("/users-with-groups", authHandler.GetUsersWithGroups)
			userAdmin.GET("/users-with-roles", authHandler.GetUsersWithRoles)
			userAdmin.POST("/users", authHandler.CreateUserByAdmin)
			userAdmin.GET("/users/:id", authHandler.GetUserWithGroups)
			userAdmin.PUT("/users/:id", authHandler.UpdateUserByAdmin)
			userAdmin.PUT("/users/:id/role", authHandler.UpdateUserRole)
			userAdmin.PUT("/users/:id/status", authHandler.UpdateUserStatus)
			userAdmin.DELETE("/users/:id", authHandler.DeleteUser)
			userAdmin.POST("/users/:id/reset-password", authHandler.ResetUserPassword)
			userAdmin.GET("/users/:id/roles", authHandler.GetUserRoles)
			userAdmin.POST("/users/:id/roles", authHandler.AssignRolesToUser)
			userAdmin.GET("/users/:id/hosts", authHandler.GetUserHosts)
			userAdmin.POST("/users/:id/hosts", authHandler.AssignHostsToUser)
			userAdmin.GET("/users/:id/permissions", authHandler.GetUserWithGroupsAndHosts)
		}
	}

	settings := authenticated.Group("/settings")
	settings.Use(middleware.AdminMiddleware())
	{
		settings.GET("", settingHandler.GetAllSettings)
		settings.GET("/:category", settingHandler.GetSettingsByCategory)
		settings.PUT("", settingHandler.UpdateSettings)
		settings.PUT("/item", settingHandler.UpdateSetting)
		settings.DELETE("/:key", settingHandler.DeleteSetting)
		settings.POST("/test-ldap", settingHandler.TestLDAPConnection)
		settings.POST("/test-sso", settingHandler.TestSSOConnection)
		settings.POST("/test-feishu", settingHandler.TestFeishuNotification)
		settings.POST("/test-dingtalk", settingHandler.TestDingtalkNotification)
		settings.POST("/test-wechat", settingHandler.TestWechatNotification)
		settings.POST("/test-wechat-webhook", settingHandler.TestWechatWebhook)
		settings.POST("/test-deploy-git", settingHandler.TestDeployGit)
	}

	adminTwoFactor := authenticated.Group("/admin/two-factor")
	adminTwoFactor.Use(middleware.AdminMiddleware())
	{
		adminTwoFactor.GET("/config", twoFactorHandler.GetGlobalConfig)
		adminTwoFactor.PUT("/config", twoFactorHandler.UpdateGlobalConfig)
		adminTwoFactor.POST("/reset/:userId", twoFactorHandler.ResetUserTwoFactor)
	}
}
