package app

import (
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/database"
)

// Repositories 包含所有 Repository 实例
type Repositories struct {
	Host             *repository.HostRepository
	Session          *repository.SessionRepository
	User             *repository.UserRepository
	Setting          *repository.SettingRepository
	Proxy            *repository.ProxyRepository
	Role             *repository.RoleRepository
	SystemUser       *repository.SystemUserRepository
	HostGroup        *repository.HostGroupRepository
	PermissionRule   *repository.PermissionRuleRepository
	Menu             *repository.MenuRepository
	API              *repository.APIRepository
	AssetSync        *repository.AssetSyncRepository
	K8sCluster       *repository.K8sClusterRepository
	Deployment       *repository.DeploymentRepository
	Bill             *repository.BillRepository
	Monitor          *repository.MonitorRepository
	Organization     *repository.OrganizationRepository
	Application          *repository.ApplicationRepository
	AppDeployBinding     *repository.ApplicationDeployBindingRepository
	Jenkins              *repository.JenkinsRepository
	AlertRuleGroup       *repository.AlertRuleGroupRepository
	AlertRuleSource  *repository.AlertRuleSourceRepository
	AlertRule        *repository.AlertRuleRepository
	AlertEvent       *repository.AlertEventRepository
	AlertLog         *repository.AlertLogRepository
	AlertStrategy    *repository.AlertStrategyRepository
	AlertLevel       *repository.AlertLevelRepository
	AlertAggregation *repository.AlertAggregationRepository
	AlertSilence     *repository.AlertSilenceRepository
	AlertRestrain    *repository.AlertRestrainRepository
	AlertTemplate    *repository.AlertTemplateRepository
	AlertChannel     *repository.AlertChannelRepository
	AlertGroup       *repository.AlertGroupRepository
	ChannelTemplate  *repository.ChannelTemplateRepository
	StrategyLog      *repository.StrategyLogRepository
	OnCallSchedule   *repository.OnCallScheduleRepository
	OnCallShift      *repository.OnCallShiftRepository
	OnCallAssignment *repository.OnCallAssignmentRepository
	DBInstance            *repository.DBInstanceRepository
	QueryLog              *repository.QueryLogRepository
	DBPermission          *repository.DBPermissionRepository
	DomainCertificate     *repository.DomainCertificateRepository
	SSLCertificate        *repository.SSLCertificateRepository
	HostedCertificate     *repository.HostedCertificateRepository
	ReleaseRun            *repository.ReleaseRunRepository
	ReleasePipelineDef    *repository.ReleasePipelineDefinitionRepository
	BuildMaster           *repository.BuildMasterRepository
}

// InitializeRepositories 初始化所有 Repository
func InitializeRepositories() *Repositories {
	return &Repositories{
		Host:             repository.NewHostRepository(database.DB),
		Session:          repository.NewSessionRepository(database.DB),
		User:             repository.NewUserRepository(database.DB),
		Setting:          repository.NewSettingRepository(database.DB),
		Proxy:            repository.NewProxyRepository(database.DB),
		Role:             repository.NewRoleRepository(database.DB),
		SystemUser:       repository.NewSystemUserRepository(database.DB),
		HostGroup:        repository.NewHostGroupRepository(database.DB),
		PermissionRule:   repository.NewPermissionRuleRepository(database.DB),
		Menu:             repository.NewMenuRepository(database.DB),
		API:              repository.NewAPIRepository(database.DB),
		AssetSync:        repository.NewAssetSyncRepository(database.DB),
		K8sCluster:       repository.NewK8sClusterRepository(database.DB),
		Deployment:       repository.NewDeploymentRepository(database.DB),
		Bill:             repository.NewBillRepository(database.DB),
		Monitor:          repository.NewMonitorRepository(database.DB),
		Organization:     repository.NewOrganizationRepository(database.DB),
		Application:          repository.NewApplicationRepository(database.DB),
		AppDeployBinding:     repository.NewApplicationDeployBindingRepository(database.DB),
		Jenkins:              repository.NewJenkinsRepository(database.DB),
		AlertRuleSource:      repository.NewAlertRuleSourceRepository(database.DB),
		AlertRule:        repository.NewAlertRuleRepository(database.DB),
		AlertEvent:       repository.NewAlertEventRepository(database.DB),
		AlertLog:         repository.NewAlertLogRepository(database.DB),
		AlertStrategy:    repository.NewAlertStrategyRepository(database.DB),
		AlertLevel:       repository.NewAlertLevelRepository(database.DB),
		AlertAggregation: repository.NewAlertAggregationRepository(database.DB),
		AlertSilence:     repository.NewAlertSilenceRepository(database.DB),
		AlertRestrain:    repository.NewAlertRestrainRepository(database.DB),
		AlertTemplate:    repository.NewAlertTemplateRepository(database.DB),
		AlertChannel:     repository.NewAlertChannelRepository(database.DB),
		AlertGroup:       repository.NewAlertGroupRepository(database.DB),
		ChannelTemplate:  repository.NewChannelTemplateRepository(database.DB),
		StrategyLog:      repository.NewStrategyLogRepository(database.DB),
		OnCallSchedule:   repository.NewOnCallScheduleRepository(database.DB),
		OnCallShift:      repository.NewOnCallShiftRepository(database.DB),
		OnCallAssignment: repository.NewOnCallAssignmentRepository(database.DB),
		AlertRuleGroup:   repository.NewAlertRuleGroupRepository(database.DB),
		DBInstance:        repository.NewDBInstanceRepository(database.DB),
		QueryLog:          repository.NewQueryLogRepository(database.DB),
		DBPermission:      repository.NewDBPermissionRepository(database.DB),
		DomainCertificate: repository.NewDomainCertificateRepository(database.DB),
		SSLCertificate:    repository.NewSSLCertificateRepository(database.DB),
		HostedCertificate: repository.NewHostedCertificateRepository(database.DB),
		ReleaseRun:        repository.NewReleaseRunRepository(database.DB),
		ReleasePipelineDef: repository.NewReleasePipelineDefinitionRepository(database.DB),
		BuildMaster:       repository.NewBuildMasterRepository(database.DB),
	}
}
