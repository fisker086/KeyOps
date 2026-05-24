package router

import (
	_ "github.com/fisker086/keyops/docs"
	"github.com/fisker086/keyops/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func Setup(d Deps) *gin.Engine {
	r := gin.New()

	r.MaxMultipartMemory = 1 << 30

	r.Use(middleware.RecoveryMiddleware())
	if d.Mode != "release" {
		r.Use(gin.Logger())
	}
	r.Use(middleware.CORS())

	h := d.Handlers
	s := d.Services

	r.GET("/ws/connect", h.Connection.HandleConnection)

	api := r.Group("/api")
	registerPublic(api, h.Auth, h.Setting, h.Release, h.Proxy, h.Blacklist, h.Session)

	authenticated := api.Group("")
	authenticated.Use(middleware.AuthMiddleware(s.Auth))
	{
		registerCore(
			authenticated,
			h.ApiKey,
			h.Host,
			h.Dashboard,
			h.Session,
			h.Proxy,
			h.Auth,
			h.Blacklist,
			h.Setting,
			h.Routing,
			h.HostGroup,
			h.File,
			h.TwoFactor,
		)
		registerOps(
			authenticated,
			h.Approval,
			h.AssetSync,
			h.SystemUser,
			h.Organization,
			h.Application,
			h.AppDeployBinding,
			h.Role,
			h.PermissionRule,
			h.Permission,
			h.FormTemplate,
			h.FormCategory,
			h.Ticket,
			h.TicketDraft,
			h.Workflow,
		)
		registerPlatform(
			authenticated,
			h.K8s,
			h.K8sCluster,
			h.K8sPermission,
			h.K8sSearch,
			h.Deployment,
			h.Release,
			h.BuildMaster,
			h.Registry,
			h.Monitor,
			h.Jenkins,
			h.Audit,
			h.DMSInstance,
			h.DMSQuery,
			h.DMSQueryLog,
			h.DMSPermission,
			h.AiAssistant,
			s.K8sPermission,
			d.RoleRepo,
		)
		registerAlert(api, authenticated, h.Alert, h.OnCall)
		registerBill(authenticated, h.Bill, h.ExpensesMap, h.CloudAccount, h.Resources, h.BillDashboard)
	}

	registerInfra(r, api, authenticated, d)

	return r
}
