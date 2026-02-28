package router

import (
	"net/http"

	_ "github.com/fisker086/keyops/docs"
	"github.com/fisker086/keyops/internal/aiassistant"
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/pkg/logger" // swagger docs
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Setup(
	hostHandler *handler.HostHandler,
	dashboardHandler *handler.DashboardHandler,
	sessionHandler *handler.SessionHandler,
	proxyHandler *handler.ProxyHandler,
	authHandler *handler.AuthHandler,
	blacklistHandler *handler.BlacklistHandler,
	settingHandler *handler.SettingHandler,
	routingHandler *handler.RoutingHandler,
	connectionHandler *handler.ConnectionHandler,
	hostGroupHandler *handler.HostGroupHandler,
	approvalHandler *handler.ApprovalHandler,
	approvalCallbackHandler *handler.ApprovalCallbackHandler,
	fileHandler *handler.FileHandler,
	assetSyncHandler *handler.AssetSyncHandler,
	authService *service.AuthService,
	hostMonitorHandler *handler.HostMonitorHandler,
	systemUserHandler *handler.SystemUserHandler,
	roleHandler *handler.RoleHandler,
	permissionRuleHandler *handler.PermissionRuleHandler,
	twoFactorHandler *handler.TwoFactorHandler,
	permissionHandler *handler.PermissionHandler,
	formTemplateHandler *handler.FormTemplateHandler,
	formCategoryHandler *handler.FormCategoryHandler,
	ticketHandler *handler.TicketHandler,
	ticketDraftHandler *handler.TicketDraftHandler,
	workflowHandler *handler.WorkflowHandler,
	k8sHandler *handler.K8sHandler,
	k8sClusterHandler *handler.K8sClusterHandler,
	k8sPermissionHandler *handler.K8sPermissionHandler,
	k8sSearchHandler *handler.K8sSearchHandler,
	deploymentHandler *handler.DeploymentHandler,
	billHandler *handler.BillHandler,
	monitorHandler *handler.MonitorHandler,
	organizationHandler *handler.OrganizationHandler,
	applicationHandler *handler.ApplicationHandler,
	appDeployBindingHandler *handler.AppDeployBindingHandler,
	registryHandler *handler.RegistryHandler,
	jenkinsHandler *handler.JenkinsHandler,
	auditHandler *handler.AuditHandler,
	alertHandler *handler.AlertHandler,
	onCallHandler *handler.OnCallHandler,
	dmsInstanceHandler *handler.DMSInstanceHandler,
	dmsQueryHandler *handler.DMSQueryHandler,
	dmsQueryLogHandler *handler.DMSQueryLogHandler,
	dmsPermissionHandler *handler.DMSPermissionHandler,
	releaseHandler *handler.ReleaseHandler,
	buildMasterHandler *handler.BuildMasterHandler,
	aiAssistantHandler *aiassistant.Handler,
	k8sPermissionService *service.K8sPermissionService,
	roleRepo *repository.RoleRepository,
	mode string,
) *gin.Engine {
	r := gin.New()

	// 设置文件上传大小限制为 1GB
	r.MaxMultipartMemory = 1 << 30 // 1GB = 1024 * 1024 * 1024 bytes

	// 使用自定义的 recovery 中间件（打印详细错误信息）
	r.Use(middleware.RecoveryMiddleware())
	// 使用 Gin 的 Logger 中间件（记录请求日志）
	r.Use(gin.Logger())

	// 中间件
	r.Use(middleware.CORS())

	// WebSocket 连接入口（统一入口，支持直连和代理）
	r.GET("/ws/connect", connectionHandler.HandleConnection)

	// 公开API（不需要认证）
	api := r.Group("/api")
	{
		// 认证相关（公开）
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/method", authHandler.GetAuthMethod)     // 获取当前认证方式
			auth.GET("/sso/config", authHandler.GetSSOConfig)  // 获取SSO配置状态
			auth.GET("/sso/initiate", authHandler.InitiateSSO) // 发起 SSO 登录
			auth.GET("/sso/callback", authHandler.SSOCallback) // SSO 回调
		}

		// 公开的系统设置（登录页也需要）
		api.GET("/settings/public", settingHandler.GetPublicSettings)
		api.GET("/auth/methods", settingHandler.GetAuthMethods) // 获取启用的认证方式

		// 发布代码 Webhook（GitHub/GitLab 推送时调用，不需要认证）
		api.POST("/release/webhook", releaseHandler.HandleWebhook)
		// 带 token 的动态推送 URL（应用发布绑定生成），根据 token 确定应用并创建发布记录
		api.POST("/release/webhook/push", releaseHandler.HandleWebhookPush)

		// Proxy 注册和同步（不需要认证，由 Proxy 调用）
		proxy := api.Group("/proxy")
		{
			proxy.POST("/register", proxyHandler.RegisterProxy)
			proxy.POST("/unregister", proxyHandler.Unregister)
			proxy.POST("/heartbeat", proxyHandler.Heartbeat)

			// 实时上报
			proxy.POST("/sessions", proxyHandler.ReportSession) // 实时会话上报
			proxy.POST("/sessions/:session_id/close", proxyHandler.CloseSession)
			proxy.POST("/commands", proxyHandler.ReportCommand) // 实时命令上报

			// 批量同步（定时任务，兼容旧方式）
			proxy.POST("/sessions/batch", proxyHandler.SyncSessions)
			proxy.POST("/commands/batch", proxyHandler.SyncCommands)

			// 黑名单（供 Proxy 获取，不需要认证）
			proxy.GET("/blacklist", blacklistHandler.GetActiveCommands)

			// 令牌验证（供 Proxy 调用）
			proxy.GET("/validate-token", sessionHandler.ValidateToken)
		}
	}

	// 需要认证的API
	authenticated := api.Group("")
	authenticated.Use(middleware.AuthMiddleware(authService))
	{
		// 用户相关
		authenticated.GET("/auth/me", authHandler.GetCurrentUser)
		authenticated.GET("/auth/login-records", authHandler.GetPlatformLoginRecords)

		// 用户列表（用于黑名单选择用户）
		authenticated.GET("/users", authHandler.GetUsers)

		// 2FA相关
		twoFactor := authenticated.Group("/two-factor")
		{
			twoFactor.GET("/status", twoFactorHandler.GetUserStatus)
			twoFactor.GET("/global-status", twoFactorHandler.GetGlobalStatus) // 普通用户可以查看全局MFA状态
			twoFactor.POST("/setup", twoFactorHandler.SetupTwoFactor)
			twoFactor.POST("/verify", twoFactorHandler.VerifyTwoFactor)
			twoFactor.POST("/disable", twoFactorHandler.DisableTwoFactor)
			twoFactor.POST("/verify-code", twoFactorHandler.VerifyCode)
			twoFactor.GET("/backup-codes", twoFactorHandler.GetBackupCodes)
			twoFactor.POST("/regenerate-backup-codes", twoFactorHandler.RegenerateBackupCodes)
		}

		// Dashboard
		dashboard := authenticated.Group("/dashboard")
		{
			dashboard.GET("/stats", dashboardHandler.GetStats)
			dashboard.GET("/recent-logins", dashboardHandler.GetRecentLogins)
			dashboard.GET("/frequent-hosts", dashboardHandler.GetFrequentHosts) // 获取用户常用主机
		}

		// Hosts
		hosts := authenticated.Group("/hosts")
		{
			hosts.GET("", hostHandler.ListHosts)
			hosts.POST("", hostHandler.CreateHost)
			hosts.GET("/:id", hostHandler.GetHost)
			hosts.PUT("/:id", hostHandler.UpdateHost)
			hosts.DELETE("/:id", hostHandler.DeleteHost)
			hosts.POST("/:id/test", hostHandler.TestConnection)
			hosts.POST("/:id/check-status", hostMonitorHandler.CheckHostStatus)     // 手动检查主机状态
			hosts.POST("/check-all-status", hostMonitorHandler.CheckAllHostsStatus) // 检查所有主机状态
			hosts.GET("/:id/groups", hostGroupHandler.GetHostGroups)                // 获取主机所属分组
		}

		// Host Groups (主机分组管理)
		hostGroups := authenticated.Group("/host-groups")
		{
			hostGroups.GET("", hostGroupHandler.ListGroups)                        // 获取所有分组
			hostGroups.POST("", hostGroupHandler.CreateGroup)                      // 创建分组
			hostGroups.GET("/:id", hostGroupHandler.GetGroup)                      // 获取分组详情
			hostGroups.PUT("/:id", hostGroupHandler.UpdateGroup)                   // 更新分组
			hostGroups.DELETE("/:id", hostGroupHandler.DeleteGroup)                // 删除分组
			hostGroups.GET("/:id/hosts", hostGroupHandler.GetGroupHosts)           // 获取分组中的主机
			hostGroups.POST("/:id/hosts", hostGroupHandler.AddHostsToGroup)        // 添加主机到分组
			hostGroups.DELETE("/:id/hosts", hostGroupHandler.RemoveHostsFromGroup) // 从分组移除主机
			hostGroups.GET("/:id/statistics", hostGroupHandler.GetGroupStatistics) // 获取分组统计
			hostGroups.GET("/:id/users", hostGroupHandler.GetGroupUsers)           // 获取分组的授权用户列表
		}

		// Sessions（普通接口）
		sessions := authenticated.Group("/sessions")
		{
			sessions.POST("", sessionHandler.CreateSession)
			sessions.GET("/records", sessionHandler.GetLoginRecords) // 历史记录（用户看自己的，管理员看全部）
		}

		// 文件传输
		files := authenticated.Group("/files")
		{
			files.GET("/list", fileHandler.ListFiles)             // 列出远程目录文件
			files.POST("/upload", fileHandler.UploadFile)         // 上传文件到目标服务器
			files.GET("/download", fileHandler.DownloadFile)      // 从目标服务器下载文件
			files.GET("/transfers", fileHandler.GetFileTransfers) // 获取文件传输记录
		}

		// 会话管理（仅管理员）
		sessionManage := authenticated.Group("/sessions")
		sessionManage.Use(middleware.AdminMiddleware())
		{
			// 会话录制
			sessionManage.GET("/recordings", sessionHandler.GetSessionRecordings)
			sessionManage.GET("/recordings/:sessionId", sessionHandler.GetSessionRecording)
			// 添加日志中间件用于调试
			sessionManage.GET("/recordings/:sessionId/file", func(c *gin.Context) {
				logger.Infof("========== ROUTE MATCHED: /recordings/:sessionId/file ==========")
				logger.Infof("SessionID param: %s", c.Param("sessionId"))
				logger.Infof("Full path: %s", c.Request.URL.Path)
				sessionHandler.GetSessionRecordingFile(c)
			})
			sessionManage.POST("/recordings", sessionHandler.CreateSessionRecording)

			// 终止会话
			sessionManage.DELETE("/:sessionId/terminate", sessionHandler.TerminateSession)
		}

		// 命令审计（仅管理员）
		commands := authenticated.Group("/commands")
		commands.Use(middleware.AdminMiddleware())
		{
			commands.GET("", sessionHandler.GetCommandRecords)
			commands.POST("", sessionHandler.CreateCommandRecord)
			commands.GET("/session/:sessionId", sessionHandler.GetCommandsBySession)
		}

		// 黑名单管理（仅管理员）
		blacklist := authenticated.Group("/proxy/blacklist")
		blacklist.Use(middleware.AdminMiddleware())
		{
			blacklist.GET("/commands", blacklistHandler.GetCommands)
			blacklist.POST("/commands", blacklistHandler.CreateCommand)
			blacklist.PATCH("/commands/:id", blacklistHandler.UpdateCommand)
			blacklist.DELETE("/commands/:id", blacklistHandler.DeleteCommand)
		}

		// Proxy 管理（需要管理员权限）
		proxyManage := authenticated.Group("/proxy")
		proxyManage.Use(middleware.AdminMiddleware())
		{
			proxyManage.GET("/list", proxyHandler.ListProxies)
			proxyManage.GET("/:proxy_id/stats", proxyHandler.GetProxyStats)
		}

		// SSH密钥管理（用户可以管理自己的密钥，handler内有权限检查）
		userSSHKey := authenticated.Group("/user-management")
		{
			userSSHKey.POST("/users/:id/ssh-key/generate", authHandler.GenerateSSHKey)       // 生成SSH密钥
			userSSHKey.DELETE("/users/:id/ssh-key", authHandler.DeleteSSHKey)                // 删除SSH密钥
			userSSHKey.GET("/users/:id/ssh-key/download", authHandler.DownloadSSHPrivateKey) // 下载私钥
			userSSHKey.PUT("/users/:id/auth-method", authHandler.UpdateUserAuthMethod)       // 更新认证方式
		}

		// 用户管理（需要管理员权限）
		userManage := authenticated.Group("/user-management")
		userManage.Use(middleware.AdminMiddleware())
		{
			userManage.GET("/users", authHandler.GetUsersWithPagination)                    // 分页获取用户列表
			userManage.GET("/users-with-groups", authHandler.GetUsersWithGroups)            // 获取用户及其分组信息
			userManage.GET("/users-with-roles", authHandler.GetUsersWithRoles)              // 获取用户及其所属角色
			userManage.POST("/users", authHandler.CreateUserByAdmin)                        // 创建用户
			userManage.GET("/users/:id", authHandler.GetUserWithGroups)                     // 获取用户详情
			userManage.PUT("/users/:id", authHandler.UpdateUserByAdmin)                     // 更新用户信息
			userManage.PUT("/users/:id/role", authHandler.UpdateUserRole)                   // 更新用户角色
			userManage.PUT("/users/:id/status", authHandler.UpdateUserStatus)               // 更新用户状态
			userManage.DELETE("/users/:id", authHandler.DeleteUser)                         // 删除用户
			userManage.POST("/users/:id/reset-password", authHandler.ResetUserPassword)     // 重置密码
			userManage.GET("/users/:id/roles", authHandler.GetUserRoles)                    // 获取用户角色
			userManage.POST("/users/:id/roles", authHandler.AssignRolesToUser)              // 分配角色
			userManage.GET("/users/:id/hosts", authHandler.GetUserHosts)                    // 获取用户主机权限
			userManage.POST("/users/:id/hosts", authHandler.AssignHostsToUser)              // 分配主机权限
			userManage.GET("/users/:id/permissions", authHandler.GetUserWithGroupsAndHosts) // 获取用户完整权限
		}

		// 系统设置（仅管理员）
		settings := authenticated.Group("/settings")
		settings.Use(middleware.AdminMiddleware())
		{
			settings.GET("", settingHandler.GetAllSettings)                          // 获取所有设置
			settings.GET("/:category", settingHandler.GetSettingsByCategory)         // 根据分类获取设置
			settings.PUT("", settingHandler.UpdateSettings)                          // 批量更新设置
			settings.PUT("/item", settingHandler.UpdateSetting)                      // 更新单个设置
			settings.DELETE("/:key", settingHandler.DeleteSetting)                   // 删除设置
			settings.POST("/test-ldap", settingHandler.TestLDAPConnection)           // 测试 LDAP 连接
			settings.POST("/test-sso", settingHandler.TestSSOConnection)             // 测试 SSO 配置
			settings.POST("/test-feishu", settingHandler.TestFeishuNotification)     // 测试飞书通知
			settings.POST("/test-dingtalk", settingHandler.TestDingtalkNotification) // 测试钉钉通知
			settings.POST("/test-wechat", settingHandler.TestWechatNotification)     // 测试企业微信通知（应用模式）
			settings.POST("/test-wechat-webhook", settingHandler.TestWechatWebhook)  // 测试企业微信群机器人 Webhook
			settings.POST("/test-deploy-git", settingHandler.TestDeployGit)          // 测试发布 Git 连接
		}

		// 2FA全局配置（仅管理员）
		adminTwoFactor := authenticated.Group("/admin/two-factor")
		adminTwoFactor.Use(middleware.AdminMiddleware())
		{
			adminTwoFactor.GET("/config", twoFactorHandler.GetGlobalConfig)
			adminTwoFactor.PUT("/config", twoFactorHandler.UpdateGlobalConfig)
			adminTwoFactor.POST("/reset/:userId", twoFactorHandler.ResetUserTwoFactor) // 重置用户2FA
		}

		// 路由决策（基于标签）
		routing := authenticated.Group("/routing")
		{
			// 路由配置管理（基于标签）
			routing.GET("/config", routingHandler.GetRoutingConfig)     // 获取路由配置
			routing.PUT("/config", routingHandler.UpdateRoutingConfig)  // 更新路由配置
			routing.GET("/proxies", routingHandler.GetAvailableProxies) // 获取可用代理列表

			// 旧的路由规则API（已废弃，仅为兼容性保留）
			routing.GET("/rules", routingHandler.ListRoutingRules)               // Deprecated
			routing.GET("/rules/:id", routingHandler.GetRoutingRule)             // Deprecated
			routing.POST("/rules", routingHandler.CreateRoutingRule)             // Deprecated
			routing.PUT("/rules/:id", routingHandler.UpdateRoutingRule)          // Deprecated
			routing.DELETE("/rules/:id", routingHandler.DeleteRoutingRule)       // Deprecated
			routing.PATCH("/rules/:id/toggle", routingHandler.ToggleRoutingRule) // Deprecated
		}

		// 主机路由决策（需要认证）
		authenticated.GET("/hosts/:id/route", routingHandler.GetRoutingDecision) // 获取主机的路由决策

		// 审批管理（工单系统）
		approvals := authenticated.Group("/approvals")
		{
			approvals.GET("", approvalHandler.ListApprovals)            // 获取审批列表（支持筛选：我的申请、待我审批、全部）
			approvals.POST("", approvalHandler.CreateApproval)          // 创建审批申请
			approvals.GET("/stats", approvalHandler.GetApprovalStats)   // 获取审批统计
			approvals.GET("/config", approvalHandler.GetApprovalConfig) // 获取审批配置（所有用户可读）
			// 注意：搜索路由和第三方审批路由必须在 /:id 之前，否则会被动态路由匹配
			approvals.GET("/search/users", approvalHandler.SearchUsers)                       // 搜索用户（审批人选择）
			approvals.GET("/search/hosts", approvalHandler.SearchHosts)                       // 搜索主机（资源选择）
			approvals.POST("/third-party/create", approvalHandler.CreateThirdPartyApproval)   // 创建第三方审批实例（从工单创建）
			approvals.POST("/third-party/form-detail", approvalHandler.GetApprovalFormDetail) // 获取审批表单详情
			// 动态路由放在后面
			approvals.GET("/:id", approvalHandler.GetApproval)              // 获取审批详情
			approvals.POST("/:id/approve", approvalHandler.ApproveApproval) // 批准审批
			approvals.POST("/:id/reject", approvalHandler.RejectApproval)   // 拒绝审批
			approvals.POST("/:id/cancel", approvalHandler.CancelApproval)   // 取消审批
			approvals.POST("/:id/comments", approvalHandler.AddComment)     // 添加评论
			approvals.PUT("/:id", approvalHandler.UpdateApproval)           // 更新审批（用于标记已发布等）
		}

		// 审批配置管理（仅管理员可修改）
		approvalConfig := authenticated.Group("/approvals/config")
		approvalConfig.Use(middleware.AdminMiddleware())
		{
			approvalConfig.POST("", approvalHandler.UpdateApprovalConfig)       // 创建审批配置
			approvalConfig.PUT("/:id", approvalHandler.UpdateApprovalConfig)    // 更新审批配置
			approvalConfig.DELETE("/:id", approvalHandler.DeleteApprovalConfig) // 删除审批配置
		}

		// 资产同步（仅管理员）
		assetSync := authenticated.Group("/asset-sync")
		assetSync.Use(middleware.AdminMiddleware())
		{
			assetSync.GET("/excel-template", assetSyncHandler.DownloadExcelTemplate) // 下载 Excel 模板
			assetSync.GET("/configs", assetSyncHandler.ListConfigs)                  // 获取所有同步配置
			assetSync.POST("/configs", assetSyncHandler.CreateConfig)                // 创建同步配置
			assetSync.PUT("/configs/:id", assetSyncHandler.UpdateConfig)             // 更新同步配置
			assetSync.DELETE("/configs/:id", assetSyncHandler.DeleteConfig)          // 删除同步配置
			assetSync.POST("/configs/:id/toggle", assetSyncHandler.ToggleConfig)     // 启用/禁用配置
			assetSync.POST("/configs/:id/sync", assetSyncHandler.SyncNow)            // 立即同步
			assetSync.GET("/logs", assetSyncHandler.GetLogs)                         // 获取同步日志
		}

		// 系统用户管理
		systemUsers := authenticated.Group("/system-users")
		{
			systemUsers.GET("", systemUserHandler.ListSystemUsers)                                       // 获取系统用户列表
			systemUsers.GET("/available", systemUserHandler.GetAvailableSystemUsers)                     // 获取用户可用的系统用户（用于登录前选择）
			systemUsers.GET("/check-permission", systemUserHandler.CheckPermission)                      // 检查权限
			systemUsers.GET("/:id", systemUserHandler.GetSystemUser)                                     // 获取单个系统用户
			systemUsers.POST("", middleware.AdminMiddleware(), systemUserHandler.CreateSystemUser)       // 创建系统用户（管理员）
			systemUsers.PUT("/:id", middleware.AdminMiddleware(), systemUserHandler.UpdateSystemUser)    // 更新系统用户（管理员）
			systemUsers.DELETE("/:id", middleware.AdminMiddleware(), systemUserHandler.DeleteSystemUser) // 删除系统用户（管理员）
		}

		// 组织管理（部门管理）
		organizations := authenticated.Group("/organizations")
		organizations.Use(middleware.AdminMiddleware())
		{
			organizations.GET("", organizationHandler.ListOrganizations)         // 获取组织列表
			organizations.GET("/tree", organizationHandler.GetOrganizationTree)  // 获取组织树
			organizations.GET("/:id", organizationHandler.GetOrganization)       // 获取单个组织
			organizations.POST("", organizationHandler.CreateOrganization)       // 创建组织
			organizations.PUT("/:id", organizationHandler.UpdateOrganization)    // 更新组织
			organizations.DELETE("/:id", organizationHandler.DeleteOrganization) // 删除组织
		}

		// 应用服务管理：GET 对已登录用户开放（列表/详情按运维/测试/研发负责人过滤），增删改仅管理员
		applications := authenticated.Group("/applications")
		{
			applications.GET("", applicationHandler.ListApplications)                                       // 获取应用列表（非管理员仅能看到自己负责的应用）
			applications.GET("/:id", applicationHandler.GetApplication)                                     // 获取单个应用（非管理员仅能看自己负责的）
			applications.POST("", middleware.AdminMiddleware(), applicationHandler.CreateApplication)       // 创建应用（仅管理员）
			applications.PUT("/:id", middleware.AdminMiddleware(), applicationHandler.UpdateApplication)    // 更新应用（仅管理员）
			applications.DELETE("/:id", middleware.AdminMiddleware(), applicationHandler.DeleteApplication) // 删除应用（仅管理员）
		}

		// 应用-发布绑定管理
		appDeployBindings := authenticated.Group("/app-deploy-bindings")
		appDeployBindings.Use(middleware.AdminMiddleware())
		{
			appDeployBindings.GET("", appDeployBindingHandler.ListApplicationDeployBindings)         // 获取绑定列表
			appDeployBindings.POST("", appDeployBindingHandler.CreateApplicationDeployBinding)       // 创建绑定
			appDeployBindings.GET("/applications", appDeployBindingHandler.GetApplicationsForDeploy) // 获取可用于发布的应用列表
			appDeployBindings.PUT("/:id", appDeployBindingHandler.UpdateApplicationDeployBinding)    // 更新绑定
			appDeployBindings.DELETE("/:id", appDeployBindingHandler.DeleteApplicationDeployBinding) // 删除绑定
		}

		// 用户组管理
		roles := authenticated.Group("/roles")
		{
			roles.GET("", roleHandler.ListRoles)                                                             // 获取角色列表
			roles.GET("/by-user", roleHandler.GetRoles)                                                      // 获取用户所在的角色
			roles.GET("/:id", roleHandler.GetRole)                                                           // 获取单个角色
			roles.GET("/:id/members", roleHandler.GetRoleMembers)                                            // 获取角色成员
			roles.POST("", middleware.AdminMiddleware(), roleHandler.CreateRole)                             // 创建角色（管理员）
			roles.PUT("/:id", middleware.AdminMiddleware(), roleHandler.UpdateRole)                          // 更新角色（管理员）
			roles.DELETE("/:id", middleware.AdminMiddleware(), roleHandler.DeleteRole)                       // 删除角色（管理员）
			roles.POST("/:id/members", middleware.AdminMiddleware(), roleHandler.AddRoleMember)              // 添加成员（管理员）
			roles.DELETE("/:id/members/:userId", middleware.AdminMiddleware(), roleHandler.RemoveRoleMember) // 移除成员（管理员）
			roles.POST("/:id/members/batch", middleware.AdminMiddleware(), roleHandler.BatchAddMembers)      // 批量添加成员（管理员）
		}

		// 授权规则管理（仅管理员）
		permissionRules := authenticated.Group("/permission-rules")
		permissionRules.Use(middleware.AdminMiddleware())
		{
			permissionRules.GET("", permissionRuleHandler.ListPermissionRules)                         // 获取授权规则列表
			permissionRules.GET("/:id", permissionRuleHandler.GetPermissionRule)                       // 获取单个授权规则
			permissionRules.POST("", permissionRuleHandler.CreatePermissionRule)                       // 创建授权规则
			permissionRules.PUT("/:id", permissionRuleHandler.UpdatePermissionRule)                    // 更新授权规则
			permissionRules.DELETE("/:id", permissionRuleHandler.DeletePermissionRule)                 // 删除授权规则
			permissionRules.GET("/by-role", permissionRuleHandler.GetPermissionRulesByRole)            // 根据角色查询
			permissionRules.GET("/by-host-group", permissionRuleHandler.GetPermissionRulesByHostGroup) // 根据主机组查询
		}

		// 权限管理（菜单和API权限）
		permissions := authenticated.Group("/permissions")
		{
			// 获取当前用户的菜单（根据权限过滤）- 所有认证用户都可以访问
			permissions.GET("/user-menus", permissionHandler.GetUserMenus)

			// 菜单管理（仅管理员）
			menus := permissions.Group("/menus")
			menus.Use(middleware.AdminMiddleware())
			{
				menus.GET("", permissionHandler.ListMenus)                              // 获取所有菜单
				menus.POST("", permissionHandler.CreateMenu)                            // 创建菜单
				menus.PUT("/:id", permissionHandler.UpdateMenu)                         // 更新菜单
				menus.DELETE("/:id", permissionHandler.DeleteMenu)                      // 删除菜单
				menus.PUT("/sort/batch", permissionHandler.BatchUpdateMenuSortOrder)    // 批量更新菜单排序
				menus.GET("/role/:role", permissionHandler.GetMenuPermissionsByRole)    // 获取角色的菜单权限
				menus.PUT("/role/:role", permissionHandler.UpdateMenuPermissionsByRole) // 更新角色的菜单权限
			}

			// API管理（仅管理员）
			apis := permissions.Group("/apis")
			apis.Use(middleware.AdminMiddleware())
			{
				apis.GET("", permissionHandler.ListAPIs)                              // 获取所有API
				apis.POST("", permissionHandler.CreateAPI)                            // 创建API
				apis.PUT("/:id", permissionHandler.UpdateAPI)                         // 更新API
				apis.DELETE("/:id", permissionHandler.DeleteAPI)                      // 删除API
				apis.GET("/groups", permissionHandler.GetAPIGroups)                   // 获取API分组
				apis.GET("/role/:role", permissionHandler.GetAPIPermissionsByRole)    // 获取角色的API权限
				apis.PUT("/role/:role", permissionHandler.UpdateAPIPermissionsByRole) // 更新角色的API权限
			}
		}

		// 表单模板管理
		formTemplates := authenticated.Group("/form-templates")
		{
			formTemplates.GET("", formTemplateHandler.ListFormTemplates)                // 获取模板列表
			formTemplates.POST("", formTemplateHandler.CreateFormTemplate)              // 创建模板
			formTemplates.GET("/:id", formTemplateHandler.GetFormTemplate)              // 获取模板详情
			formTemplates.PUT("/:id", formTemplateHandler.UpdateFormTemplate)           // 更新模板
			formTemplates.DELETE("/:id", formTemplateHandler.DeleteFormTemplate)        // 删除模板
			formTemplates.POST("/:id/preview", formTemplateHandler.PreviewFormTemplate) // 预览模板
		}

		// 表单模板分类管理
		formCategories := authenticated.Group("/form-categories")
		{
			formCategories.GET("", formCategoryHandler.ListCategories)     // 获取分类列表
			formCategories.POST("", formCategoryHandler.CreateCategory)    // 创建分类
			formCategories.GET("/:id", formCategoryHandler.GetCategory)    // 获取分类详情
			formCategories.PUT("/:id", formCategoryHandler.UpdateCategory) // 更新分类
			formCategories.DELETE("/:id", formCategoryHandler.DeleteCategory)
		}

		// 工单管理
		tickets := authenticated.Group("/tickets")
		{
			tickets.GET("", ticketHandler.ListTickets)              // 获取工单列表
			tickets.POST("", ticketHandler.CreateTicket)            // 创建工单
			tickets.GET("/:id", ticketHandler.GetTicket)            // 获取工单详情
			tickets.PUT("/:id", ticketHandler.UpdateTicket)         // 更新工单
			tickets.POST("/:id/submit", ticketHandler.SubmitTicket) // 提交工单
			tickets.POST("/:id/cancel", ticketHandler.CancelTicket) // 取消工单
			tickets.GET("/:id/render", ticketHandler.GetRenderForm) // 获取渲染表单
		}

		// 工单草稿管理
		ticketDrafts := authenticated.Group("/ticket-drafts")
		{
			ticketDrafts.GET("", ticketDraftHandler.ListDrafts)         // 获取草稿列表
			ticketDrafts.POST("", ticketDraftHandler.SaveDraft)         // 保存草稿
			ticketDrafts.PUT("/:id", ticketDraftHandler.UpdateDraft)    // 更新草稿
			ticketDrafts.DELETE("/:id", ticketDraftHandler.DeleteDraft) // 删除草稿
		}

		// 工作流（精简版，对齐 Venus）
		authenticated.GET("/workflow", workflowHandler.GetWorkflow)
		authenticated.POST("/workflow", workflowHandler.CreateWorkflow)
		authenticated.PUT("/workflow", workflowHandler.UpdateWorkflow)
		authenticated.GET("/workflow_draft", workflowHandler.ListDrafts)
		authenticated.POST("/workflow_draft", workflowHandler.SaveDraft)
		authenticated.PUT("/workflow_draft", workflowHandler.SaveDraft)
		authenticated.DELETE("/workflow_draft", workflowHandler.DeleteDraft)
		authenticated.GET("/workflow_step_notify", workflowHandler.ListStepNotify)

		// K8s 集群管理（需要操作审计）
		k8sClusters := authenticated.Group("/k8s/clusters")
		k8sClusters.Use(middleware.OperationLogMiddleware())
		{
			k8sClusters.GET("", k8sClusterHandler.ListClusters)                                       // 获取集群列表
			k8sClusters.GET("/summary", k8sClusterHandler.GetAllClustersSummary)                      // 获取所有集群摘要
			k8sClusters.GET("/dashboard/statistics", k8sClusterHandler.GetDashboardStatistics)        // 获取 K8s 大盘统计数据
			k8sClusters.POST("", k8sClusterHandler.CreateCluster)                                     // 创建集群
			k8sClusters.GET("/:id", k8sClusterHandler.GetCluster)                                     // 获取集群详情
			k8sClusters.GET("/:id/summary", k8sClusterHandler.GetClusterSummary)                      // 获取集群摘要
			k8sClusters.GET("/:id/permissions", k8sPermissionHandler.GetClusterPermissions)           // 获取集群权限
			k8sClusters.GET("/:id/permitted-namespaces", k8sPermissionHandler.GetPermittedNamespaces) // 获取当前用户可访问的命名空间
			k8sClusters.GET("/:id/effective-action", k8sPermissionHandler.GetEffectiveAction)         // 获取有效操作权限（用于前端控制按钮显示）
			k8sClusters.PUT("/:id", k8sClusterHandler.UpdateCluster)                                  // 更新集群
			k8sClusters.DELETE("/:id", k8sClusterHandler.DeleteCluster)                               // 删除集群
		}

		// K8s 权限管理
		k8sPermissions := authenticated.Group("/k8s/permissions")
		k8sPermissions.Use(middleware.OperationLogMiddleware())
		{
			k8sPermissions.GET("", k8sPermissionHandler.GetPermissions)         // 获取权限列表
			k8sPermissions.POST("", k8sPermissionHandler.AddPermission)         // 添加权限
			k8sPermissions.PUT("", k8sPermissionHandler.UpdatePermission)       // 更新权限（原子）
			k8sPermissions.DELETE("", k8sPermissionHandler.RemovePermission)    // 删除权限
			k8sPermissions.POST("/check", k8sPermissionHandler.CheckPermission) // 检查权限
		}

		// K8s 全局搜索
		k8sSearch := authenticated.Group("/k8s/search")
		{
			k8sSearch.GET("", k8sSearchHandler.GlobalSearch) // 全局搜索
		}

		// K8s 管理（需要操作审计和权限检查）
		// 使用 /v1/kube 路径以匹配前端
		// 权限检查使用统一的 Casbin 权限系统，权限存储在 casbin_rule 表中
		// 权限可以通过 /api/k8s/permissions API 或 k8S权限管理界面配置
		k8s := authenticated.Group("/v1/kube")
		k8s.Use(middleware.OperationLogMiddleware())
		k8s.Use(middleware.K8sPermissionMiddleware(k8sPermissionService, roleRepo))
		{
			k8s.GET("/base", k8sHandler.GetBaseInfo)                                                // 获取基础信息
			k8s.GET("/namespace", k8sHandler.GetNamespaceList)                                      // 获取命名空间列表
			k8s.GET("/pod", k8sHandler.GetPodList)                                                  // 获取Pod列表
			k8s.GET("/pod/detail", k8sHandler.GetPodDetail)                                         // 获取Pod详情
			k8s.GET("/service", k8sHandler.GetServiceList)                                          // 获取Service列表
			k8s.GET("/ingress", k8sHandler.GetIngressList)                                          // 获取Ingress列表
			k8s.GET("/hpa", k8sHandler.GetHPAList)                                                  // 获取HPA列表
			k8s.GET("/event", k8sHandler.GetEventList)                                              // 获取Event列表
			k8s.GET("/deployment", k8sHandler.GetDeploymentList)                                    // 获取Deployment列表
			k8s.GET("/deployment/:deployment_name", k8sHandler.GetDeploymentDetail)                 // 获取Deployment详情
			k8s.GET("/daemonset", k8sHandler.GetDaemonSetList)                                      // 获取DaemonSet列表
			k8s.GET("/daemonset/:daemonset_name", k8sHandler.GetDaemonSetDetail)                    // 获取DaemonSet详情
			k8s.GET("/statefulset", k8sHandler.GetStatefulSetList)                                  // 获取StatefulSet列表
			k8s.GET("/statefulset/:statefulset_name", k8sHandler.GetStatefulSetDetail)              // 获取StatefulSet详情
			k8s.GET("/cronjob", k8sHandler.GetCronJobList)                                          // 获取CronJob列表
			k8s.GET("/cronjob/:cronjob_name", k8sHandler.GetCronJobDetail)                          // 获取CronJob详情
			k8s.GET("/job", k8sHandler.GetJobList)                                                  // 获取Job列表
			k8s.GET("/job/:job_name", k8sHandler.GetJobDetail)                                      // 获取Job详情
			k8s.GET("/node", k8sHandler.GetNodeList)                                                // 获取Node列表
			k8s.GET("/pv", k8sHandler.GetPVList)                                                    // 获取PV列表
			k8s.GET("/pvc", k8sHandler.GetPVCList)                                                  // 获取PVC列表
			k8s.GET("/storageclass", k8sHandler.GetStorageClassList)                                // 获取StorageClass列表
			k8s.GET("/configmap", k8sHandler.GetConfigMapList)                                      // 获取ConfigMap列表
			k8s.GET("/secret", k8sHandler.GetSecretList)                                            // 获取Secret列表
			k8s.GET("/containers", k8sHandler.GetContainersList)                                    // 获取容器列表
			k8s.GET("/scale", k8sHandler.GetReplica)                                                // 获取副本数
			k8s.POST("/scale", k8sHandler.ScaleReplica)                                             // 扩缩容（使用POST匹配前端）
			k8s.DELETE("/pod", k8sHandler.RestartPod)                                               // 重启Pod
			k8s.GET("/pod/down_logs", k8sHandler.DownloadContainerLogs)                             // 下载容器日志
			k8s.GET("/pod/metrics", k8sHandler.GetPodMetrics)                                       // 获取Pod指标
			k8s.GET("/pod/ws/logs", k8sHandler.StreamPodLogs)                                       // 流式传输Pod日志（WebSocket）
			k8s.GET("/pod/ws/terminal", k8sHandler.ConnectPodTerminal)                              // 连接Pod终端（WebSocket）
			k8s.GET("/yaml", k8sHandler.GetResourceYaml)                                            // 获取资源YAML
			k8s.PUT("/yaml", k8sHandler.UpdateResourceYaml)                                         // 更新资源YAML
			k8s.DELETE("/yaml", k8sHandler.DeleteResource)                                          // 删除资源
			k8s.POST("/yaml/dry-run", k8sHandler.DryRunResourceYaml)                                // Dry-run 预览资源变更
			k8s.GET("/deployment/:deployment_name/revisions", k8sHandler.GetDeploymentRevisions)    // 获取Deployment历史版本
			k8s.POST("/deployment/:deployment_name/rollback", k8sHandler.RollbackDeployment)        // 回滚Deployment
			k8s.GET("/deployment/:deployment_name/metrics", k8sHandler.GetDeploymentMetrics)        // 获取Deployment监控数据
			k8s.GET("/daemonset/:daemonset_name/revisions", k8sHandler.GetDaemonSetRevisions)       // 获取DaemonSet历史版本
			k8s.POST("/daemonset/:daemonset_name/rollback", k8sHandler.RollbackDaemonSet)           // 回滚DaemonSet
			k8s.GET("/daemonset/:daemonset_name/metrics", k8sHandler.GetDaemonSetMetrics)           // 获取DaemonSet监控数据
			k8s.GET("/statefulset/:statefulset_name/revisions", k8sHandler.GetStatefulSetRevisions) // 获取StatefulSet历史版本
			k8s.POST("/statefulset/:statefulset_name/rollback", k8sHandler.RollbackStatefulSet)     // 回滚StatefulSet
			k8s.GET("/statefulset/:statefulset_name/metrics", k8sHandler.GetStatefulSetMetrics)     // 获取StatefulSet监控数据
		}

		// 发布管理
		deployments := authenticated.Group("/deployments")
		{
			deployments.GET("", deploymentHandler.ListDeployments)                   // 获取部署记录列表
			deployments.POST("", deploymentHandler.CreateDeployment)                 // 创建部署记录
			deployments.GET("/:id", deploymentHandler.GetDeployment)                 // 获取部署记录详情
			deployments.POST("/:id/execute", deploymentHandler.ExecuteK8sDeployment) // 执行 K8s 部署（包含 kubedog 监听）
			deployments.PUT("/:id/status", deploymentHandler.UpdateDeploymentStatus) // 更新部署状态
			deployments.DELETE("/:id", deploymentHandler.DeleteDeployment)           // 删除部署记录
		}

		// 发布代码（Git Webhook 触发的发布记录列表与手动触发）
		release := authenticated.Group("/release")
		{
			release.GET("/runs", releaseHandler.ListRuns)                                 // 发布记录列表
			release.GET("/runs/:id", releaseHandler.GetRun)                               // 发布记录详情
			release.POST("/runs", releaseHandler.CreateRun)                               // 手动创建发布记录
			release.POST("/runs/:id/execute", releaseHandler.ExecuteRun)                  // 执行发布（触发 Jenkins）
			release.POST("/runs/:id/status", releaseHandler.UpdateRunStatus)              // 更新 run 状态（success/failed，供回滚源）
			release.GET("/applications/:id/last-prod-run", releaseHandler.GetLastProdRun) // 最近一次 prod 成功 run（回滚用）
			release.POST("/rollback", releaseHandler.RollbackProd)                        // 提交生产回滚工单
			release.GET("/pipelines", releaseHandler.ListPipelines)                       // 流水线列表
			release.GET("/pipeline", releaseHandler.GetPipeline)                          // 获取单条流水线
			release.PUT("/pipeline", releaseHandler.SavePipeline)                         // 保存流水线（id 为空则新建）
			release.DELETE("/pipeline/:id", releaseHandler.DeletePipeline)                // 删除流水线
		}

		buildMaster := authenticated.Group("/build-master")
		{
			buildMaster.GET("/lists", buildMasterHandler.List)
			buildMaster.GET("/lists/:id", buildMasterHandler.Get)
			buildMaster.POST("/lists", buildMasterHandler.Create)
			buildMaster.PATCH("/lists/:id", buildMasterHandler.Update)
			buildMaster.GET("/records", buildMasterHandler.RecordsByQuery)
		}

		// 容器仓库：根据应用名从 Harbor/ECR/Nexus 获取制品版本号（Settings 中配置 registry_type）
		registry := authenticated.Group("/registry")
		{
			registry.GET("/applications/:appId/versions", registryHandler.GetApplicationVersions)
			registry.GET("/test", registryHandler.TestConnection) // 测试制品仓库连接（使用当前 Settings）
		}

		// 监控管理
		monitors := authenticated.Group("/monitors")
		{
			// Prometheus 监控查询语句管理
			monitors.GET("/prom", monitorHandler.ListMonitors)         // 获取监控列表
			monitors.GET("/prom/count", monitorHandler.CountMonitors)  // 统计监控数量
			monitors.POST("/prom", monitorHandler.CreateMonitor)       // 创建监控
			monitors.GET("/prom/:id", monitorHandler.GetMonitor)       // 获取监控详情
			monitors.PUT("/prom/:id", monitorHandler.UpdateMonitor)    // 更新监控
			monitors.DELETE("/prom/:id", monitorHandler.DeleteMonitor) // 删除监控

			// Probe 监控探针
			monitors.GET("/probe", monitorHandler.GetProbe) // 查询 Probe 监控数据
		}

		// 告警管理
		alerts := authenticated.Group("/alerts")
		{
			// 规则组管理
			alerts.GET("/rule-groups", alertHandler.GetRuleGroups)          // 获取规则组列表
			alerts.GET("/rule-groups/:id", alertHandler.GetRuleGroup)       // 获取规则组详情
			alerts.POST("/rule-groups", alertHandler.CreateRuleGroup)       // 创建规则组
			alerts.PUT("/rule-groups/:id", alertHandler.UpdateRuleGroup)    // 更新规则组
			alerts.DELETE("/rule-groups/:id", alertHandler.DeleteRuleGroup) // 删除规则组

			// 告警规则数据源
			alerts.GET("/rule-sources", alertHandler.GetRuleSources)                           // 获取数据源列表（分页）
			alerts.GET("/rule-sources/by-department", alertHandler.GetRuleSourcesByDepartment) // 根据部门获取数据源列表（不分页）
			alerts.GET("/rule-sources/by-group", alertHandler.GetRuleSourcesByGroup)           // 根据规则组获取数据源列表（不分页）
			alerts.GET("/rule-sources/:id", alertHandler.GetRuleSource)                        // 获取数据源详情
			alerts.POST("/rule-sources", alertHandler.CreateRuleSource)                        // 创建数据源
			alerts.PUT("/rule-sources/:id", alertHandler.UpdateRuleSource)                     // 更新数据源
			alerts.DELETE("/rule-sources/:id", alertHandler.DeleteRuleSource)                  // 删除数据源
			alerts.POST("/rule-sources/:id/sync", alertHandler.SyncRulesFromDatasource)        // 从数据源同步规则

			// 告警组
			alerts.GET("/groups", alertHandler.GetAlertGroups)          // 获取告警组列表（分页）
			alerts.GET("/groups/all", alertHandler.GetAllAlertGroups)   // 获取所有告警组（不分页，用于下拉选择）
			alerts.GET("/groups/:id", alertHandler.GetAlertGroup)       // 获取告警组详情
			alerts.POST("/groups", alertHandler.CreateAlertGroup)       // 创建告警组
			alerts.PUT("/groups/:id", alertHandler.UpdateAlertGroup)    // 更新告警组
			alerts.DELETE("/groups/:id", alertHandler.DeleteAlertGroup) // 删除告警组

			// 告警规则
			alerts.GET("/rules", alertHandler.GetRules)                                          // 获取告警规则列表
			alerts.GET("/rules/:id", alertHandler.GetRule)                                       // 获取告警规则详情
			alerts.POST("/rules", alertHandler.CreateRule)                                       // 创建告警规则
			alerts.PUT("/rules/:id", alertHandler.UpdateRule)                                    // 更新告警规则
			alerts.DELETE("/rules/:id", alertHandler.DeleteRule)                                 // 删除告警规则
			alerts.POST("/datasources/:source_id/reload", alertHandler.ReloadDatasource)         // 重新加载数据源配置
			alerts.PATCH("/rules/:id/toggle", alertHandler.ToggleRule)                           // 启用/禁用告警规则
			alerts.POST("/rules/:id/move-to-default-group", alertHandler.MoveRuleToDefaultGroup) // 移出规则组（归入默认组）

			// 告警事件
			alerts.GET("/events", alertHandler.GetEvents)                          // 获取告警事件列表
			alerts.GET("/events/:id", alertHandler.GetEvent)                       // 获取告警事件详情
			alerts.POST("/events/:id/claim", alertHandler.ClaimEvent)              // 认领告警
			alerts.POST("/events/:id/cancel-claim", alertHandler.CancelClaimEvent) // 取消认领
			alerts.POST("/events/:id/close", alertHandler.CloseEvent)              // 关闭告警
			alerts.POST("/events/:id/open", alertHandler.OpenEvent)                // 打开告警

			// 告警策略
			alerts.GET("/strategies", alertHandler.GetStrategies)               // 获取告警策略列表
			alerts.GET("/strategies/:id", alertHandler.GetStrategy)             // 获取告警策略详情
			alerts.POST("/strategies", alertHandler.CreateStrategy)             // 创建告警策略
			alerts.PUT("/strategies/:id", alertHandler.UpdateStrategy)          // 更新告警策略
			alerts.DELETE("/strategies/:id", alertHandler.DeleteStrategy)       // 删除告警策略
			alerts.PATCH("/strategies/:id/toggle", alertHandler.ToggleStrategy) // 启用/禁用告警策略

			// 告警静默
			alerts.GET("/silences", alertHandler.GetSilences)          // 获取告警静默列表
			alerts.GET("/silences/:id", alertHandler.GetSilence)       // 获取告警静默详情
			alerts.POST("/silences", alertHandler.CreateSilence)       // 创建告警静默
			alerts.PUT("/silences/:id", alertHandler.UpdateSilence)    // 更新告警静默
			alerts.DELETE("/silences/:id", alertHandler.DeleteSilence) // 删除告警静默

			// 告警聚合
			alerts.GET("/aggregations", alertHandler.GetAggregations)          // 获取告警聚合列表
			alerts.GET("/aggregations/:id", alertHandler.GetAggregation)       // 获取告警聚合详情
			alerts.POST("/aggregations", alertHandler.CreateAggregation)       // 创建告警聚合
			alerts.PUT("/aggregations/:id", alertHandler.UpdateAggregation)    // 更新告警聚合
			alerts.DELETE("/aggregations/:id", alertHandler.DeleteAggregation) // 删除告警聚合

			// 告警抑制
			alerts.GET("/restrains", alertHandler.GetRestrains)          // 获取告警抑制列表
			alerts.GET("/restrains/:id", alertHandler.GetRestrain)       // 获取告警抑制详情
			alerts.POST("/restrains", alertHandler.CreateRestrain)       // 创建告警抑制
			alerts.PUT("/restrains/:id", alertHandler.UpdateRestrain)    // 更新告警抑制
			alerts.DELETE("/restrains/:id", alertHandler.DeleteRestrain) // 删除告警抑制

			// 告警模板
			alerts.GET("/templates", alertHandler.GetTemplates)    // 获取告警模板列表
			alerts.POST("/templates", alertHandler.CreateTemplate) // 创建告警模板

			// 渠道模板内容（必须放在 /templates/:id 之前，避免路由冲突，统一使用 :id 参数名）
			alerts.GET("/templates/:id/channels", alertHandler.GetChannelTemplates)                 // 获取模板的所有渠道模板内容
			alerts.PUT("/templates/:id/channels/:channelId", alertHandler.UpdateChannelTemplate)    // 更新或创建渠道模板内容
			alerts.DELETE("/templates/:id/channels/:channelId", alertHandler.DeleteChannelTemplate) // 删除渠道模板内容

			// 告警模板详情（放在子路由之后，避免路由冲突）
			alerts.GET("/templates/:id", alertHandler.GetTemplate)       // 获取告警模板详情
			alerts.PUT("/templates/:id", alertHandler.UpdateTemplate)    // 更新告警模板
			alerts.DELETE("/templates/:id", alertHandler.DeleteTemplate) // 删除告警模板

			// 告警渠道
			alerts.GET("/channels", alertHandler.GetChannels)          // 获取告警渠道列表
			alerts.GET("/channels/:id", alertHandler.GetChannel)       // 获取告警渠道详情
			alerts.POST("/channels", alertHandler.CreateChannel)       // 创建告警渠道
			alerts.PUT("/channels/:id", alertHandler.UpdateChannel)    // 更新告警渠道
			alerts.DELETE("/channels/:id", alertHandler.DeleteChannel) // 删除告警渠道

			// 策略日志
			alerts.GET("/strategy-logs", alertHandler.GetStrategyLogs)    // 获取策略日志列表
			alerts.GET("/strategy-logs/:id", alertHandler.GetStrategyLog) // 获取策略日志详情

			// 告警等级
			alerts.GET("/levels", alertHandler.GetLevels) // 获取告警等级列表

			// 告警统计
			alerts.GET("/statistics", alertHandler.GetStatistics)            // 获取告警统计信息
			alerts.GET("/statistics/trend", alertHandler.GetTrendStatistics) // 获取告警趋势统计
			alerts.GET("/statistics/top", alertHandler.GetTopAlerts)         // 获取Top N告警

			// 证书管理
			// 域名证书
			alerts.GET("/certificates/domains", alertHandler.GetDomainCertificates)                // 获取域名证书列表
			alerts.POST("/certificates/domains", alertHandler.CreateDomainCertificate)             // 创建域名证书
			alerts.POST("/certificates/domains/check-alerts", alertHandler.CheckCertificateAlerts) // 手动触发证书告警检查
			// 注意：带子路径的路由必须放在 /:id 之前，避免路由冲突
			alerts.POST("/certificates/domains/:id/refresh", alertHandler.RefreshDomainCertificate) // 刷新域名证书信息（通过HTTPS连接获取）
			alerts.GET("/certificates/domains/:id", alertHandler.GetDomainCertificate)              // 获取域名证书详情
			alerts.PUT("/certificates/domains/:id", alertHandler.UpdateDomainCertificate)           // 更新域名证书
			alerts.DELETE("/certificates/domains/:id", alertHandler.DeleteDomainCertificate)        // 删除域名证书

			// SSL证书
			alerts.GET("/certificates/ssl", alertHandler.GetSslCertificates)          // 获取SSL证书列表
			alerts.GET("/certificates/ssl/:id", alertHandler.GetSslCertificate)       // 获取SSL证书详情
			alerts.POST("/certificates/ssl", alertHandler.CreateSslCertificate)       // 创建SSL证书
			alerts.PUT("/certificates/ssl/:id", alertHandler.UpdateSslCertificate)    // 更新SSL证书
			alerts.DELETE("/certificates/ssl/:id", alertHandler.DeleteSslCertificate) // 删除SSL证书

			// 托管证书
			alerts.GET("/certificates/hosted", alertHandler.GetHostedCertificates)          // 获取托管证书列表
			alerts.GET("/certificates/hosted/:id", alertHandler.GetHostedCertificate)       // 获取托管证书详情
			alerts.POST("/certificates/hosted", alertHandler.CreateHostedCertificate)       // 创建托管证书
			alerts.PUT("/certificates/hosted/:id", alertHandler.UpdateHostedCertificate)    // 更新托管证书
			alerts.DELETE("/certificates/hosted/:id", alertHandler.DeleteHostedCertificate) // 删除托管证书
		}

		// Prometheus Webhook（必须使用API Key认证）
		// 认证方式：Authorization: Bearer <api_key> 或 X-API-Key: <api_key> 或 ?api_key=<api_key>
		// 通过API Key自动识别数据源，无需提供source_id
		api.POST("/alerts/webhook/prometheus", alertHandler.WebhookPrometheus) // 接收Prometheus告警

		// 值班排班管理
		oncall := authenticated.Group("/oncall")
		{
			// 排班管理 - 将带子路径的路由放在前面，避免路由冲突
			oncall.GET("/schedules", onCallHandler.ListSchedules)   // 获取排班列表
			oncall.POST("/schedules", onCallHandler.CreateSchedule) // 创建排班

			// 排班的子路由（更具体的路径）需要放在前面
			oncall.GET("/schedules/:id/shifts", onCallHandler.ListShiftsBySchedule)           // 获取排班的班次列表
			oncall.GET("/schedules/:id/current-user", onCallHandler.GetOnCallUserForSchedule) // 获取指定排班的当前值班用户

			// 排班的基本CRUD（放在子路由后面）
			oncall.GET("/schedules/:id", onCallHandler.GetSchedule)       // 获取排班详情
			oncall.PUT("/schedules/:id", onCallHandler.UpdateSchedule)    // 更新排班
			oncall.DELETE("/schedules/:id", onCallHandler.DeleteSchedule) // 删除排班

			// 班次管理
			oncall.GET("/shifts/:id", onCallHandler.GetShift)       // 获取班次详情
			oncall.POST("/shifts", onCallHandler.CreateShift)       // 创建班次
			oncall.PUT("/shifts/:id", onCallHandler.UpdateShift)    // 更新班次
			oncall.DELETE("/shifts/:id", onCallHandler.DeleteShift) // 删除班次

			// 值班查询
			oncall.GET("/current-users", onCallHandler.GetCurrentOnCallUsers) // 获取当前值班用户

			// 告警分配
			oncall.POST("/alerts/:alert_id/auto-assign", onCallHandler.AutoAssignAlert)     // 自动分配告警
			oncall.POST("/alerts/:alert_id/manual-assign", onCallHandler.ManualAssignAlert) // 手动分配告警
			oncall.GET("/alerts/:alert_id/assignment", onCallHandler.GetAssignmentByAlert)  // 获取告警分配信息
			oncall.GET("/users/:user_id/assignments", onCallHandler.ListAssignmentsByUser)  // 获取用户的告警分配列表
		}

		// 账单管理
		bill := authenticated.Group("/bill")
		{
			// 账单明细
			bill.GET("/records", billHandler.GetRecords) // 获取账单明细列表

			// 月度账单
			bill.GET("/summary", billHandler.GetSummary) // 获取月度账单汇总

			// 费用统计
			bill.GET("/statistics", billHandler.GetStatistics)  // 获取费用统计（当月总费用）
			bill.GET("/trend", billHandler.GetTrend)            // 获取费用趋势
			bill.GET("/trend/month", billHandler.GetTrendMonth) // 获取费用趋势月份列表

			// 虚拟机分摊
			bill.GET("/vm", billHandler.GetVM) // 获取虚拟机分摊账单

			// 单价管理
			bill.GET("/price", billHandler.GetPriceList)    // 获取单价列表
			bill.POST("/price", billHandler.CreatePrice)    // 创建单价
			bill.PUT("/price/:id", billHandler.UpdatePrice) // 更新单价

			// 资源管理
			bill.GET("/resource", billHandler.GetResource) // 获取我的资源列表
		}

		// Jenkins管理
		jenkins := authenticated.Group("/jenkins")
		{
			// Jenkins服务器管理路由
			jenkins.GET("/servers", jenkinsHandler.GetJenkinsServers)              // 获取Jenkins服务器列表
			jenkins.POST("/servers", jenkinsHandler.CreateJenkinsServer)           // 创建Jenkins服务器
			jenkins.GET("/servers/:id", jenkinsHandler.GetJenkinsServerDetail)     // 获取Jenkins服务器详情
			jenkins.PUT("/servers/:id", jenkinsHandler.UpdateJenkinsServer)        // 更新Jenkins服务器
			jenkins.DELETE("/servers/:id", jenkinsHandler.DeleteJenkinsServer)     // 删除Jenkins服务器
			jenkins.POST("/test-connection", jenkinsHandler.TestJenkinsConnection) // 测试Jenkins连接

			// 任务管理路由
			jenkins.GET("/:serverId/jobs", jenkinsHandler.GetJobs)               // 获取任务列表
			jenkins.GET("/:serverId/jobs/search", jenkinsHandler.SearchJobs)     // 搜索任务
			jenkins.GET("/:serverId/jobs/:jobName", jenkinsHandler.GetJobDetail) // 获取任务详情

			// Jenkins任务操作路由
			jenkins.POST("/:serverId/jobs/:jobName/start", jenkinsHandler.StartJob) // 启动任务

			// Jenkins构建管理路由
			jenkins.GET("/:serverId/jobs/:jobName/builds/:buildNumber", jenkinsHandler.GetBuildDetail)  // 获取构建详情
			jenkins.POST("/:serverId/jobs/:jobName/builds/:buildNumber/stop", jenkinsHandler.StopBuild) // 停止构建
			jenkins.GET("/:serverId/jobs/:jobName/builds/:buildNumber/log", jenkinsHandler.GetBuildLog) // 获取构建日志

			// Jenkins系统信息路由
			jenkins.GET("/:serverId/system-info", jenkinsHandler.GetSystemInfo) // 获取系统信息
			jenkins.GET("/:serverId/queue", jenkinsHandler.GetQueueInfo)        // 获取队列信息
		}

		// 操作审计
		audit := authenticated.Group("/v1/audit")
		{
			audit.GET("/operation-logs", auditHandler.GetOperationLogs)                  // 获取操作日志列表
			audit.GET("/operation-logs/:id", auditHandler.GetOperationLogDetail)         // 获取操作日志详情
			audit.DELETE("/operation-logs/:id", auditHandler.DeleteOperationLog)         // 删除操作日志
			audit.DELETE("/operation-logs/batch", auditHandler.BatchDeleteOperationLogs) // 批量删除操作日志
			audit.GET("/pod-commands", auditHandler.GetPodCommandLogs)                   // 获取 Pod 命令日志列表
		}

		// 数据库管理 (DMS)
		dms := authenticated.Group("/dms")
		{
			// 实例管理
			dms.GET("/instances", dmsInstanceHandler.ListInstances)                           // 获取实例列表
			dms.POST("/instances", dmsInstanceHandler.CreateInstance)                         // 创建实例
			dms.POST("/instances/test-connection", dmsInstanceHandler.TestConnectionWithBody) // 仅测试连接（不落库）
			dms.GET("/instances/:id", dmsInstanceHandler.GetInstance)                         // 获取实例详情
			dms.PUT("/instances/:id", dmsInstanceHandler.UpdateInstance)                      // 更新实例
			dms.DELETE("/instances/:id", dmsInstanceHandler.DeleteInstance)                   // 删除实例
			dms.POST("/instances/:id/test", dmsInstanceHandler.TestConnection)                // 测试连接（已存在实例）

			// 查询执行
			dms.POST("/query/execute", dmsQueryHandler.ExecuteQuery)  // 执行查询
			dms.GET("/query/databases", dmsQueryHandler.GetDatabases) // 获取数据库列表
			dms.GET("/query/tables", dmsQueryHandler.GetTables)       // 获取表列表

			// 查询日志
			dms.GET("/logs/queries", dmsQueryLogHandler.ListQueryLogs)   // 获取查询日志列表
			dms.GET("/logs/queries/:id", dmsQueryLogHandler.GetQueryLog) // 获取查询日志详情

			// 权限管理
			dms.GET("/permissions", dmsPermissionHandler.GetUserPermissions)                // 获取用户权限列表
			dms.GET("/permissions/my", dmsPermissionHandler.GetMyPermissions)               // 获取我的权限
			dms.POST("/permissions", dmsPermissionHandler.GrantPermission)                  // 分配权限
			dms.POST("/permissions/batch", dmsPermissionHandler.BatchGrantPermissions)      // 批量分配权限
			dms.PUT("/permissions", dmsPermissionHandler.UpdatePermission)                  // 更新权限（只更新元数据）
			dms.PUT("/permissions/resource", dmsPermissionHandler.UpdatePermissionResource) // 更新权限资源路径
			dms.DELETE("/permissions", dmsPermissionHandler.RevokePermission)               // 回收权限
		}

		if aiAssistantHandler != nil {
			aiAssistantHandler.RegisterRoutes(authenticated)
		}
	}

	// 第三方审批平台回调（不需要认证）
	api.POST("/approvals/callback/feishu", approvalCallbackHandler.HandleFeishuCallback)
	api.POST("/approvals/callback/dingtalk", approvalCallbackHandler.HandleDingTalkCallback)
	api.POST("/approvals/callback/wechat", approvalCallbackHandler.HandleWeChatCallback)

	// Prometheus Metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check (支持 GET 和 HEAD 方法)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"type":   "api-server",
		})
	})
	r.HEAD("/health", func(c *gin.Context) {
		c.Status(200)
	})

	// Swagger API documentation (only in debug mode)
	if mode == "debug" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 静态文件由 Nginx 处理，后端不需要提供静态文件服务
	r.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNotFound)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested resource was not found. In separated architecture, static files are served by Nginx.",
		})
	})

	return r
}
