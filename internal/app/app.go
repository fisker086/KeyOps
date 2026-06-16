package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fisker086/keyops/internal/audit"
	"github.com/fisker086/keyops/internal/infrastructure/mongodb"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/sshserver/server"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
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
	BastionMongo        *mongo.Client
	HTTPServer          *http.Server
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

	// 2. 初始化 MongoDB（堡垒机会话/录制，engine=mongodb 时）
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
	services := InitializeServices(repos, cfg)
	logger.Infof("Services initialized")

	// 4. Initialize audit service
	unifiedAuditor := audit.NewDatabaseAuditor(database.DB, repos.Session).(*audit.DatabaseAuditor)
	logger.Infof("Unified Audit Service initialized")
	logger.Infof("   Supports: SSH Gateway + WebShell")

	// 5. Initialize notification manager
	notificationMgr := notification.InitFromDatabase(database.DB)
	logger.Infof("Notification Manager initialized")

	// 6. Initialize background services
	backgroundServices := InitializeBackgroundServices(repos, cfg, notificationMgr, services.Alert, services.Bill, services.Deployment)
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 1. Stop HTTP server first (stops accepting new requests)
	if app.HTTPServer != nil {
		logger.Infof("[Shutdown] Stopping HTTP server...")
		if err := app.HTTPServer.Shutdown(shutdownCtx); err != nil {
			logger.Warnf("[Shutdown] HTTP server shutdown error: %v", err)
		} else {
			logger.Infof("[Shutdown] HTTP server stopped")
		}
	}

	// 2. Stop Expiration Service
	if app.BackgroundServices != nil && app.BackgroundServices.Expiration != nil {
		logger.Infof("[Shutdown] Stopping expiration service...")
		app.BackgroundServices.Expiration.Stop()
		logger.Infof("[Shutdown] Expiration service stopped")
	}

	// 3. Stop On-Call Notification Service
	if app.BackgroundServices != nil && app.BackgroundServices.OnCallNotification != nil {
		if onCallNotificationService, ok := app.BackgroundServices.OnCallNotification.(interface{ Stop() }); ok {
			logger.Infof("[Shutdown] Stopping on-call notification service...")
			onCallNotificationService.Stop()
			logger.Infof("[Shutdown] On-call notification service stopped")
		}
	}

	// 4. Stop bill sync scheduler
	if app.BackgroundServices != nil && app.BackgroundServices.BillSync != nil {
		app.BackgroundServices.BillSync.Stop()
		logger.Infof("[Shutdown] Bill sync scheduler stopped")
	}

	// 5. Stop proxy monitor
	if app.BackgroundServices != nil && app.BackgroundServices.ProxyMonitor != nil {
		logger.Infof("[Shutdown] Stopping proxy monitor...")
		app.BackgroundServices.ProxyMonitor.Stop()
		logger.Infof("[Shutdown] Proxy monitor stopped")
	}

	// 6. Close MongoDB connections (bastion)
	if app.BastionMongo != nil {
		logger.Infof("[Shutdown] Closing bastion MongoDB...")
		if err := app.BastionMongo.Disconnect(shutdownCtx); err != nil {
			logger.Warnf("[Shutdown] Bastion MongoDB close error: %v", err)
		} else {
			logger.Infof("[Shutdown] Bastion MongoDB connection closed")
		}
	}

	// 7. Stop SSH server
	if app.SSHServer != nil {
		logger.Infof("[Shutdown] Stopping SSH server...")
		if err := app.SSHServer.Stop(); err != nil {
			logger.Warnf("[Shutdown] SSH server stop error: %v", err)
		} else {
			logger.Infof("[Shutdown] SSH server stopped")
		}
	}

	// 8. Close database (MySQL/PostgreSQL)
	logger.Infof("[Shutdown] Closing database...")
	if err := CloseDatabase(); err != nil {
		logger.Warnf("[Shutdown] Database close error: %v", err)
	} else {
		logger.Infof("[Shutdown] Database connection closed")
	}

	// 9. Close Redis if enabled
	if app.Config != nil && app.Config.Redis.Enabled {
		logger.Infof("[Shutdown] Closing Redis...")
		pkgredis.Close()
		logger.Infof("[Shutdown] Redis closed")
	}

	logger.Infof("[Shutdown] Graceful shutdown completed")
}

// WaitForShutdown 阻塞等待关闭信号（SIGINT/SIGTERM），然后触发优雅关闭
func WaitForShutdown(app *App) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Infof("[Shutdown] Received signal: %v", sig)
	Shutdown(app)
}
