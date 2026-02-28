// Package handler 提供统一的 handler 导出
// 所有 handler 按功能模块分类到子目录中
package handler

// 重新导出所有 handler 类型，保持向后兼容
import (
	// Auth handlers
	authHandler "github.com/fisker086/keyops/internal/api/handler/auth"
	// Bastion handlers
	bastionHandler "github.com/fisker086/keyops/internal/api/handler/bastion"
	// Ticket handlers
	ticketHandler "github.com/fisker086/keyops/internal/api/handler/ticket"
	// Workflow handlers
	workflowHandler "github.com/fisker086/keyops/internal/api/handler/workflow"
	// K8s handlers
	k8sHandler "github.com/fisker086/keyops/internal/api/handler/k8s"
	// Monitor handlers
	monitorHandler "github.com/fisker086/keyops/internal/api/handler/monitor"
	// Bill handlers
	billHandler "github.com/fisker086/keyops/internal/api/handler/bill"
	// Alert handlers
	alertHandler "github.com/fisker086/keyops/internal/api/handler/alert"
	// OnCall handlers
	onCallHandler "github.com/fisker086/keyops/internal/api/handler/alert/oncall"
	// Permission handlers
	permissionHandler "github.com/fisker086/keyops/internal/api/handler/permission"
	// System handlers
	systemHandler "github.com/fisker086/keyops/internal/api/handler/system"
	// Jenkins handlers
	jenkinsHandler "github.com/fisker086/keyops/internal/api/handler/jenkins"
	// Audit handlers
	auditHandler "github.com/fisker086/keyops/internal/api/handler/audit"
	// DMS handlers
	dmsHandler "github.com/fisker086/keyops/internal/api/handler/dms"
	// Release (发布代码) handlers
	releaseHandler "github.com/fisker086/keyops/internal/api/handler/release"
	buildmasterHandler "github.com/fisker086/keyops/internal/api/handler/buildmaster"
)

// Auth handlers
type AuthHandler = authHandler.AuthHandler
type TwoFactorHandler = authHandler.TwoFactorHandler

var NewAuthHandler = authHandler.NewAuthHandler
var NewTwoFactorHandler = authHandler.NewTwoFactorHandler

// Bastion handlers
type HostHandler = bastionHandler.HostHandler
type SessionHandler = bastionHandler.SessionHandler
type ConnectionHandler = bastionHandler.ConnectionHandler
type BlacklistHandler = bastionHandler.BlacklistHandler
type SystemUserHandler = bastionHandler.SystemUserHandler
type PermissionRuleHandler = bastionHandler.PermissionRuleHandler
type FileHandler = bastionHandler.FileHandler
type HostGroupHandler = bastionHandler.HostGroupHandler
type RoutingHandler = bastionHandler.RoutingHandler
type ProxyHandler = bastionHandler.ProxyHandler
type DashboardHandler = bastionHandler.DashboardHandler
type HostMonitorHandler = bastionHandler.HostMonitorHandler

var NewHostHandler = bastionHandler.NewHostHandler
var NewSessionHandler = bastionHandler.NewSessionHandler
var NewConnectionHandler = bastionHandler.NewConnectionHandler
var NewBlacklistHandler = bastionHandler.NewBlacklistHandler
var NewSystemUserHandler = bastionHandler.NewSystemUserHandler
var NewPermissionRuleHandler = bastionHandler.NewPermissionRuleHandler
var NewFileHandler = bastionHandler.NewFileHandler
var NewHostGroupHandler = bastionHandler.NewHostGroupHandler
var NewRoutingHandler = bastionHandler.NewRoutingHandler
var NewProxyHandler = bastionHandler.NewProxyHandler
var NewDashboardHandler = bastionHandler.NewDashboardHandler
var NewHostMonitorHandler = bastionHandler.NewHostMonitorHandler

// Ticket handlers
type TicketHandler = ticketHandler.TicketHandler
type TicketDraftHandler = ticketHandler.TicketDraftHandler
type FormTemplateHandler = ticketHandler.FormTemplateHandler
type FormCategoryHandler = ticketHandler.FormCategoryHandler
type ApprovalHandler = ticketHandler.ApprovalHandler
type ApprovalCallbackHandler = ticketHandler.ApprovalCallbackHandler
type WorkflowHandler = workflowHandler.WorkflowHandler

var NewTicketHandler = ticketHandler.NewTicketHandler
var NewTicketDraftHandler = ticketHandler.NewTicketDraftHandler
var NewFormTemplateHandler = ticketHandler.NewFormTemplateHandler
var NewFormCategoryHandler = ticketHandler.NewFormCategoryHandler
var NewApprovalHandler = ticketHandler.NewApprovalHandler
var NewApprovalCallbackHandler = ticketHandler.NewApprovalCallbackHandler
var NewWorkflowHandler = workflowHandler.NewWorkflowHandler

// K8s handlers
type K8sHandler = k8sHandler.K8sHandler
type K8sClusterHandler = k8sHandler.K8sClusterHandler
type K8sPermissionHandler = k8sHandler.K8sPermissionHandler
type K8sSearchHandler = k8sHandler.SearchHandler
type DeploymentHandler = k8sHandler.DeploymentHandler

var NewK8sHandler = k8sHandler.NewK8sHandler
var NewK8sClusterHandler = k8sHandler.NewK8sClusterHandler
var NewK8sPermissionHandler = k8sHandler.NewK8sPermissionHandler
var NewK8sSearchHandler = k8sHandler.NewSearchHandler
var NewDeploymentHandler = k8sHandler.NewDeploymentHandler

// Monitor handlers
type MonitorHandler = monitorHandler.MonitorHandler

var NewMonitorHandler = monitorHandler.NewMonitorHandler

// Alert handlers
type AlertHandler = alertHandler.AlertHandler

var NewAlertHandler = alertHandler.NewAlertHandler

// OnCall handlers
type OnCallHandler = onCallHandler.OnCallHandler

var NewOnCallHandler = onCallHandler.NewOnCallHandler

// Bill handlers
type BillHandler = billHandler.BillHandler

var NewBillHandler = billHandler.NewBillHandler

// Permission handlers
type PermissionHandler = permissionHandler.PermissionHandler
type RoleHandler = permissionHandler.RoleHandler

var NewPermissionHandler = permissionHandler.NewPermissionHandler
var NewRoleHandler = permissionHandler.NewRoleHandler

// System handlers
type SettingHandler = systemHandler.SettingHandler
type AssetSyncHandler = systemHandler.AssetSyncHandler
type OrganizationHandler = systemHandler.OrganizationHandler
type ApplicationHandler = systemHandler.ApplicationHandler
type AppDeployBindingHandler = systemHandler.AppDeployBindingHandler
type RegistryHandler = systemHandler.RegistryHandler

var NewSettingHandler = systemHandler.NewSettingHandler
var NewAssetSyncHandler = systemHandler.NewAssetSyncHandler
var NewOrganizationHandler = systemHandler.NewOrganizationHandler
var NewApplicationHandler = systemHandler.NewApplicationHandler
var NewAppDeployBindingHandler = systemHandler.NewAppDeployBindingHandler
var NewRegistryHandler = systemHandler.NewRegistryHandler

// Jenkins handlers
type JenkinsHandler = jenkinsHandler.JenkinsHandler

var NewJenkinsHandler = jenkinsHandler.NewJenkinsHandler

// Audit handlers
type AuditHandler = auditHandler.AuditHandler

var NewAuditHandler = auditHandler.NewAuditHandler

// DMS handlers
type DMSInstanceHandler = dmsHandler.InstanceHandler
type DMSQueryHandler = dmsHandler.QueryHandler
type DMSQueryLogHandler = dmsHandler.QueryLogHandler
type DMSPermissionHandler = dmsHandler.PermissionHandler

var NewDMSInstanceHandler = dmsHandler.NewInstanceHandler
var NewDMSQueryHandler = dmsHandler.NewQueryHandler
var NewDMSQueryLogHandler = dmsHandler.NewQueryLogHandler
var NewDMSPermissionHandler = dmsHandler.NewPermissionHandler

// Release handlers (发布代码)
type ReleaseHandler = releaseHandler.ReleaseHandler

var NewReleaseHandler = releaseHandler.NewReleaseHandler

type BuildMasterHandler = buildmasterHandler.BuildMasterHandler

var NewBuildMasterHandler = buildmasterHandler.NewBuildMasterHandler
