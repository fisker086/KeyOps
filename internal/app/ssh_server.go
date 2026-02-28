package app

import (
	"fmt"
	"os"

	"time"

	"github.com/fisker086/keyops/internal/audit"
	"github.com/fisker086/keyops/internal/bastion/blacklist"
	"github.com/fisker086/keyops/internal/bastion/storage"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/internal/sshserver/auth"
	"github.com/fisker086/keyops/internal/sshserver/recorder"
	"github.com/fisker086/keyops/internal/sshserver/server"
	"github.com/fisker086/keyops/internal/sshserver/terminal"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
)

// InitializeSSHServer 初始化 SSH Gateway Server
func InitializeSSHServer(
	cfg *config.Config,
	authService *service.AuthService,
	repos *Repositories,
	notificationMgr *notification.NotificationManager,
	unifiedAuditor *audit.DatabaseAuditor,
) (*server.Server, error) {
	if cfg.Server.SSHPort <= 0 {
		return nil, nil
	}

	logger.Infof("")
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Infof("Initializing SSH Gateway Server...")
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Use new AuthManager with handler architecture
	sshAuthenticator := auth.NewAuthManager(authService)
	// Use unified auditor through adapter
	sshAuditor := audit.NewSSHGatewayAuditorAdapter(unifiedAuditor)

	// Create temporary storage for recorder (will be unified later)
	tempStorage := storage.NewDatabaseStorage(database.DB)
	sshRecorder := recorder.NewAdapterRecorder(tempStorage)

	// 使用新权限架构的主机选择器（V2）
	hostSelector := terminal.NewHostSelectorV2(
		repos.Host,
		repos.HostGroup,
		repos.User,
		repos.SystemUser,
	)
	blacklistMgr := blacklist.NewManagerFromDB(database.DB)

	// 连接通知管理器到黑名单管理器
	blacklistMgr.SetNotificationManager(notificationMgr)

	// 使用新权限架构的终端处理器（V2）
	terminalHandler := terminal.NewProxyHandlerV2(
		hostSelector,
		sshAuditor,
		sshRecorder,
		blacklistMgr,
		repos.SystemUser,
	)

	// 获取实例ID（用于日志显示）
	instanceID := cfg.Server.ProxyID
	if instanceID == "" {
		// 如果没有配置proxy_id，使用hostname
		if hostname, err := os.Hostname(); err == nil {
			instanceID = hostname
		} else {
			instanceID = "default"
		}
	}

	logger.Infof("   Instance ID:    %s", instanceID)
	logger.Infof("   Host Key Mode:  Database Shared (multi-instance)")
	logger.Infof("   Storage:        ssh_host_keys table")
	logger.Infof("   Note: No local files will be generated")

	sshConfig := &server.Config{
		ListenAddress:    fmt.Sprintf(":%d", cfg.Server.SSHPort),
		MaxSessions:      getMaxSessions(cfg),
		SessionTimeout:   24 * time.Hour,
		IdleTimeout:      30 * time.Minute,
		ServerVersion:    "SSH-2.0-ZJump_1.0",
		HostKeyPath:      "", // 不使用本地文件（使用数据库共享模式）
		EnablePassword:   true,
		EnablePublicKey:  true,        // 启用公钥认证
		DB:               database.DB, // 数据库连接
		UseSharedHostKey: true,        // 启用数据库共享密钥（多实例部署推荐）
		BannerMessage:    "Welcome to ZJump SSH Gateway\r\n",
	}

	// 创建标准SSH服务器
	sshServer, err := server.NewServer(sshConfig, sshAuthenticator, terminalHandler)
	if err != nil {
		logger.Infof("Warning: Failed to create SSH server: %v", err)
		logger.Infof("   Continuing without SSH Gateway...")
		return nil, err
	}

	go func() {
		if err := sshServer.Start(); err != nil {
			logger.Infof("SSH Server failed to start: %v", err)
		}
	}()

	logger.Infof("")
	logger.Infof("SSH Gateway Server initialized")
	logger.Infof("   Listen Address: :%d", cfg.Server.SSHPort)
	logger.Infof("   Max Sessions:   %d", getMaxSessions(cfg))
	logger.Infof("   Authentication: Password, PublicKey")
	logger.Infof("")
	logger.Infof("Usage: ssh <username>@<server-ip> -p %d", cfg.Server.SSHPort)
	logger.Infof("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Infof("")

	return sshServer, nil
}

// getMaxSessions returns max SSH sessions from config or default
func getMaxSessions(cfg *config.Config) int {
	if cfg.SSH.MaxSessions > 0 {
		return cfg.SSH.MaxSessions
	}
	return 100
}
