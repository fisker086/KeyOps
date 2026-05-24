package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

func registerCore(
	authenticated *gin.RouterGroup,
	apiKeyHandler *handler.ApiKeyHandler,
	hostHandler *handler.HostHandler,
	dashboardHandler *handler.DashboardHandler,
	sessionHandler *handler.SessionHandler,
	proxyHandler *handler.ProxyHandler,
	authHandler *handler.AuthHandler,
	blacklistHandler *handler.BlacklistHandler,
	settingHandler *handler.SettingHandler,
	routingHandler *handler.RoutingHandler,
	hostGroupHandler *handler.HostGroupHandler,
	fileHandler *handler.FileHandler,
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

	dashboard := authenticated.Group("/dashboard")
	{
		dashboard.GET("/stats", dashboardHandler.GetStats)
		dashboard.GET("/recent-logins", dashboardHandler.GetRecentLogins)
		dashboard.GET("/frequent-hosts", dashboardHandler.GetFrequentHosts)
	}

	hosts := authenticated.Group("/hosts")
	{
		hosts.GET("", hostHandler.ListHosts)
		hosts.POST("", hostHandler.CreateHost)
		hosts.GET("/:id", hostHandler.GetHost)
		hosts.PUT("/:id", hostHandler.UpdateHost)
		hosts.DELETE("/:id", hostHandler.DeleteHost)
		hosts.POST("/:id/test", hostHandler.TestConnection)
		hosts.GET("/:id/groups", hostGroupHandler.GetHostGroups)
	}

	hostGroups := authenticated.Group("/host-groups")
	{
		hostGroups.GET("", hostGroupHandler.ListGroups)
		hostGroups.POST("", hostGroupHandler.CreateGroup)
		hostGroups.GET("/:id", hostGroupHandler.GetGroup)
		hostGroups.PUT("/:id", hostGroupHandler.UpdateGroup)
		hostGroups.DELETE("/:id", hostGroupHandler.DeleteGroup)
		hostGroups.GET("/:id/hosts", hostGroupHandler.GetGroupHosts)
		hostGroups.POST("/:id/hosts", hostGroupHandler.AddHostsToGroup)
		hostGroups.DELETE("/:id/hosts", hostGroupHandler.RemoveHostsFromGroup)
		hostGroups.GET("/:id/statistics", hostGroupHandler.GetGroupStatistics)
		hostGroups.GET("/:id/users", hostGroupHandler.GetGroupUsers)
	}

	sessions := authenticated.Group("/sessions")
	{
		sessions.POST("", sessionHandler.CreateSession)
		sessions.GET("/records", sessionHandler.GetLoginRecords)
	}

	files := authenticated.Group("/files")
	{
		files.GET("/list", fileHandler.ListFiles)
		files.POST("/upload", fileHandler.UploadFile)
		files.GET("/download", fileHandler.DownloadFile)
		files.GET("/transfers", fileHandler.GetFileTransfers)
	}

	sessionManage := authenticated.Group("/sessions")
	sessionManage.Use(middleware.AdminMiddleware())
	{
		sessionManage.GET("/recordings", sessionHandler.GetSessionRecordings)
		sessionManage.GET("/recordings/:sessionId", sessionHandler.GetSessionRecording)
		sessionManage.GET("/recordings/:sessionId/file", func(c *gin.Context) {
			logger.Infof("========== ROUTE MATCHED: /recordings/:sessionId/file ==========")
			logger.Infof("SessionID param: %s", c.Param("sessionId"))
			logger.Infof("Full path: %s", c.Request.URL.Path)
			sessionHandler.GetSessionRecordingFile(c)
		})
		sessionManage.POST("/recordings", sessionHandler.CreateSessionRecording)
		sessionManage.DELETE("/:sessionId/terminate", sessionHandler.TerminateSession)
	}

	commands := authenticated.Group("/commands")
	commands.Use(middleware.AdminMiddleware())
	{
		commands.GET("", sessionHandler.GetCommandRecords)
		commands.POST("", sessionHandler.CreateCommandRecord)
		commands.GET("/session/:sessionId", sessionHandler.GetCommandsBySession)
	}

	blacklist := authenticated.Group("/proxy/blacklist")
	blacklist.Use(middleware.AdminMiddleware())
	{
		blacklist.GET("/commands", blacklistHandler.GetCommands)
		blacklist.POST("/commands", blacklistHandler.CreateCommand)
		blacklist.PATCH("/commands/:id", blacklistHandler.UpdateCommand)
		blacklist.DELETE("/commands/:id", blacklistHandler.DeleteCommand)
	}

	proxyManage := authenticated.Group("/proxy")
	proxyManage.Use(middleware.AdminMiddleware())
	{
		proxyManage.GET("/list", proxyHandler.ListProxies)
		proxyManage.GET("/:proxy_id/stats", proxyHandler.GetProxyStats)
	}

	userSSHKey := authenticated.Group("/user-management")
	{
		userSSHKey.POST("/users/:id/ssh-key/generate", authHandler.GenerateSSHKey)
		userSSHKey.DELETE("/users/:id/ssh-key", authHandler.DeleteSSHKey)
		userSSHKey.GET("/users/:id/ssh-key/download", authHandler.DownloadSSHPrivateKey)
		userSSHKey.PUT("/users/:id/auth-method", authHandler.UpdateUserAuthMethod)
	}

	apiKeys := authenticated.Group("/api-keys")
	{
		apiKeys.GET("", apiKeyHandler.List)
		apiKeys.POST("", apiKeyHandler.Create)
		apiKeys.DELETE("/:id", apiKeyHandler.Revoke)
	}

	userManage := authenticated.Group("/user-management")
	userManage.Use(middleware.AdminMiddleware())
	{
		userManage.GET("/users", authHandler.GetUsersWithPagination)
		userManage.GET("/users-with-groups", authHandler.GetUsersWithGroups)
		userManage.GET("/users-with-roles", authHandler.GetUsersWithRoles)
		userManage.POST("/users", authHandler.CreateUserByAdmin)
		userManage.GET("/users/:id", authHandler.GetUserWithGroups)
		userManage.PUT("/users/:id", authHandler.UpdateUserByAdmin)
		userManage.PUT("/users/:id/role", authHandler.UpdateUserRole)
		userManage.PUT("/users/:id/status", authHandler.UpdateUserStatus)
		userManage.DELETE("/users/:id", authHandler.DeleteUser)
		userManage.POST("/users/:id/reset-password", authHandler.ResetUserPassword)
		userManage.GET("/users/:id/roles", authHandler.GetUserRoles)
		userManage.POST("/users/:id/roles", authHandler.AssignRolesToUser)
		userManage.GET("/users/:id/hosts", authHandler.GetUserHosts)
		userManage.POST("/users/:id/hosts", authHandler.AssignHostsToUser)
		userManage.GET("/users/:id/permissions", authHandler.GetUserWithGroupsAndHosts)
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

	routing := authenticated.Group("/routing")
	{
		routing.GET("/config", routingHandler.GetRoutingConfig)
		routing.PUT("/config", routingHandler.UpdateRoutingConfig)
		routing.GET("/proxies", routingHandler.GetAvailableProxies)
	}
	authenticated.GET("/hosts/:id/route", routingHandler.GetRoutingDecision)
}
