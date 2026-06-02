package app

import (
	"context"
	"os"
	"time"

	"github.com/fisker086/keyops/internal/aiassistant"
	alertnotification "github.com/fisker086/keyops/internal/alert/notification"
	oncallNotification "github.com/fisker086/keyops/internal/alert/oncall"
	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/infrastructure/mongodb"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/internal/service/auth"
	bastionService "github.com/fisker086/keyops/internal/service/bastion"
	billService "github.com/fisker086/keyops/internal/service/bill"
	certificateService "github.com/fisker086/keyops/internal/service/certificate"
	"github.com/fisker086/keyops/internal/service/dms"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/internal/service/registry"
	release "github.com/fisker086/keyops/internal/service/release"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/crypto"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"go.mongodb.org/mongo-driver/mongo"
)

// Services 包含所有 Service 实例
type Services struct {
	ApiKey        *service.ApiKeyService
	Host          *service.HostService
	Session       *service.SessionService
	Auth          *service.AuthService
	AssetSync     *service.AssetSyncService
	K8s           *service.K8sService
	K8sCluster    *service.K8sClusterService
	K8sPermission *service.K8sPermissionService
	Deployment    *service.DeploymentService
	Bill          *service.BillService
	Monitor       *service.MonitorService
	Alert         *service.AlertService
	OnCall        *service.OnCallService
	DMSInstance   *dms.InstanceService
	DMSQuery      *dms.QueryService
	DMSPermission *dms.PermissionService
	Release       *release.Service
	Registry      *registry.Service
	MongoClient   *mongodb.Client
}

// InitializeServices 初始化所有 Service
func InitializeServices(repos *Repositories, cfg *config.Config, mongoClient *mongodb.Client) *Services {
	hostService := service.NewHostService(repos.Host)
	sessionService := service.NewSessionService(repos.Session, repos.Host)

	// 初始化 MongoDB 连接（用于账单原始数据存储）
	if mongoClient == nil && cfg.BillStorage.GetURI() != "" {
		mongoClient2, err := mongodb.NewClientWithDatabase(cfg.BillStorage.GetURI(), cfg.BillStorage.Database)
		if err != nil {
			logger.Warnf("MongoDB connect failed: %v, AWS bill sync will not work", err)
			mongoClient2 = nil
		} else {
			if err := mongoClient2.InitIndexes(context.Background()); err != nil {
				logger.Warnf("MongoDB init indexes failed: %v", err)
			}
		}
		mongoClient = mongoClient2
	}
	var mongoColl *mongo.Collection
	if mongoClient != nil {
		mongoColl = mongoClient.RawExpenses()
	}
	billSvc := billService.NewBillService(repos.Bill, repos.CloudAccount, repos.AlertChannel, mongoColl)

	// Session 存储引擎已在 Repositories 初始化时根据配置选定 (repos.Session)

	monitorService := service.NewMonitorService(repos.Monitor)
	cryptoService := crypto.NewCrypto(cfg.Security.JWTSecret)
	// 规则文件目录：不设则仅在本系统内记录规则/规则组（DB），不写本地文件、不触发 Prometheus 挂载；若需写文件可设置 ALERT_RULE_DIR
	ruleDir := os.Getenv("ALERT_RULE_DIR")
	alertService := service.NewAlertService(
		repos.AlertRuleGroup,
		repos.AlertRuleSource,
		repos.AlertRule,
		repos.AlertEvent,
		repos.AlertLog,
		repos.AlertStrategy,
		repos.AlertLevel,
		repos.AlertAggregation,
		repos.AlertSilence,
		repos.AlertRestrain,
		repos.AlertTemplate,
		repos.AlertChannel,
		repos.ChannelTemplate,
		repos.AlertGroup,
		repos.StrategyLog,
		ruleDir,
	)

	// 启动数据源同步调度器
	if err := alertService.StartSyncScheduler(); err != nil {
		logger.Warnf("Failed to start datasource sync scheduler: %v", err)
	} else {
		logger.Infof("Datasource sync scheduler started")
	}
	onCallSvc := service.NewOnCallService(
		repos.OnCallSchedule,
		repos.OnCallShift,
		repos.OnCallAssignment,
	)

	// DMS 服务
	dmsPermissionService := dms.NewPermissionService(repos.DBPermission, repos.DBInstance)
	dmsInstanceService := dms.NewInstanceService(repos.DBInstance, cryptoService)
	dmsQueryService := dms.NewQueryService(repos.DBInstance, repos.QueryLog, dmsPermissionService, cryptoService)

	releaseService := release.NewService(repos.ReleaseRun)
	releaseService.SetSettingRepository(repos.Setting)
	releaseService.SetDB(database.DB)
	releaseService.SetAppRepo(repos.Application)

	registryService := registry.NewService(repos.Setting)

	// 初始化 Auth 服务
	authService := auth.NewAuthService(repos.User, repos.Setting, repos.RefreshToken, cfg.Security.JWTSecret, cfg.Security.AdminWhitelist)

	// 初始化 K8s 相关服务
	k8sService := service.NewK8sService(repos.K8sCluster)
	kubedogService := service.NewKubeDogService(cfg)
	k8sClusterService := service.NewK8sClusterService(repos.K8sCluster)
	k8sPermissionService := service.NewK8sPermissionService()
	deploymentService := service.NewDeploymentService(repos.Deployment, kubedogService, k8sService, repos.K8sCluster, cfg)
	releaseService.SetDeploymentService(deploymentService)
	assetSyncService := service.NewAssetSyncService(repos.AssetSync, repos.Host)

	approvalFactory := approval.NewFactory()
	loadApprovalProviders(database.DB, approvalFactory)

	return &Services{
		ApiKey:        service.NewApiKeyService(repos.ApiKey),
		Host:          hostService,
		Session:       sessionService,
		Auth:          authService,
		AssetSync:     assetSyncService,
		K8s:           k8sService,
		K8sCluster:    k8sClusterService,
		K8sPermission: k8sPermissionService,
		Deployment:    deploymentService,
		Bill:          billSvc,
		Monitor:       monitorService,
		Alert:         alertService,
		OnCall:        onCallSvc,
		DMSInstance:   dmsInstanceService,
		DMSQuery:      dmsQueryService,
		DMSPermission: dmsPermissionService,
		Release:       releaseService,
		Registry:      registryService,
		MongoClient:   mongoClient,
	}
}

// BackgroundServices 后台服务
type BackgroundServices struct {
	ProxyMonitor           *service.ProxyMonitor
	Expiration             *service.ExpirationService
	OnCallNotification     interface{} // OnCallNotificationService (使用interface避免循环依赖)
	CertificateAlert       *certificateService.CertificateAlertService
	InspectionReportSender aiassistant.InspectionReportSender // AI 巡检报告发往告警渠道（可选）
	BillSync               *billService.SyncScheduler
	DeploymentRecovery     *k8sService.DeploymentRecoveryWorker
}

// InitializeBackgroundServices 初始化后台服务
func InitializeBackgroundServices(repos *Repositories, cfg *config.Config, notificationMgr *notification.NotificationManager, alertService *service.AlertService, billSvc *billService.BillService, deploymentSvc *service.DeploymentService) *BackgroundServices {
	// Proxy monitor (仅在启用Proxy时启动)
	var proxyMonitor *service.ProxyMonitor
	if cfg.Proxy.Enabled {
		proxyMonitor = service.NewProxyMonitor(database.DB, service.MonitorConfig{
			CheckInterval:    1 * time.Minute,
			HeartbeatTimeout: 2 * time.Minute,
		})
		go proxyMonitor.Start()
	}

	// Expiration service
	expirationService := service.NewExpirationService(database.DB, notificationMgr)

	// On-call notification service
	onCallNotificationService := oncallNotification.NewOnCallNotificationService(
		database.DB,
		repos.OnCallShift,
		repos.OnCallSchedule,
	)

	// Certificate alert service
	// 创建 AlertNotifier（与 AlertService 使用相同的配置）
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = ""
	}
	alertNotifier := alertnotification.NewAlertNotifier(
		repos.StrategyLog,
		repos.AlertTemplate,
		repos.AlertChannel,
		repos.ChannelTemplate,
		repos.AlertGroup,
		repos.AlertRuleSource,
		frontendURL,
	)
	certificateAlertService := certificateService.NewCertificateAlertService(
		repos.DomainCertificate,
		repos.AlertTemplate,
		repos.AlertChannel,
		alertNotifier,
		database.DB,
	)
	inspectionReportSender := NewAIAssistantReportSender(alertNotifier)

	// 启动证书告警定时任务（每天检查一次）
	go func() {
		// 等待数据库连接就绪
		time.Sleep(5 * time.Second)

		// 立即执行一次检查
		if err := certificateAlertService.CheckAndSendAlerts(); err != nil {
			logger.Errorf("Failed to check certificate alerts: %v", err)
		}

		// 设置定时任务：每天凌晨2点执行
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			// 每天执行一次检查
			if err := certificateAlertService.CheckAndSendAlerts(); err != nil {
				logger.Errorf("Failed to check certificate alerts: %v", err)
			}
		}
	}()
	logger.Infof("Certificate alert service started, will check daily at 2:00 AM")

	// Recording converter service (convert .guac to MP4)
	recordingConverter := bastionService.GetRecordingConverter()
	recordingBasePath := os.Getenv("RECORDING_CONTAINER_PATH")
	if recordingBasePath == "" {
		recordingBasePath = "/replay" // 默认路径
	}
	// 启动后台转换服务，每5分钟扫描一次
	go recordingConverter.StartBackgroundConverter(recordingBasePath, 5*time.Minute)
	logger.Infof("Recording converter service started, scanning: %s, interval: 5 minutes", recordingBasePath)

	// Bill sync scheduler
	billSyncScheduler := billService.NewSyncScheduler(billSvc)
	billSvc.SetSyncScheduler(billSyncScheduler)
	go billSyncScheduler.Start()

	// Deployment recovery worker: 恢复重启后遗留的 pending 部署
	var deploymentRecoveryWorker *k8sService.DeploymentRecoveryWorker
	if deploymentSvc != nil && database.DB != nil {
		deploymentRecoveryWorker = k8sService.NewDeploymentRecoveryWorker(database.DB, deploymentSvc)
		go deploymentRecoveryWorker.Start(context.Background())
	}

	return &BackgroundServices{
		ProxyMonitor:           proxyMonitor,
		Expiration:             expirationService,
		OnCallNotification:     onCallNotificationService,
		CertificateAlert:       certificateAlertService,
		InspectionReportSender: inspectionReportSender,
		BillSync:               billSyncScheduler,
		DeploymentRecovery:     deploymentRecoveryWorker,
	}
}
