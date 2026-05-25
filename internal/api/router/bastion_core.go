package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

func registerBastionCore(
	authenticated *gin.RouterGroup,
	hostHandler *handler.HostHandler,
	dashboardHandler *handler.DashboardHandler,
	sessionHandler *handler.SessionHandler,
	blacklistHandler *handler.BlacklistHandler,
	routingHandler *handler.RoutingHandler,
	hostGroupHandler *handler.HostGroupHandler,
	fileHandler *handler.FileHandler,
) {
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

		sessionsAdmin := sessions.Group("")
		sessionsAdmin.Use(middleware.AdminMiddleware())
		{
			sessionsAdmin.GET("/recordings", sessionHandler.GetSessionRecordings)
			sessionsAdmin.GET("/recordings/:sessionId", sessionHandler.GetSessionRecording)
			sessionsAdmin.GET("/recordings/:sessionId/file", func(c *gin.Context) {
				logger.Infof("========== ROUTE MATCHED: /recordings/:sessionId/file ==========")
				logger.Infof("SessionID param: %s", c.Param("sessionId"))
				logger.Infof("Full path: %s", c.Request.URL.Path)
				sessionHandler.GetSessionRecordingFile(c)
			})
			sessionsAdmin.POST("/recordings", sessionHandler.CreateSessionRecording)
			sessionsAdmin.DELETE("/:sessionId/terminate", sessionHandler.TerminateSession)
		}
	}

	files := authenticated.Group("/files")
	{
		files.GET("/list", fileHandler.ListFiles)
		files.POST("/upload", fileHandler.UploadFile)
		files.GET("/download", fileHandler.DownloadFile)
		files.GET("/transfers", fileHandler.GetFileTransfers)
	}

	commands := authenticated.Group("/commands")
	commands.Use(middleware.AdminMiddleware())
	{
		commands.GET("", sessionHandler.GetCommandRecords)
		commands.POST("", sessionHandler.CreateCommandRecord)
		commands.GET("/session/:sessionId", sessionHandler.GetCommandsBySession)
	}

	blacklist := authenticated.Group("/blacklist")
	blacklist.Use(middleware.AdminMiddleware())
	{
		blacklist.GET("/commands", blacklistHandler.GetCommands)
		blacklist.POST("/commands", blacklistHandler.CreateCommand)
		blacklist.PATCH("/commands/:id", blacklistHandler.UpdateCommand)
		blacklist.DELETE("/commands/:id", blacklistHandler.DeleteCommand)
	}

	routing := authenticated.Group("/routing")
	{
		routing.GET("/config", routingHandler.GetRoutingConfig)
		routing.PUT("/config", routingHandler.UpdateRoutingConfig)
	}
	authenticated.GET("/hosts/:id/route", routingHandler.GetRoutingDecision)
}
