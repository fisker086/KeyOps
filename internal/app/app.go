package app

import (
	"github.com/fisker086/keyops/internal/audit"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/sshserver/server"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
)

// App 应用程序上下文
type App struct {
	Config              *config.Config
	Repos               *Repositories
	Services            *Services
	BackgroundServices  *BackgroundServices
	Handlers            *Handlers
	SSHServer           *server.Server
	UnifiedAuditor      *audit.DatabaseAuditor
	NotificationManager *notification.NotificationManager
}

// Initialize 初始化应用程序
func Initialize(cfgPath string) (*App, error) {
	// 1. Bootstrap (logger, database, redis, casbin)
	cfg, err := Bootstrap(cfgPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			database.Close()
		}
	}()

	// 2. Initialize repositories
	repos := InitializeRepositories()
	logger.Infof("Repositories initialized")

	// 3. Initialize services
	services := InitializeServices(repos, cfg)
	logger.Infof("Services initialized")

	// 4. Initialize audit service
	unifiedAuditor := audit.NewDatabaseAuditor(database.DB).(*audit.DatabaseAuditor)
	logger.Infof("Unified Audit Service initialized")
	logger.Infof("   Supports: SSH Gateway + WebShell")

	// 5. Initialize notification manager
	notificationMgr := notification.InitFromDatabase(database.DB)
	logger.Infof("Notification Manager initialized")

	// 6. Initialize background services
	backgroundServices := InitializeBackgroundServices(repos, cfg, notificationMgr, services.Alert)
	logger.Infof("Background services initialized")
	logger.Infof("   Asset sync scheduler started")
	logger.Infof("   Host status monitor started (interval: 5 minutes)")
	logger.Infof("   Certificate alert service started (daily check)")

	// 7. Initialize handlers
	handlers := InitializeHandlers(repos, services, backgroundServices, notificationMgr, unifiedAuditor)
	logger.Infof("Handlers initialized")

	// 8. Initialize SSH server (optional)
	sshServer, err := InitializeSSHServer(cfg, services.Auth, repos, notificationMgr, unifiedAuditor)
	if err != nil && cfg.Server.SSHPort > 0 {
		logger.Warnf("SSH Server initialization failed: %v", err)
	}

	return &App{
		Config:              cfg,
		Repos:               repos,
		Services:            services,
		BackgroundServices:  backgroundServices,
		Handlers:            handlers,
		SSHServer:           sshServer,
		UnifiedAuditor:      unifiedAuditor,
		NotificationManager: notificationMgr,
	}, nil
}
