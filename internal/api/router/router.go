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
	r.Use(middleware.AccessLogMiddleware())
	if d.Mode != "release" {
		r.Use(gin.Logger())
	}
	r.Use(middleware.CORS())

	h := d.Handlers
	s := d.Services

	r.GET("/ws/connect", h.Connection.HandleConnection)

	api := r.Group("/api")
	registerPublic(api, h.Auth, h.Setting, h.Release)

	authenticated := api.Group("")
	authenticated.Use(middleware.AuthMiddleware(s.Auth))
	{
		registerAuthCore(authenticated, h.ApiKey, h.Auth, h.Setting, h.TwoFactor)
		registerBastionCore(authenticated, h.Host, h.Dashboard, h.Session, h.Blacklist, h.Routing, h.HostGroup, h.File)
		registerOps(
			authenticated,
			h.Approval,
			h.AssetSync,
			h.SystemUser,
			h.Organization,
			h.Environment,
			h.Application,
			h.Role,
			h.PermissionRule,
			h.Permission,
			h.FormTemplate,
			h.FormCategory,
			h.Ticket,
			h.TicketDraft,
			h.Workflow,
			h.AppDeployParam,
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
			h.DeployParam,
			h.Registry,
			h.Monitor,
			h.Audit,
			h.DMSInstance,
			h.DMSQuery,
			h.DMSQueryLog,
			h.DMSPermission,
			h.AiAssistant,
			s.K8sPermission,
			d.RoleRepo,
			h.AppDeployParam,
		)
		registerAlert(api, authenticated, h.Alert, h.OnCall)
		registerBill(authenticated, h.Bill, h.CloudAccount, h.Resources, h.BillDashboard)
	}

	registerMCP(r, api, authenticated, d)
	registerCallbacks(api, d)
	registerInfra(r, api, d)

	return r
}
