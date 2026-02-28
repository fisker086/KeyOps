package app

import (
	"fmt"
	"os"

	casbinpkg "github.com/fisker086/keyops/pkg/casbin"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
)

// BootstrapConfig 初始化配置
type BootstrapConfig struct {
	ConfigPath string
}

// Bootstrap 初始化基础设施（logger, database, redis, casbin）
func Bootstrap(cfgPath string) (*config.Config, error) {
	// 支持通过环境变量指定配置文件路径
	if cfgPath == "" {
		cfgPath = os.Getenv("KEYOPS_CONFIG")
		if cfgPath == "" {
			cfgPath = "config/config.yaml"
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	// Initialize logger
	if err := logger.Init(&cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	logger.Infof("Database initialized successfully")

	// Initialize Redis (optional, for distributed features)
	if err := pkgredis.Init(&cfg.Redis); err != nil {
		logger.Warnf("⚠️  Redis initialization failed: %v", err)
		logger.Info("   → System will use database mode (single-server deployment)")
		logger.Info("   → Casbin permissions will sync via database (manual ReloadPolicy required)")
	} else if cfg.Redis.Enabled {
		logger.Infof("✅ Redis initialized successfully - distributed features enabled")
	} else {
		logger.Info("ℹ️  Redis is disabled in config - using database mode")
	}

	// Initialize Casbin permission manager (after Redis, so Watcher can be configured)
	if err := casbinpkg.Init(); err != nil {
		logger.Fatalf("Failed to initialize Casbin: %v", err)
	}
	logger.Infof("Casbin permission manager initialized successfully")

	return cfg, nil
}

