package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/api/router"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
)

// StartServer 启动 HTTP 服务器（不注册信号处理，由 Shutdown 统一管理优雅关闭）
func StartServer(app *App) {
	r := router.Setup(RouterDeps(app.Handlers, app.Services, app.Repos, app.Config.Server.Mode))

	// Start expiration service (延迟启动，确保数据库连接完全就绪)
	ctx := context.Background()
	go func() {
		time.Sleep(3 * time.Second)
		if err := app.BackgroundServices.Expiration.Start(ctx); err != nil {
			logger.Warnf("Failed to start expiration service: %v", err)
		} else {
			logger.Infof("Expiration Service started")
			logger.Infof("   Checking for expired users and permissions")
		}
	}()

	// Start on-call notification service (延迟启动，确保数据库连接完全就绪)
	go func() {
		time.Sleep(4 * time.Second)
		if onCallNotificationService, ok := app.BackgroundServices.OnCallNotification.(interface {
			Start(context.Context) error
		}); ok {
			if err := onCallNotificationService.Start(ctx); err != nil {
				logger.Warnf("Failed to start on-call notification service: %v", err)
			} else {
				logger.Infof("On-Call Notification Service started")
				logger.Infof("   Checking for upcoming shifts (interval: 1 minute)")
			}
		}
	}()
	logger.Infof("")

	// Start HTTP server
	addr := fmt.Sprintf(":%d", app.Config.Server.APIPort)
	app.HTTPServer = &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    300 * time.Second,
		WriteTimeout:   300 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Print startup banner
	printStartupBanner(app.Config)

	logger.Infof("HTTP server listening on %s", addr)

	// Block until Shutdown calls httpServer.Shutdown()
	if err := app.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// printStartupBanner 打印启动横幅
func printStartupBanner(cfg *config.Config) {
	logger.Infof("")
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Infof("KeyOps Unified Server v2.0 - Intelligent Routing Architecture")
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Infof("")
	logger.Infof("Features:")
	logger.Infof("   • Authentication & Authorization")
	logger.Infof("   • Intelligent Routing - Auto path selection")
	logger.Infof("   • Direct Connection - Default mode, low latency")
	logger.Infof("   • Proxy Forwarding - Use Proxy Agent in isolated networks")
	logger.Infof("   • Full Audit Trail - Complete operation logs")
	if cfg.Server.SSHPort > 0 {
		logger.Infof("   • SSH Gateway - CLI login with full audit")
	}
	logger.Infof("")
	logger.Infof("🔀 Connection Modes:")
	logger.Infof("   • Web Mode   - Browser access (:%d)", cfg.Server.APIPort)
	if cfg.Server.SSHPort > 0 {
		logger.Infof("   • SSH Mode   - SSH client (:%d)", cfg.Server.SSHPort)
	}
	logger.Infof("   • Direct     - API Server connects to target directly")
	logger.Infof("   • Proxy      - Via Proxy Agent (8022) for isolated networks")
	logger.Infof("")
	logger.Infof("Tips:")
	logger.Infof("   Start only this service for both Web and SSH access")
	logger.Infof("   Proxy Agent is optional, needed only for isolated networks")
	logger.Infof("")
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Infof("")
}
