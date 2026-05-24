package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func registerOps(
	authenticated *gin.RouterGroup,
	approvalHandler *handler.ApprovalHandler,
	assetSyncHandler *handler.AssetSyncHandler,
	systemUserHandler *handler.SystemUserHandler,
	organizationHandler *handler.OrganizationHandler,
	applicationHandler *handler.ApplicationHandler,
	appDeployBindingHandler *handler.AppDeployBindingHandler,
	roleHandler *handler.RoleHandler,
	permissionRuleHandler *handler.PermissionRuleHandler,
	permissionHandler *handler.PermissionHandler,
	formTemplateHandler *handler.FormTemplateHandler,
	formCategoryHandler *handler.FormCategoryHandler,
	ticketHandler *handler.TicketHandler,
	ticketDraftHandler *handler.TicketDraftHandler,
	workflowHandler *handler.WorkflowHandler,
) {
	approvals := authenticated.Group("/approvals")
	{
		approvals.GET("", approvalHandler.ListApprovals)
		approvals.POST("", approvalHandler.CreateApproval)
		approvals.GET("/stats", approvalHandler.GetApprovalStats)
		approvals.GET("/config", approvalHandler.GetApprovalConfig)
		approvals.GET("/search/users", approvalHandler.SearchUsers)
		approvals.GET("/search/hosts", approvalHandler.SearchHosts)
		approvals.POST("/third-party/create", approvalHandler.CreateThirdPartyApproval)
		approvals.POST("/third-party/form-detail", approvalHandler.GetApprovalFormDetail)
		approvals.GET("/:id", approvalHandler.GetApproval)
		approvals.POST("/:id/approve", approvalHandler.ApproveApproval)
		approvals.POST("/:id/reject", approvalHandler.RejectApproval)
		approvals.POST("/:id/cancel", approvalHandler.CancelApproval)
		approvals.POST("/:id/comments", approvalHandler.AddComment)
		approvals.PUT("/:id", approvalHandler.UpdateApproval)
	}

	approvalConfig := authenticated.Group("/approvals/config")
	approvalConfig.Use(middleware.AdminMiddleware())
	{
		approvalConfig.POST("", approvalHandler.UpdateApprovalConfig)
		approvalConfig.PUT("/:id", approvalHandler.UpdateApprovalConfig)
		approvalConfig.DELETE("/:id", approvalHandler.DeleteApprovalConfig)
	}

	assetSync := authenticated.Group("/asset-sync")
	assetSync.Use(middleware.AdminMiddleware())
	{
		assetSync.GET("/excel-template", assetSyncHandler.DownloadExcelTemplate)
		assetSync.GET("/configs", assetSyncHandler.ListConfigs)
		assetSync.POST("/configs", assetSyncHandler.CreateConfig)
		assetSync.PUT("/configs/:id", assetSyncHandler.UpdateConfig)
		assetSync.DELETE("/configs/:id", assetSyncHandler.DeleteConfig)
		assetSync.POST("/configs/:id/toggle", assetSyncHandler.ToggleConfig)
		assetSync.POST("/configs/:id/sync", assetSyncHandler.SyncNow)
		assetSync.GET("/logs", assetSyncHandler.GetLogs)
	}

	systemUsers := authenticated.Group("/system-users")
	{
		systemUsers.GET("", systemUserHandler.ListSystemUsers)
		systemUsers.GET("/available", systemUserHandler.GetAvailableSystemUsers)
		systemUsers.GET("/check-permission", systemUserHandler.CheckPermission)
		systemUsers.GET("/:id", systemUserHandler.GetSystemUser)
		systemUsers.POST("", middleware.AdminMiddleware(), systemUserHandler.CreateSystemUser)
		systemUsers.PUT("/:id", middleware.AdminMiddleware(), systemUserHandler.UpdateSystemUser)
		systemUsers.DELETE("/:id", middleware.AdminMiddleware(), systemUserHandler.DeleteSystemUser)
	}

	organizations := authenticated.Group("/organizations")
	{
		organizations.GET("", organizationHandler.ListOrganizations)
		organizations.GET("/tree", organizationHandler.GetOrganizationTree)
		organizations.GET("/:id", organizationHandler.GetOrganization)
		organizations.POST("", middleware.AdminMiddleware(), organizationHandler.CreateOrganization)
		organizations.PUT("/:id", middleware.AdminMiddleware(), organizationHandler.UpdateOrganization)
		organizations.DELETE("/:id", middleware.AdminMiddleware(), organizationHandler.DeleteOrganization)
	}

	applications := authenticated.Group("/applications")
	{
		applications.GET("", applicationHandler.ListApplications)
		applications.GET("/:id", applicationHandler.GetApplication)
		applications.POST("", middleware.AdminMiddleware(), applicationHandler.CreateApplication)
		applications.PUT("/:id", middleware.AdminMiddleware(), applicationHandler.UpdateApplication)
		applications.DELETE("/:id", middleware.AdminMiddleware(), applicationHandler.DeleteApplication)
	}

	appDeployBindings := authenticated.Group("/app-deploy-bindings")
	appDeployBindings.Use(middleware.AdminMiddleware())
	{
		appDeployBindings.GET("", appDeployBindingHandler.ListApplicationDeployBindings)
		appDeployBindings.POST("", appDeployBindingHandler.CreateApplicationDeployBinding)
		appDeployBindings.GET("/applications", appDeployBindingHandler.GetApplicationsForDeploy)
		appDeployBindings.PUT("/:id", appDeployBindingHandler.UpdateApplicationDeployBinding)
		appDeployBindings.DELETE("/:id", appDeployBindingHandler.DeleteApplicationDeployBinding)
	}

	roles := authenticated.Group("/roles")
	{
		roles.GET("", roleHandler.ListRoles)
		roles.GET("/by-user", roleHandler.GetRoles)
		roles.GET("/:id", roleHandler.GetRole)
		roles.GET("/:id/members", roleHandler.GetRoleMembers)
		roles.POST("", middleware.AdminMiddleware(), roleHandler.CreateRole)
		roles.PUT("/:id", middleware.AdminMiddleware(), roleHandler.UpdateRole)
		roles.DELETE("/:id", middleware.AdminMiddleware(), roleHandler.DeleteRole)
		roles.POST("/:id/members", middleware.AdminMiddleware(), roleHandler.AddRoleMember)
		roles.DELETE("/:id/members/:userId", middleware.AdminMiddleware(), roleHandler.RemoveRoleMember)
		roles.POST("/:id/members/batch", middleware.AdminMiddleware(), roleHandler.BatchAddMembers)
	}

	permissionRules := authenticated.Group("/permission-rules")
	permissionRules.Use(middleware.AdminMiddleware())
	{
		permissionRules.GET("", permissionRuleHandler.ListPermissionRules)
		permissionRules.GET("/:id", permissionRuleHandler.GetPermissionRule)
		permissionRules.POST("", permissionRuleHandler.CreatePermissionRule)
		permissionRules.PUT("/:id", permissionRuleHandler.UpdatePermissionRule)
		permissionRules.DELETE("/:id", permissionRuleHandler.DeletePermissionRule)
		permissionRules.GET("/by-role", permissionRuleHandler.GetPermissionRulesByRole)
		permissionRules.GET("/by-host-group", permissionRuleHandler.GetPermissionRulesByHostGroup)
	}

	permissions := authenticated.Group("/permissions")
	{
		permissions.GET("/user-menus", permissionHandler.GetUserMenus)
		menus := permissions.Group("/menus")
		menus.Use(middleware.AdminMiddleware())
		{
			menus.GET("", permissionHandler.ListMenus)
			menus.POST("", permissionHandler.CreateMenu)
			menus.PUT("/:id", permissionHandler.UpdateMenu)
			menus.DELETE("/:id", permissionHandler.DeleteMenu)
			menus.PUT("/sort/batch", permissionHandler.BatchUpdateMenuSortOrder)
			menus.GET("/role/:role", permissionHandler.GetMenuPermissionsByRole)
			menus.PUT("/role/:role", permissionHandler.UpdateMenuPermissionsByRole)
		}
		apis := permissions.Group("/apis")
		apis.Use(middleware.AdminMiddleware())
		{
			apis.GET("", permissionHandler.ListAPIs)
			apis.POST("", permissionHandler.CreateAPI)
			apis.PUT("/:id", permissionHandler.UpdateAPI)
			apis.DELETE("/:id", permissionHandler.DeleteAPI)
			apis.GET("/groups", permissionHandler.GetAPIGroups)
			apis.GET("/role/:role", permissionHandler.GetAPIPermissionsByRole)
			apis.PUT("/role/:role", permissionHandler.UpdateAPIPermissionsByRole)
		}
	}

	formTemplates := authenticated.Group("/form-templates")
	{
		formTemplates.GET("", formTemplateHandler.ListFormTemplates)
		formTemplates.POST("", formTemplateHandler.CreateFormTemplate)
		formTemplates.GET("/:id", formTemplateHandler.GetFormTemplate)
		formTemplates.PUT("/:id", formTemplateHandler.UpdateFormTemplate)
		formTemplates.DELETE("/:id", formTemplateHandler.DeleteFormTemplate)
		formTemplates.POST("/:id/preview", formTemplateHandler.PreviewFormTemplate)
	}

	formCategories := authenticated.Group("/form-categories")
	{
		formCategories.GET("", formCategoryHandler.ListCategories)
		formCategories.POST("", formCategoryHandler.CreateCategory)
		formCategories.GET("/:id", formCategoryHandler.GetCategory)
		formCategories.PUT("/:id", formCategoryHandler.UpdateCategory)
		formCategories.DELETE("/:id", formCategoryHandler.DeleteCategory)
	}

	tickets := authenticated.Group("/tickets")
	{
		tickets.GET("", ticketHandler.ListTickets)
		tickets.POST("", ticketHandler.CreateTicket)
		tickets.GET("/:id", ticketHandler.GetTicket)
		tickets.PUT("/:id", ticketHandler.UpdateTicket)
		tickets.POST("/:id/submit", ticketHandler.SubmitTicket)
		tickets.POST("/:id/cancel", ticketHandler.CancelTicket)
		tickets.GET("/:id/render", ticketHandler.GetRenderForm)
	}

	ticketDrafts := authenticated.Group("/ticket-drafts")
	{
		ticketDrafts.GET("", ticketDraftHandler.ListDrafts)
		ticketDrafts.POST("", ticketDraftHandler.SaveDraft)
		ticketDrafts.PUT("/:id", ticketDraftHandler.UpdateDraft)
		ticketDrafts.DELETE("/:id", ticketDraftHandler.DeleteDraft)
	}

	authenticated.GET("/workflow", workflowHandler.GetWorkflow)
	authenticated.POST("/workflow", workflowHandler.CreateWorkflow)
	authenticated.PUT("/workflow", workflowHandler.UpdateWorkflow)
	authenticated.GET("/workflow_draft", workflowHandler.ListDrafts)
	authenticated.POST("/workflow_draft", workflowHandler.SaveDraft)
	authenticated.PUT("/workflow_draft", workflowHandler.SaveDraft)
	authenticated.DELETE("/workflow_draft", workflowHandler.DeleteDraft)
	authenticated.GET("/workflow_step_notify", workflowHandler.ListStepNotify)
}
