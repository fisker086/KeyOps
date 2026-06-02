package router

import (
	"github.com/fisker086/keyops/internal/aiassistant"
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/fisker086/keyops/internal/mcp"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service"
)

// Deps bundles handlers and services required to build the HTTP router.
type Deps struct {
	Mode     string
	Handlers HandlerSet
	Services ServiceSet
	RoleRepo repository.RoleRepository
}

// HandlerSet groups HTTP handlers passed to route registrars.
type HandlerSet struct {
	Mcp              *mcp.Handler
	ApiKey           *handler.ApiKeyHandler
	Host             *handler.HostHandler
	Dashboard        *handler.DashboardHandler
	Session          *handler.SessionHandler
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
	ExpensesMap      *handler.ExpensesMapHandler
	CloudAccount     *handler.CloudAccountHandler
	Resources        *handler.ResourcesHandler
	BillDashboard    *handler.BillDashboardHandler
	Monitor          *handler.MonitorHandler
	Organization     *handler.OrganizationHandler
	Environment      *handler.EnvironmentHandler
	Application      *handler.ApplicationHandler
	Registry         *handler.RegistryHandler
	Audit            *handler.AuditHandler
	Alert            *handler.AlertHandler
	OnCall           *handler.OnCallHandler
	DMSInstance      *handler.DMSInstanceHandler
	DMSQuery         *handler.DMSQueryHandler
	DMSQueryLog      *handler.DMSQueryLogHandler
	DMSPermission    *handler.DMSPermissionHandler
	Release          *handler.ReleaseHandler
	BuildMaster      *handler.BuildMasterHandler
	DeployParam      *handler.DeployParamHandler
	AppDeployParam   *handler.AppDeployParamHandler
	AiAssistant      *aiassistant.Handler
}

// ServiceSet groups services used by middleware and route registrars.
type ServiceSet struct {
	ApiKey        *service.ApiKeyService
	Auth          *service.AuthService
	K8sPermission *service.K8sPermissionService
}
