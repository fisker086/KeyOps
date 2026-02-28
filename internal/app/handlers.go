package app

import (
	"context"

	"github.com/fisker086/keyops/internal/aiassistant"
	aitemporal "github.com/fisker086/keyops/internal/aiassistant/temporal"
	"github.com/fisker086/keyops/internal/aiassistant/tools"
	"github.com/fisker086/keyops/internal/aiassistant/tools/k8s"
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/audit"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/routing"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"gorm.io/gorm"
)

// Handlers 包含所有 Handler 实例
type Handlers struct {
	Host             *handler.HostHandler
	Dashboard        *handler.DashboardHandler
	Session          *handler.SessionHandler
	Proxy            *handler.ProxyHandler
	Auth             *handler.AuthHandler
	Blacklist        *handler.BlacklistHandler
	Setting          *handler.SettingHandler
	Routing          *handler.RoutingHandler
	Connection       *handler.ConnectionHandler
	HostGroup        *handler.HostGroupHandler
	Approval         *handler.ApprovalHandler
	ApprovalCallback *handler.ApprovalCallbackHandler
	File             *handler.FileHandler
	AssetSync        *handler.AssetSyncHandler
	HostMonitor      *handler.HostMonitorHandler
	SystemUser       *handler.SystemUserHandler
	Role             *handler.RoleHandler
	PermissionRule   *handler.PermissionRuleHandler
	TwoFactor        *handler.TwoFactorHandler
	Permission       *handler.PermissionHandler
	FormTemplate     *handler.FormTemplateHandler
	FormCategory     *handler.FormCategoryHandler
	Ticket           *handler.TicketHandler
	TicketDraft      *handler.TicketDraftHandler
	Workflow         *handler.WorkflowHandler
	K8s              *handler.K8sHandler
	K8sCluster       *handler.K8sClusterHandler
	K8sPermission    *handler.K8sPermissionHandler
	K8sSearch        *handler.K8sSearchHandler
	Deployment       *handler.DeploymentHandler
	Bill             *handler.BillHandler
	Monitor          *handler.MonitorHandler
	Organization     *handler.OrganizationHandler
	Application          *handler.ApplicationHandler
	AppDeployBinding     *handler.AppDeployBindingHandler
	Registry             *handler.RegistryHandler
	Jenkins              *handler.JenkinsHandler
	Audit                *handler.AuditHandler
	Alert            *handler.AlertHandler
	OnCall           *handler.OnCallHandler
	DMSInstance      *handler.DMSInstanceHandler
	DMSQuery         *handler.DMSQueryHandler
	DMSQueryLog      *handler.DMSQueryLogHandler
	DMSPermission    *handler.DMSPermissionHandler
	Release          *handler.ReleaseHandler
	BuildMaster      *handler.BuildMasterHandler
	AiAssistant      *aiassistant.Handler
}

// InitializeHandlers 初始化所有 Handler
func InitializeHandlers(
	repos *Repositories,
	services *Services,
	backgroundServices *BackgroundServices,
	notificationMgr *notification.NotificationManager,
	unifiedAuditor *audit.DatabaseAuditor,
) *Handlers {
	// Create adapters for backward compatibility
	st := audit.NewWebShellStorageAdapter(unifiedAuditor)
	logger.Infof("WebShell Storage Adapter created")

	// Initialize connection router
	connectionRouter := routing.NewConnectionRouter(
		repos.Host,
		repos.Proxy,
		repos.Setting,
	)

	// Initialize approval factory
	approvalFactory := approval.NewFactory()
	loadApprovalProviders(database.DB, approvalFactory)

	// Initialize handlers
	hostHandler := handler.NewHostHandler(services.Host)
	dashboardHandler := handler.NewDashboardHandler(services.Host, services.Session)
	sessionHandler := handler.NewSessionHandler(services.Session)
	proxyHandler := handler.NewProxyHandler(database.DB)
	authHandler := handler.NewAuthHandler(services.Auth, repos.Setting, repos.Role)
	blacklistHandler := handler.NewBlacklistHandler(database.DB)
	settingHandler := handler.NewSettingHandler(repos.Setting, notificationMgr)
	routingHandler := handler.NewRoutingHandler(connectionRouter, repos.Setting, repos.Host, repos.Proxy)
	connectionHandler := handler.NewConnectionHandler(
		connectionRouter,
		repos.Host,
		services.Auth,
		st,
		database.DB,
		notificationMgr,
		repos.SystemUser,
		repos.Setting,
	)
	hostGroupHandler := handler.NewHostGroupHandler(repos.HostGroup, repos.Host, repos.User)
	approvalHandler := handler.NewApprovalHandler(database.DB, approvalFactory, services.Release)
	approvalCallbackHandler := handler.NewApprovalCallbackHandler(database.DB)
	fileHandler := handler.NewFileHandler(database.DB, repos.Host, repos.SystemUser)
	assetSyncHandler := handler.NewAssetSyncHandler(repos.AssetSync, services.AssetSync)
	hostMonitorHandler := handler.NewHostMonitorHandler(backgroundServices.HostMonitor)
	systemUserHandler := handler.NewSystemUserHandler(repos.SystemUser)
	roleHandler := handler.NewRoleHandler(repos.Role)
	permissionRuleHandler := handler.NewPermissionRuleHandler(repos.PermissionRule)
	twoFactorHandler := handler.NewTwoFactorHandler(database.DB, services.Auth.TwoFactorSvc)
	permissionHandler := handler.NewPermissionHandler(repos.Menu, repos.API, repos.Role)
	formTemplateHandler := handler.NewFormTemplateHandler(database.DB)
	formCategoryHandler := handler.NewFormCategoryHandler(database.DB)
	ticketHandler := handler.NewTicketHandler(database.DB)
	ticketDraftHandler := handler.NewTicketDraftHandler(database.DB)
	workflowHandler := handler.NewWorkflowHandler(database.DB)
	k8sHandler := handler.NewK8sHandler(services.K8s, services.K8sPermission, repos.Role, services.Auth)
	k8sClusterHandler := handler.NewK8sClusterHandler(services.K8sCluster, services.K8sPermission, repos.Role)
	k8sPermissionHandler := handler.NewK8sPermissionHandler(services.K8sPermission, services.K8sCluster, repos.Role)
	k8sSearchHandler := handler.NewK8sSearchHandler(services.K8sCluster, services.K8s, services.K8sPermission, repos.Role)
	deploymentHandler := handler.NewDeploymentHandler(services.Deployment, services.K8sPermission, repos.Role)
	billHandler := handler.NewBillHandler(services.Bill)
	monitorHandler := handler.NewMonitorHandler(services.Monitor)
	organizationHandler := handler.NewOrganizationHandler(repos.Organization)
	applicationHandler := handler.NewApplicationHandler(repos.Application)
	appDeployBindingHandler := handler.NewAppDeployBindingHandler(repos.AppDeployBinding, repos.Application)
	registryHandler := handler.NewRegistryHandler(repos.Application, services.Registry)
	jenkinsHandler := handler.NewJenkinsHandler(services.Jenkins)
	auditHandler := handler.NewAuditHandler()
	alertHandler := handler.NewAlertHandler(services.Alert, notificationMgr)
	alertHandler.SetCertificateRepositories(repos.DomainCertificate, repos.SSLCertificate, repos.HostedCertificate)
	alertHandler.SetCertificateAlertService(backgroundServices.CertificateAlert)
	onCallHandler := handler.NewOnCallHandler(services.OnCall)

	// DMS handlers
	dmsInstanceHandler := handler.NewDMSInstanceHandler(services.DMSInstance)
	dmsQueryHandler := handler.NewDMSQueryHandler(services.DMSQuery)
	dmsQueryLogHandler := handler.NewDMSQueryLogHandler(repos.QueryLog)
	dmsPermissionHandler := handler.NewDMSPermissionHandler(services.DMSPermission)
	releaseHandler := handler.NewReleaseHandler(services.Release, repos.ReleasePipelineDef, repos.AppDeployBinding)
	buildMasterHandler := handler.NewBuildMasterHandler(repos.BuildMaster)

	// AI运维助手（纯 Go 实现）：注入 GetUserRoleIDs 用于专家与平台角色关联（列表过滤与创建任务校验）
	var aiAssistantHandler *aiassistant.Handler
	if config.GlobalConfig != nil {
		dataDir := "data/ai_assistant"
		sessionMgr, err := aiassistant.NewSessionManager(dataDir+"/sessions", config.GlobalConfig.Security.JWTSecret)
		if err == nil {
			scheduleMgr, _ := aiassistant.NewScheduleManager(dataDir)
			envMgr := aiassistant.NewEnvManager()
			var store *aiassistant.Store
			if database.DB != nil {
				store = aiassistant.NewStore(database.DB)
				if err := store.AutoMigrate(); err != nil {
					logger.Warnf("AI Assistant store migrate: %v", err)
				}
			}
			var getUserRoleIDs aiassistant.GetUserRoleIDsFunc
			if repos.Role != nil {
				getUserRoleIDs = func(userID string) ([]string, error) {
					roles, err := repos.Role.GetRolesByUserID(userID)
					if err != nil {
						return nil, err
					}
					ids := make([]string, len(roles))
					for i := range roles {
						ids[i] = roles[i].ID
					}
					return ids, nil
				}
			}
			runners := make(map[string]tools.Runner)
			if services.K8s != nil && services.K8sCluster != nil {
				if r := NewK8sToolRunner(services.K8s, services.K8sCluster); r != nil {
					runners[k8s.ID] = r
				}
			}
			aiAssistantHandler = aiassistant.NewHandler(sessionMgr, scheduleMgr, envMgr, store, config.GlobalConfig.Security.JWTSecret, getUserRoleIDs, runners, repos.Setting)
		}
		if aiAssistantHandler != nil {
			if backgroundServices != nil && backgroundServices.InspectionReportSender != nil {
				aiAssistantHandler.SetInspectionReportSender(backgroundServices.InspectionReportSender)
			}
			if starter := NewTemporalScheduleStarter(); starter != nil {
				aiAssistantHandler.SetScheduleWorkflowStarter(starter)
				aitemporal.RunWorkerInBackground(aiAssistantHandler)
				logger.Infof("AI Assistant schedule workflow: Temporal enabled (TEMPORAL_HOST + TEMPORAL_TASK_QUEUE)")
			}
			go aiassistant.NewCronScheduler(aiAssistantHandler).Run(context.Background())
			logger.Infof("AI Assistant (aiassistant) handler initialized (cron scheduler started)")
		}
	}

	// Set cross-references
	settingHandler.SetHostMonitor(backgroundServices.HostMonitor)
	hostHandler.SetMonitorService(backgroundServices.HostMonitor)

	return &Handlers{
		Host:             hostHandler,
		Dashboard:        dashboardHandler,
		Session:          sessionHandler,
		Proxy:            proxyHandler,
		Auth:             authHandler,
		Blacklist:        blacklistHandler,
		Setting:          settingHandler,
		Routing:          routingHandler,
		Connection:       connectionHandler,
		HostGroup:        hostGroupHandler,
		Approval:         approvalHandler,
		ApprovalCallback: approvalCallbackHandler,
		File:             fileHandler,
		AssetSync:        assetSyncHandler,
		HostMonitor:      hostMonitorHandler,
		SystemUser:       systemUserHandler,
		Role:             roleHandler,
		PermissionRule:   permissionRuleHandler,
		TwoFactor:        twoFactorHandler,
		Permission:       permissionHandler,
		FormTemplate:     formTemplateHandler,
		FormCategory:     formCategoryHandler,
		Ticket:           ticketHandler,
		TicketDraft:      ticketDraftHandler,
		Workflow:         workflowHandler,
		K8s:              k8sHandler,
		K8sCluster:       k8sClusterHandler,
		K8sPermission:    k8sPermissionHandler,
		K8sSearch:        k8sSearchHandler,
		Deployment:       deploymentHandler,
		Bill:             billHandler,
		Monitor:          monitorHandler,
		Organization:     organizationHandler,
		Application:          applicationHandler,
		AppDeployBinding:     appDeployBindingHandler,
		Registry:             registryHandler,
		Jenkins:              jenkinsHandler,
		Audit:                auditHandler,
		Alert:            alertHandler,
		OnCall:           onCallHandler,
		DMSInstance:      dmsInstanceHandler,
		DMSQuery:         dmsQueryHandler,
		DMSQueryLog:      dmsQueryLogHandler,
		DMSPermission:    dmsPermissionHandler,
		Release:          releaseHandler,
		BuildMaster:      buildMasterHandler,
		AiAssistant:      aiAssistantHandler,
	}
}

// loadApprovalProviders loads approval configurations from database and registers providers
func loadApprovalProviders(db *gorm.DB, factory *approval.Factory) {
	var configs []model.ApprovalConfig
	if err := db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		logger.Warnf("Failed to load approval configurations: %v", err)
		return
	}

	logger.Infof("Found %d enabled approval configurations", len(configs))

	for _, config := range configs {
		logger.Infof("Processing config: %s (type: %s, app_id: %s, approval_code: %s)",
			config.Name, config.Type, config.AppID, config.ApprovalCode)

		switch config.Type {
		case "feishu":
			if config.AppID != "" && config.AppSecret != "" && config.ApprovalCode != "" {
				provider := approval.NewFeishuProvider(&config, database.DB)
				factory.Register(model.ApprovalPlatformFeishu, provider)
				logger.Infof("Registered Feishu approval provider: %s", config.Name)
			} else {
				logger.Warnf("Feishu config incomplete: app_id=%s, app_secret=%s, approval_code=%s",
					config.AppID, config.AppSecret, config.ApprovalCode)
			}
		case "dingtalk":
			if config.AppID != "" && config.AppSecret != "" && config.ProcessCode != "" {
				provider := approval.NewDingTalkProvider(&config, database.DB)
				factory.Register(model.ApprovalPlatformDingTalk, provider)
				logger.Infof("Registered DingTalk approval provider: %s", config.Name)
			} else {
				logger.Warnf("DingTalk config incomplete: app_id=%s, app_secret=%s, process_code=%s",
					config.AppID, config.AppSecret, config.ProcessCode)
			}
		case "wechat":
			if config.AppID != "" && config.AppSecret != "" && config.TemplateID != "" {
				provider := approval.NewWeChatProvider(&config, database.DB)
				factory.Register(model.ApprovalPlatformWeChat, provider)
				logger.Infof("Registered WeChat approval provider: %s", config.Name)
			} else {
				logger.Warnf("WeChat config incomplete: app_id=%s, app_secret=%s, template_id=%s",
					config.AppID, config.AppSecret, config.TemplateID)
			}
		}
	}
}
