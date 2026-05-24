package app

import (
	"context"
	"time"

	"github.com/fisker086/keyops/internal/audit"
	"github.com/fisker086/keyops/internal/infrastructure/mongodb"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/sshserver/server"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"go.mongodb.org/mongo-driver/mongo"
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
	MongoClient         *mongodb.Client
	BastionMongo        *mongo.Client
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

	// 2. 初始化 MongoDB：账单库（独立 Client）与堡垒机库（独立 URI / Database）
	var mongoClient *mongodb.Client
	if cfg.BillStorage.GetURI() != "" {
		mongoClient, err = mongodb.NewClientWithDatabase(cfg.BillStorage.GetURI(), cfg.BillStorage.Database)
		if err != nil {
			logger.Warnf("MongoDB (bill) connect failed: %v", err)
		} else {
			if err := mongoClient.InitIndexes(context.Background()); err != nil {
				logger.Warnf("MongoDB init indexes failed: %v", err)
			}
		}
	}

	var bastionMongo *mongo.Client
	if cfg.BastionStorage.GetEngine() == "mongodb" {
		cfg.BastionStorage.SetDefaults()
		uri := cfg.BastionStorage.GetMongoURI()
		if uri != "" {
			ctxm, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var connErr error
			bastionMongo, connErr = mongodb.Connect(ctxm, uri)
			cancel()
			if connErr != nil {
				logger.Warnf("Bastion MongoDB connect failed: %v", connErr)
				bastionMongo = nil
			} else {
				logger.Infof("Bastion MongoDB connected (db=%s)", cfg.BastionStorage.MongoDB.Database)
			}
		} else {
			logger.Warnf("bastion_storage.engine=mongodb but Mongo URI is empty; falling back to SQL for bastion data")
		}
	}

	// 3. Initialize repositories（根据配置选择存储引擎）
	repos := InitializeRepositories(cfg, bastionMongo)
	logger.Infof("Repositories initialized")

	// 4. Initialize services
	services := InitializeServices(repos, cfg, mongoClient)
	logger.Infof("Services initialized")

	// 4. Initialize audit service
	unifiedAuditor := audit.NewDatabaseAuditor(database.DB, repos.Session).(*audit.DatabaseAuditor)
	logger.Infof("Unified Audit Service initialized")
	logger.Infof("   Supports: SSH Gateway + WebShell")

	// 5. Initialize notification manager
	notificationMgr := notification.InitFromDatabase(database.DB)
	logger.Infof("Notification Manager initialized")

	// 6. Initialize background services
	backgroundServices := InitializeBackgroundServices(repos, cfg, notificationMgr, services.Alert, services.Bill)
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
		MongoClient:         services.MongoClient,
		BastionMongo:        bastionMongo,
	}, nil
}

// CloseDatabase 关闭数据库连接（供优雅关闭调用）
func CloseDatabase() error {
	return database.Close()
}

// Shutdown 优雅关闭应用程序
func Shutdown(app *App) {
	logger.Infof("[Shutdown] Received shutdown signal, starting graceful shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 停止账单同步调度器
	// if app.BackgroundServices.BillSync != nil {
	// 	app.BackgroundServices.BillSync.Stop()
	// 	logger.Infof("[Shutdown] Bill sync scheduler stopped")
	// }

	// 2. Close MongoDB connections
	if app.BastionMongo != nil {
		if err := app.BastionMongo.Disconnect(ctx); err != nil {
			logger.Warnf("[Shutdown] Bastion MongoDB close error: %v", err)
		} else {
			logger.Infof("[Shutdown] Bastion MongoDB connection closed")
		}
	}
	if app.MongoClient != nil {
		if err := app.MongoClient.Close(ctx); err != nil {
			logger.Warnf("[Shutdown] MongoDB (bill) close error: %v", err)
		} else {
			logger.Infof("[Shutdown] MongoDB (bill) connection closed")
		}
	}

	// 3. Stop SSH server
	if app.SSHServer != nil {
		if err := app.SSHServer.Stop(); err != nil {
			logger.Warnf("[Shutdown] SSH server stop error: %v", err)
		} else {
			logger.Infof("[Shutdown] SSH server stopped")
		}
	}

	// 4. Close database (MySQL/PostgreSQL)
	if err := CloseDatabase(); err != nil {
		logger.Warnf("[Shutdown] Database close error: %v", err)
	} else {
		logger.Infof("[Shutdown] Database connection closed")
	}

	logger.Infof("[Shutdown] Graceful shutdown completed")
}

// WaitForShutdown 阻塞等待关闭信号，然后触发优雅关闭
func WaitForShutdown(app *App) {
	// TODO: 实现信号处理
	<-make(chan struct{})
	Shutdown(app)
}
