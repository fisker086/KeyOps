package database

import (
	"fmt"

	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(cfg *config.DatabaseConfig) error {
	// 设置默认值
	cfg.SetDefaults()

	// 初始化数据库连接（内部已经 Ping 验证）
	if err := InitDatabase(cfg); err != nil {
		return err
	}

	// 验证连接在迁移前仍然可用
	if DB == nil {
		return fmt.Errorf("database connection is nil after InitDatabase")
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance after InitDatabase: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database connection lost before migration: %w", err)
	}

	// 检查并自动迁移表（仅在表不存在时）
	if err := AutoMigrateAll(); err != nil {
		return fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	// 验证连接在迁移后仍然可用
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database connection lost after migration: %w", err)
	}

	logger.Infof("Database initialized successfully")
	return nil
}

// AutoMigrate 已废弃，请使用 AutoMigrateAll
// 保留此函数以保持向后兼容
func AutoMigrate() error {
	return AutoMigrateAll()
}

func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
