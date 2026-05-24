package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func registerAlert(
	api *gin.RouterGroup,
	authenticated *gin.RouterGroup,
	alertHandler *handler.AlertHandler,
	onCallHandler *handler.OnCallHandler,
) {
	alerts := authenticated.Group("/alerts")
	{
		alerts.GET("/rule-groups", alertHandler.GetRuleGroups)
		alerts.GET("/rule-groups/:id", alertHandler.GetRuleGroup)
		alerts.POST("/rule-groups", alertHandler.CreateRuleGroup)
		alerts.PUT("/rule-groups/:id", alertHandler.UpdateRuleGroup)
		alerts.DELETE("/rule-groups/:id", alertHandler.DeleteRuleGroup)

		alerts.GET("/rule-sources", alertHandler.GetRuleSources)
		alerts.GET("/rule-sources/by-department", alertHandler.GetRuleSourcesByDepartment)
		alerts.GET("/rule-sources/by-group", alertHandler.GetRuleSourcesByGroup)
		alerts.GET("/rule-sources/:id", alertHandler.GetRuleSource)
		alerts.POST("/rule-sources", alertHandler.CreateRuleSource)
		alerts.PUT("/rule-sources/:id", alertHandler.UpdateRuleSource)
		alerts.DELETE("/rule-sources/:id", alertHandler.DeleteRuleSource)
		alerts.POST("/rule-sources/:id/sync", alertHandler.SyncRulesFromDatasource)

		alerts.GET("/groups", alertHandler.GetAlertGroups)
		alerts.GET("/groups/all", alertHandler.GetAllAlertGroups)
		alerts.GET("/groups/:id", alertHandler.GetAlertGroup)
		alerts.POST("/groups", alertHandler.CreateAlertGroup)
		alerts.PUT("/groups/:id", alertHandler.UpdateAlertGroup)
		alerts.DELETE("/groups/:id", alertHandler.DeleteAlertGroup)

		alerts.GET("/rules", alertHandler.GetRules)
		alerts.GET("/rules/:id", alertHandler.GetRule)
		alerts.POST("/rules", alertHandler.CreateRule)
		alerts.PUT("/rules/:id", alertHandler.UpdateRule)
		alerts.DELETE("/rules/:id", alertHandler.DeleteRule)
		alerts.POST("/datasources/:source_id/reload", alertHandler.ReloadDatasource)
		alerts.PATCH("/rules/:id/toggle", alertHandler.ToggleRule)
		alerts.POST("/rules/:id/move-to-default-group", alertHandler.MoveRuleToDefaultGroup)

		alerts.GET("/events", alertHandler.GetEvents)
		alerts.GET("/events/:id", alertHandler.GetEvent)
		alerts.POST("/events/:id/claim", alertHandler.ClaimEvent)
		alerts.POST("/events/:id/cancel-claim", alertHandler.CancelClaimEvent)
		alerts.POST("/events/:id/close", alertHandler.CloseEvent)
		alerts.POST("/events/:id/open", alertHandler.OpenEvent)

		alerts.GET("/strategies", alertHandler.GetStrategies)
		alerts.GET("/strategies/:id", alertHandler.GetStrategy)
		alerts.POST("/strategies", alertHandler.CreateStrategy)
		alerts.PUT("/strategies/:id", alertHandler.UpdateStrategy)
		alerts.DELETE("/strategies/:id", alertHandler.DeleteStrategy)
		alerts.PATCH("/strategies/:id/toggle", alertHandler.ToggleStrategy)

		alerts.GET("/silences", alertHandler.GetSilences)
		alerts.GET("/silences/:id", alertHandler.GetSilence)
		alerts.POST("/silences", alertHandler.CreateSilence)
		alerts.PUT("/silences/:id", alertHandler.UpdateSilence)
		alerts.DELETE("/silences/:id", alertHandler.DeleteSilence)

		alerts.GET("/aggregations", alertHandler.GetAggregations)
		alerts.GET("/aggregations/:id", alertHandler.GetAggregation)
		alerts.POST("/aggregations", alertHandler.CreateAggregation)
		alerts.PUT("/aggregations/:id", alertHandler.UpdateAggregation)
		alerts.DELETE("/aggregations/:id", alertHandler.DeleteAggregation)

		alerts.GET("/restrains", alertHandler.GetRestrains)
		alerts.GET("/restrains/:id", alertHandler.GetRestrain)
		alerts.POST("/restrains", alertHandler.CreateRestrain)
		alerts.PUT("/restrains/:id", alertHandler.UpdateRestrain)
		alerts.DELETE("/restrains/:id", alertHandler.DeleteRestrain)

		alerts.GET("/templates", alertHandler.GetTemplates)
		alerts.POST("/templates", alertHandler.CreateTemplate)
		alerts.GET("/templates/:id/channels", alertHandler.GetChannelTemplates)
		alerts.PUT("/templates/:id/channels/:channelId", alertHandler.UpdateChannelTemplate)
		alerts.DELETE("/templates/:id/channels/:channelId", alertHandler.DeleteChannelTemplate)
		alerts.GET("/templates/:id", alertHandler.GetTemplate)
		alerts.PUT("/templates/:id", alertHandler.UpdateTemplate)
		alerts.DELETE("/templates/:id", alertHandler.DeleteTemplate)

		alerts.GET("/channels", alertHandler.GetChannels)
		alerts.GET("/channels/:id", alertHandler.GetChannel)
		alerts.POST("/channels", alertHandler.CreateChannel)
		alerts.PUT("/channels/:id", alertHandler.UpdateChannel)
		alerts.DELETE("/channels/:id", alertHandler.DeleteChannel)

		alerts.GET("/strategy-logs", alertHandler.GetStrategyLogs)
		alerts.GET("/strategy-logs/:id", alertHandler.GetStrategyLog)

		alerts.GET("/levels", alertHandler.GetLevels)

		alerts.GET("/statistics", alertHandler.GetStatistics)
		alerts.GET("/statistics/trend", alertHandler.GetTrendStatistics)
		alerts.GET("/statistics/top", alertHandler.GetTopAlerts)

		alerts.GET("/certificates/domains", alertHandler.GetDomainCertificates)
		alerts.POST("/certificates/domains", alertHandler.CreateDomainCertificate)
		alerts.POST("/certificates/domains/check-alerts", alertHandler.CheckCertificateAlerts)
		alerts.POST("/certificates/domains/:id/refresh", alertHandler.RefreshDomainCertificate)
		alerts.GET("/certificates/domains/:id", alertHandler.GetDomainCertificate)
		alerts.PUT("/certificates/domains/:id", alertHandler.UpdateDomainCertificate)
		alerts.DELETE("/certificates/domains/:id", alertHandler.DeleteDomainCertificate)

		alerts.GET("/certificates/ssl", alertHandler.GetSslCertificates)
		alerts.GET("/certificates/ssl/:id", alertHandler.GetSslCertificate)
		alerts.POST("/certificates/ssl", alertHandler.CreateSslCertificate)
		alerts.PUT("/certificates/ssl/:id", alertHandler.UpdateSslCertificate)
		alerts.DELETE("/certificates/ssl/:id", alertHandler.DeleteSslCertificate)

		alerts.GET("/certificates/hosted", alertHandler.GetHostedCertificates)
		alerts.GET("/certificates/hosted/:id", alertHandler.GetHostedCertificate)
		alerts.POST("/certificates/hosted", alertHandler.CreateHostedCertificate)
		alerts.PUT("/certificates/hosted/:id", alertHandler.UpdateHostedCertificate)
		alerts.DELETE("/certificates/hosted/:id", alertHandler.DeleteHostedCertificate)
	}

	api.POST("/alerts/webhook/prometheus", alertHandler.WebhookPrometheus)

	oncall := authenticated.Group("/oncall")
	{
		oncall.GET("/schedules", onCallHandler.ListSchedules)
		oncall.POST("/schedules", onCallHandler.CreateSchedule)
		oncall.GET("/schedules/:id/shifts", onCallHandler.ListShiftsBySchedule)
		oncall.GET("/schedules/:id/current-user", onCallHandler.GetOnCallUserForSchedule)
		oncall.GET("/schedules/:id", onCallHandler.GetSchedule)
		oncall.PUT("/schedules/:id", onCallHandler.UpdateSchedule)
		oncall.DELETE("/schedules/:id", onCallHandler.DeleteSchedule)

		oncall.GET("/shifts/:id", onCallHandler.GetShift)
		oncall.POST("/shifts", onCallHandler.CreateShift)
		oncall.PUT("/shifts/:id", onCallHandler.UpdateShift)
		oncall.DELETE("/shifts/:id", onCallHandler.DeleteShift)

		oncall.GET("/current-users", onCallHandler.GetCurrentOnCallUsers)

		oncall.POST("/alerts/:alert_id/auto-assign", onCallHandler.AutoAssignAlert)
		oncall.POST("/alerts/:alert_id/manual-assign", onCallHandler.ManualAssignAlert)
		oncall.GET("/alerts/:alert_id/assignment", onCallHandler.GetAssignmentByAlert)
		oncall.GET("/users/:user_id/assignments", onCallHandler.ListAssignmentsByUser)
	}
}
