// Package service 提供统一的 service 导出
// 所有 service 按功能模块分类到子目录中
package service

// 重新导出所有 service 类型，保持向后兼容
import (
	// Auth services
	authService "github.com/fisker086/keyops/internal/service/auth"
	// Bastion services
	bastionService "github.com/fisker086/keyops/internal/service/bastion"
	// K8s services
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	// Monitor services
	monitorService "github.com/fisker086/keyops/internal/service/monitor"
	// Alert services
	alertService "github.com/fisker086/keyops/internal/alert/service"
	// OnCall services
	onCallService "github.com/fisker086/keyops/internal/alert/oncall/service"
	// Bill services
	billService "github.com/fisker086/keyops/internal/service/bill"
	// System services
	systemService "github.com/fisker086/keyops/internal/service/system"
	// Jenkins services
	jenkinsService "github.com/fisker086/keyops/internal/service/jenkins"
)

// Auth services
type AuthService = authService.AuthService

var NewAuthService = authService.NewAuthService

// Bastion services
type HostService = bastionService.HostService
type SessionService = bastionService.SessionService
type HostMonitorService = bastionService.HostMonitorService
type SessionToken = bastionService.SessionToken

var NewHostService = bastionService.NewHostService
var NewSessionService = bastionService.NewSessionService
var NewHostMonitorService = bastionService.NewHostMonitorService
var ValidateSessionToken = bastionService.ValidateSessionToken

// K8s services
type K8sService = k8sService.K8sService
type K8sClusterService = k8sService.K8sClusterService
type K8sPermissionService = k8sService.K8sPermissionService
type DeploymentService = k8sService.DeploymentService
type KubeDogService = k8sService.KubeDogService
type CreateDeploymentRequest = k8sService.CreateDeploymentRequest

var NewK8sService = k8sService.NewK8sService
var NewK8sClusterService = k8sService.NewK8sClusterService
var NewK8sPermissionService = k8sService.NewK8sPermissionService
var NewDeploymentService = k8sService.NewDeploymentService
var NewKubeDogService = k8sService.NewKubeDogService

// Monitor services
type MonitorService = monitorService.MonitorService

var NewMonitorService = monitorService.NewMonitorService

// Alert services
type AlertService = alertService.AlertService

var NewAlertService = alertService.NewAlertService

// OnCall services
type OnCallService = onCallService.OnCallService

var NewOnCallService = onCallService.NewOnCallService

// Bill services
type BillService = billService.BillService

var NewBillService = billService.NewBillService

// System services
type AssetSyncService = systemService.AssetSyncService
type ExpirationService = systemService.ExpirationService
type ProxyMonitor = systemService.ProxyMonitor
type MonitorConfig = systemService.MonitorConfig

var NewAssetSyncService = systemService.NewAssetSyncService
var NewExpirationService = systemService.NewExpirationService
var NewProxyMonitor = systemService.NewProxyMonitor

// Jenkins services
type JenkinsService = jenkinsService.JenkinsService

var NewJenkinsService = jenkinsService.NewJenkinsService
