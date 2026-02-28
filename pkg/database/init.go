package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/fisker086/keyops/pkg/sshkey"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// InitDatabase 初始化数据库（支持 MySQL 和 PostgreSQL）
func InitDatabase(cfg *config.DatabaseConfig) error {
	var err error
	var dialector gorm.Dialector

	// 根据配置选择数据库驱动
	switch cfg.Driver {
	case "postgres", "postgresql":
		// PostgreSQL: 先创建数据库（如果不存在）
		if err := createPostgresDatabase(cfg); err != nil {
			return fmt.Errorf("failed to create PostgreSQL database: %w", err)
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
		dialector = postgres.Open(dsn)
	case "mysql", "":
		// MySQL: 先创建数据库（如果不存在）
		if err := createMySQLDatabase(cfg); err != nil {
			return fmt.Errorf("failed to create MySQL database: %w", err)
		}
		// 默认使用 MySQL
		dsn := cfg.DSN()
		dialector = mysql.Open(dsn)
	default:
		return fmt.Errorf("unsupported database driver: %s (supported: mysql, postgres)", cfg.Driver)
	}

	logger.Infof("Connecting to %s database...", cfg.Driver)

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormLogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormLogger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})

	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 100 // 默认值
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)

	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10 // 默认值
	}
	if maxIdleConns > maxOpenConns {
		maxIdleConns = maxOpenConns
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)

	connMaxLifetime := cfg.ConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = 3600 // 默认 1 小时
	}
	sqlDB.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)

	logger.Infof("Database connection pool configured: MaxOpenConns=%d, MaxIdleConns=%d, ConnMaxLifetime=%ds",
		maxOpenConns, maxIdleConns, connMaxLifetime)

	// 立即 Ping 数据库以确保连接可用
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Infof("Database connection verified successfully")

	// 再次验证连接状态（确保连接没有被关闭）
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database connection lost after initial ping: %w", err)
	}

	logger.Infof("Database connection double-checked successfully")
	return nil
}

// createMySQLDatabase 创建 MySQL 数据库（如果不存在）
// 使用 database/sql 而不是 GORM，避免影响主连接
func createMySQLDatabase(cfg *config.DatabaseConfig) error {
	// 连接到 MySQL 服务器（不指定数据库）
	dsnWithoutDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port)

	// 使用 database/sql 直接连接，避免使用 GORM（可能影响主连接）
	db, err := sql.Open("mysql", dsnWithoutDB)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %w", err)
	}
	defer db.Close() // 确保关闭临时连接

	// 设置连接超时
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Second)

	// 测试连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	// 创建数据库（如果不存在）
	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.DBName)
	if _, err := db.Exec(createDBSQL); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	logger.Infof("Database '%s' created or already exists", cfg.DBName)
	return nil
}

// createPostgresDatabase 创建 PostgreSQL 数据库（如果不存在）
// 使用 database/sql 而不是 GORM，避免影响主连接
func createPostgresDatabase(cfg *config.DatabaseConfig) error {
	// PostgreSQL 需要连接到默认的 postgres 数据库来创建新数据库
	dsnPostgres := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password)

	// 使用 database/sql 直接连接，避免使用 GORM（可能影响主连接）
	db, err := sql.Open("postgres", dsnPostgres)
	if err != nil {
		// 如果连接 postgres 数据库失败，尝试连接 template1
		dsnTemplate1 := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=template1 sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password)
		db, err = sql.Open("postgres", dsnTemplate1)
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL server: %w", err)
		}
	}
	defer db.Close() // 确保关闭临时连接

	// 设置连接超时
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Second)

	// 测试连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL server: %w", err)
	}

	// 检查数据库是否已存在
	var count int64
	checkSQL := "SELECT COUNT(*) FROM pg_database WHERE datname = $1"
	if err := db.QueryRow(checkSQL, cfg.DBName).Scan(&count); err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	// 如果数据库不存在，创建它
	if count == 0 {
		createDBSQL := fmt.Sprintf("CREATE DATABASE %s", cfg.DBName)
		if _, err := db.Exec(createDBSQL); err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		logger.Infof("Database '%s' created successfully", cfg.DBName)
	} else {
		logger.Infof("Database '%s' already exists", cfg.DBName)
	}

	return nil
}

// CheckTableExists 检查表是否存在
func CheckTableExists(tableName string) (bool, error) {
	if DB == nil {
		return false, fmt.Errorf("database connection is not initialized")
	}

	var count int64
	var err error

	// 根据数据库类型使用不同的查询
	if DB.Dialector.Name() == "postgres" {
		err = DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?", tableName).Scan(&count).Error
	} else {
		// MySQL
		err = DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&count).Error
	}

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// AutoMigrateAll 自动迁移所有表（仅在表不存在时创建）
func AutoMigrateAll() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	logger.Info("Checking database tables...")

	// 定义所有需要创建的表（使用 GORM 的 TableName 方法获取实际表名）
	tables := []interface{}{
		&model.Host{},
		&model.LoginRecord{},
		&model.SSHSession{},
		&model.SessionRecording{},
		&model.CommandRecord{},
		&model.Proxy{},
		&model.CommandHistory{},
		&model.PodCommandRecord{},
		&model.SessionHistory{},
		&model.Setting{},
		&model.FileTransfer{},
		&model.OperationLog{},
		&model.Menu{},
		&model.MenuPermission{},
		&model.API{},
		&model.CasbinModel{},
		&model.CasbinRule{},
		&model.Workflow{},
		&model.WorkflowStep{},
		&model.WorkflowComment{},
		&model.User{},
		&model.UserGroupPermission{},
		&model.UserHostPermission{},
		&model.PlatformLoginRecord{},
		&model.Role{},
		&model.RoleMember{},
		&model.SystemUser{},
		&model.HostGroup{},
		&model.HostGroupMember{},
		&model.PermissionRule{},
		&model.PermissionRuleSystemUser{},
		&model.PermissionRuleHostGroup{},
		&model.AssetSyncConfig{},
		&model.AssetSyncLog{},
		&model.K8sCluster{},
		&model.ClusterPermission{},
		&model.Deployment{},
		&model.BillSummary{},
		&model.BillSummaryDetail{},
		&model.BillRecord{},
		&model.BillPrice{},
		&model.Monitor{},
		&model.Organization{},
		&model.Application{},
		&model.JenkinsServer{},
		&model.FormCategory{},
		&model.FormTemplate{},
		&model.Ticket{},
		&model.TicketApprovalConfig{},
		&model.AlertRuleSource{},
		&model.AlertRuleGroup{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.AlertLog{},
		&model.AlertStrategy{},
		&model.AlertLevel{},
		&model.AlertAggregation{},
		&model.AlertSilence{},
		&model.AlertRestrain{},
		&model.AlertTemplate{},
		&model.ChannelTemplate{},
		&model.AlertChannel{},
		&model.AlertGroup{},
		&model.StrategyLog{},
		&model.OnCallSchedule{},
		&model.OnCallShift{},
		&model.OnCallAssignment{},
		&model.BlacklistRule{},
		&sshkey.SSHHostKey{},
		&model.Approval{},
		&model.ApprovalComment{},
		&model.ApprovalConfig{},
		&model.ExpirationNotificationConfig{},
		&model.UserExpirationLog{},
		&model.PermissionExpirationLog{},
		&model.TwoFactorConfig{},
		&model.ReleaseRun{},
		&model.ApplicationDeployBinding{},
		&model.ReleasePipelineDefinition{},
		&model.BuildMasterList{},
		&model.BuildMasterOperationLog{},
	}

	// 检查每个表是否存在，只迁移不存在的表
	var tablesToMigrate []interface{}
	for _, table := range tables {
		// 使用 GORM 的 Statement 获取表名
		stmt := &gorm.Statement{DB: DB}
		if err := stmt.Parse(table); err != nil {
			logger.Warnf("Failed to parse table model: %v", err)
			continue
		}
		tableName := stmt.Schema.Table
		exists, err := CheckTableExists(tableName)
		if err != nil {
			logger.Warnf("Failed to check table %s: %v", tableName, err)
			// 如果检查失败，仍然尝试迁移（可能是权限问题，但迁移可能会成功）
			tablesToMigrate = append(tablesToMigrate, table)
			continue
		}
		if !exists {
			logger.Infof("Table %s does not exist, will be created", tableName)
			tablesToMigrate = append(tablesToMigrate, table)
		} else {
			logger.Debugf("Table %s already exists, skipping", tableName)
			// 已存在时仍参与迁移，以自动添加新字段
			if tableName == "release_runs" || tableName == "application_deploy_bindings" ||
				tableName == "release_pipeline_definitions" || tableName == "build_master_lists" ||
				tableName == "build_master_operation_logs" {
				tablesToMigrate = append(tablesToMigrate, table)
			}
		}
	}

	// 如果没有需要迁移的表，直接返回
	if len(tablesToMigrate) == 0 {
		logger.Info("All database tables already exist, no migration needed")
		return nil
	}

	// 执行自动迁移，只创建不存在的表
	logger.Infof("Starting auto-migration for %d table(s)...", len(tablesToMigrate))
	err := DB.AutoMigrate(tablesToMigrate...)

	if err != nil {
		return fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	logger.Infof("Successfully migrated %d table(s)", len(tablesToMigrate))

	// 创建默认数据（用户、角色等）
	if err := createDefaultData(); err != nil {
		logger.Warnf("Failed to create default data: %v", err)
		// 不返回错误，因为表已经创建成功，默认数据可以后续手动创建
	}

	return nil
}

// createDefaultData 创建默认数据（用户、角色等）
func createDefaultData() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	logger.Info("Creating default data...")

	// 1. 创建默认角色
	if err := createDefaultRoles(); err != nil {
		logger.Warnf("Failed to create default roles: %v", err)
	}

	// 2. 创建默认用户
	if err := createDefaultUser(); err != nil {
		logger.Warnf("Failed to create default user: %v", err)
	}

	// 3. 创建默认主机组
	if err := createDefaultHostGroup(); err != nil {
		logger.Warnf("Failed to create default host group: %v", err)
	}

	// 4. 创建默认黑名单规则
	if err := createDefaultBlacklistRules(); err != nil {
		logger.Warnf("Failed to create default blacklist rules: %v", err)
	}

	// 5. 创建默认系统设置
	if err := createDefaultSettings(); err != nil {
		logger.Warnf("Failed to create default settings: %v", err)
	}

	// 6. 菜单数据通过 init.sql 手动导入，不在此处初始化
	// 如果需要自动创建菜单，可以取消下面的注释
	// if err := createDefaultMenus(); err != nil {
	// 	logger.Warnf("Failed to create default menus: %v", err)
	// }
	// if err := createDefaultMenuPermissions(); err != nil {
	// 	logger.Warnf("Failed to create default menu permissions: %v", err)
	// }

	logger.Info("Default data creation completed")
	return nil
}

// createDefaultRoles 创建默认角色
func createDefaultRoles() error {
	roles := []model.Role{
		{
			ID:          "role:admin",
			Name:        "管理员",
			Description: "系统管理员角色，拥有所有权限",
			Color:       "#f5222d",
			Priority:    999,
			Status:      "active",
		},
		{
			ID:          "role:user",
			Name:        "普通用户",
			Description: "普通用户角色，拥有基础权限",
			Color:       "#52c41a",
			Priority:    0,
			Status:      "active",
		},
	}

	for _, role := range roles {
		var existing model.Role
		result := DB.Where("id = ?", role.ID).First(&existing)
		if result.Error != nil {
			// 角色不存在，创建它
			if err := DB.Create(&role).Error; err != nil {
				return fmt.Errorf("failed to create role %s: %w", role.ID, err)
			}
			logger.Infof("Created default role: %s (%s)", role.Name, role.ID)
		}
	}

	return nil
}

// createDefaultUser 创建默认管理员用户
func createDefaultUser() error {
	// 检查用户是否已存在
	var existingUser model.User
	result := DB.Where("username = ?", "admin").First(&existingUser)
	if result.Error == nil {
		// 用户已存在，检查是否需要分配角色
		var roleMember model.RoleMember
		roleMemberResult := DB.Where("role_id = ? AND user_id = ?", "role:admin", existingUser.ID).First(&roleMember)
		if roleMemberResult.Error != nil {
			// 用户存在但没有分配管理员角色，分配角色
			roleMember := model.RoleMember{
				RoleID:  "role:admin",
				UserID:  existingUser.ID,
				AddedBy: existingUser.ID,
			}
			if err := DB.Create(&roleMember).Error; err != nil {
				logger.Warnf("Failed to assign admin role to existing user: %v", err)
			} else {
				logger.Infof("Assigned admin role to existing user: admin")
			}
		}
		return nil
	}

	// 用户不存在，创建默认管理员用户
	// Password hash for 'admin123': $2a$10$j/lQBaOvW9dMo/O13g65qeCwYnxuaZerNcB/eA3IZZXxRp4MbePhG
	defaultUser := model.User{
		ID:       "00000000-0000-0000-0000-000000000001",
		Username: "admin",
		Password: "$2a$10$j/lQBaOvW9dMo/O13g65qeCwYnxuaZerNcB/eA3IZZXxRp4MbePhG", // bcrypt hash of 'admin123'
		FullName: "System Admin",
		Email:    "admin@zjump.local",
		Role:     "admin",
		Status:   "active",
	}

	if err := DB.Create(&defaultUser).Error; err != nil {
		return fmt.Errorf("failed to create default user: %w", err)
	}

	logger.Infof("Created default admin user: admin/admin123")

	// 为 admin 用户分配管理员角色
	roleMember := model.RoleMember{
		RoleID:  "role:admin",
		UserID:  defaultUser.ID,
		AddedBy: defaultUser.ID,
	}
	if err := DB.Create(&roleMember).Error; err != nil {
		logger.Warnf("Failed to assign admin role to default user: %v", err)
	} else {
		logger.Infof("Assigned admin role to default user: admin")
	}

	return nil
}

// createDefaultHostGroup 创建默认主机组
func createDefaultHostGroup() error {
	hostGroup := model.HostGroup{
		ID:          "default-group",
		Name:        "Default",
		Description: "Default host group",
		Color:       "#1890ff",
		Icon:        "",
		SortOrder:   0,
	}

	var existing model.HostGroup
	result := DB.Where("id = ?", hostGroup.ID).First(&existing)
	if result.Error != nil {
		// 主机组不存在，创建它
		if err := DB.Create(&hostGroup).Error; err != nil {
			return fmt.Errorf("failed to create default host group: %w", err)
		}
		logger.Infof("Created default host group: %s", hostGroup.Name)
	}

	return nil
}

// createDefaultBlacklistRules 创建默认黑名单规则
func createDefaultBlacklistRules() error {
	// 检查是否已有黑名单规则
	var count int64
	DB.Model(&model.BlacklistRule{}).Count(&count)
	if count > 0 {
		return nil // 已有规则，跳过
	}

	// 创建默认黑名单规则（与 init.sql 保持一致）
	rules := []model.BlacklistRule{
		{
			ID:          uuid.New().String(),
			Command:     "rm",
			Pattern:     "^rm\\s+.*(-rf?|--recursive).*",
			Description: "Block dangerous file deletion",
			Scope:       "global",
			Enabled:     true,
		},
		{
			ID:          uuid.New().String(),
			Command:     "dd",
			Pattern:     "^dd\\s+.*of=/dev/",
			Description: "Block disk overwrite",
			Scope:       "global",
			Enabled:     true,
		},
		{
			ID:          uuid.New().String(),
			Command:     "mkfs",
			Pattern:     "^mkfs\\.",
			Description: "Block filesystem formatting",
			Scope:       "global",
			Enabled:     true,
		},
		{
			ID:          uuid.New().String(),
			Command:     "reboot",
			Pattern:     "^(reboot|shutdown|halt|poweroff)",
			Description: "Block system restart",
			Scope:       "global",
			Enabled:     true,
		},
		{
			ID:          uuid.New().String(),
			Command:     "fdisk",
			Pattern:     "^fdisk\\s+/dev/",
			Description: "Block disk partitioning",
			Scope:       "global",
			Enabled:     true,
		},
	}

	for _, rule := range rules {
		if err := DB.Create(&rule).Error; err != nil {
			logger.Warnf("Failed to create blacklist rule %s: %v", rule.ID, err)
		}
	}

	logger.Infof("Created %d default blacklist rules", len(rules))
	return nil
}

// createDefaultSettings 创建默认系统设置
func createDefaultSettings() error {
	settings := []model.Setting{
		{Key: "host_monitor_enabled", Value: "false", Category: "host_monitor", Type: "boolean"},
		{Key: "host_monitor_interval", Value: "5", Category: "host_monitor", Type: "number"},
		{Key: "host_monitor_method", Value: "tcp", Category: "host_monitor", Type: "string"},
		{Key: "host_monitor_timeout", Value: "3", Category: "host_monitor", Type: "number"},
		{Key: "host_monitor_concurrent", Value: "20", Category: "host_monitor", Type: "number"},
		{Key: "expiration_check_enabled", Value: "true", Category: "expiration", Type: "boolean"},
		{Key: "expiration_check_interval", Value: "3600", Category: "expiration", Type: "number"},
		{Key: "user_expiration_auto_disable", Value: "true", Category: "expiration", Type: "boolean"},
		{Key: "permission_expiration_auto_disable", Value: "true", Category: "expiration", Type: "boolean"},
		{Key: "expiration_warning_days_user", Value: "7", Category: "expiration", Type: "number"},
		{Key: "expiration_warning_days_permission", Value: "3", Category: "expiration", Type: "number"},
	}

	for _, setting := range settings {
		var existing model.Setting
		// 根据数据库类型使用正确的引号
		var keyColumn string
		if DB.Dialector.Name() == "postgres" {
			keyColumn = "\"key\""
		} else {
			keyColumn = "`key`"
		}
		result := DB.Where(keyColumn+" = ?", setting.Key).First(&existing)
		if result.Error != nil {
			// 设置不存在，创建它
			if err := DB.Create(&setting).Error; err != nil {
				logger.Warnf("Failed to create setting %s: %v", setting.Key, err)
			}
		}
	}

	logger.Infof("Created default settings")
	return nil
}

// createDefaultMenus 创建默认菜单
// 注意：菜单数据通过 init.sql 手动导入，此函数已禁用
// 如需启用，请取消 createDefaultData() 中的注释
func createDefaultMenus() error {
	// 菜单数据通过 init.sql 手动导入，不在此处初始化
	logger.Info("Menu initialization is disabled. Please import menu data from sql/init.sql manually.")
	logger.Info("To enable automatic menu creation, uncomment the menu initialization code in createDefaultData()")
	return nil
	// 以下代码已禁用，保留备用
	// 如需启用菜单自动创建，请取消下面的注释并取消 createDefaultData() 中的注释
	/*
		// 检查是否已有菜单
		var count int64
		DB.Model(&model.Menu{}).Count(&count)
		if count > 0 {
			logger.Debugf("Menus already exist (%d menus), skipping menu creation", count)
			return nil // 已有菜单，跳过
		}

		logger.Info("Creating default menus...")

		// 完整的菜单数据（从 init.sql 转换）
		menus := []model.Menu{
			// 首页分组
			{
				ID:        "menu-home",
				ParentID:  "",
				Path:      "",
				Name:      "home",
				Component: "",
				Hidden:    false,
				Sort:      1,
				Meta: model.MenuMeta{
					Title:       "首页",
					Icon:        "Home",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 组织管理分组
			{
				ID:        "menu-user-permission",
				ParentID:  "",
				Path:      "",
				Name:      "userPermission",
				Component: "",
				Hidden:    false,
				Sort:      2,
				Meta: model.MenuMeta{
					Title:       "组织管理",
					Icon:        "AccountTree",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 资产管理分组
			{
				ID:        "menu-assets",
				ParentID:  "",
				Path:      "",
				Name:      "assets",
				Component: "",
				Hidden:    false,
				Sort:      3,
				Meta: model.MenuMeta{
					Title:       "资产管理",
					Icon:        "Storage",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 堡垒机分组
			{
				ID:        "menu-bastion",
				ParentID:  "",
				Path:      "",
				Name:      "bastion",
				Component: "",
				Hidden:    false,
				Sort:      4,
				Meta: model.MenuMeta{
					Title:       "堡垒机",
					Icon:        "Terminal",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 工单管理分组
			{
				ID:        "menu-workorder",
				ParentID:  "",
				Path:      "",
				Name:      "workorder",
				Component: "",
				Hidden:    false,
				Sort:      5,
				Meta: model.MenuMeta{
					Title:       "工单管理",
					Icon:        "Assignment",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 集群管理分组
			{
				ID:        "menu-k8s",
				ParentID:  "",
				Path:      "",
				Name:      "k8s",
				Component: "",
				Hidden:    false,
				Sort:      6,
				Meta: model.MenuMeta{
					Title:       "集群管理",
					Icon:        "Cloud",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 配置管理分组
			{
				ID:        "menu-config",
				ParentID:  "",
				Path:      "",
				Name:      "config",
				Component: "",
				Hidden:    false,
				Sort:      7,
				Meta: model.MenuMeta{
					Title:       "配置管理",
					Icon:        "Settings",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 监控告警分组
			{
				ID:        "menu-monitor",
				ParentID:  "",
				Path:      "",
				Name:      "monitor",
				Component: "",
				Hidden:    false,
				Sort:      8,
				Meta: model.MenuMeta{
					Title:       "监控告警",
					Icon:        "Monitor",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 账单管理分组
			{
				ID:        "menu-bill",
				ParentID:  "",
				Path:      "",
				Name:      "bill",
				Component: "",
				Hidden:    false,
				Sort:      9,
				Meta: model.MenuMeta{
					Title:       "账单管理",
					Icon:        "Receipt",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
			// 系统设置
			{
				ID:        "menu-system",
				ParentID:  "",
				Path:      "/settings",
				Name:      "system",
				Component: "pages/system/Settings",
				Hidden:    false,
				Sort:      10,
				Meta: model.MenuMeta{
					Title:       "系统设置",
					Icon:        "Settings",
					KeepAlive:   false,
					ActiveName:  "",
					CloseTab:    false,
					DefaultMenu: false,
				},
			},
		}

		// 批量创建菜单
		for _, menu := range menus {
			var existing model.Menu
			result := DB.Where("id = ?", menu.ID).First(&existing)
			if result.Error != nil {
				// 菜单不存在，创建它
				if err := DB.Create(&menu).Error; err != nil {
					logger.Warnf("Failed to create menu %s: %v", menu.ID, err)
				} else {
					logger.Debugf("Created menu: %s (%s)", menu.Meta.Title, menu.ID)
				}
			}
		}

		return nil
	*/
}

// createDefaultMenuPermissions 创建默认菜单权限（为管理员角色分配所有菜单权限）
// 注意：菜单权限数据通过 init.sql 手动导入，此函数已禁用
// 如需启用，请取消 createDefaultData() 中的注释
func createDefaultMenuPermissions() error {
	// 菜单权限数据通过 init.sql 手动导入，不在此处初始化
	logger.Info("Menu permission initialization is disabled. Please import menu permission data from sql/init.sql manually.")
	return nil
	// 以下代码已禁用，保留备用
	/*
		// 检查是否已有菜单权限
		var count int64
		DB.Model(&model.MenuPermission{}).Count(&count)
		if count > 0 {
			logger.Debugf("Menu permissions already exist (%d permissions), skipping", count)
			return nil // 已有权限，跳过
		}

		logger.Info("Creating default menu permissions...")

		// 获取所有菜单
		var menus []model.Menu
		if err := DB.Find(&menus).Error; err != nil {
			return fmt.Errorf("failed to query menus: %w", err)
		}

		if len(menus) == 0 {
			logger.Warnf("No menus found, skipping menu permission creation")
			return nil
		}

		// 为管理员角色分配所有菜单权限
		adminRoleID := "role:admin"
		adminUserID := "00000000-0000-0000-0000-000000000001" // admin 用户 ID

		var createdCount int
		for _, menu := range menus {
			// 检查权限是否已存在
			var existing model.MenuPermission
			result := DB.Where("role_id = ? AND menu_id = ?", adminRoleID, menu.ID).First(&existing)
			if result.Error != nil {
				// 权限不存在，创建它
				permission := model.MenuPermission{
					RoleID:    adminRoleID,
					MenuID:    menu.ID,
					CreatedBy: adminUserID,
				}
				if err := DB.Create(&permission).Error; err != nil {
					logger.Warnf("Failed to create menu permission for menu %s: %v", menu.ID, err)
				} else {
					createdCount++
				}
			}
		}

		logger.Infof("Created %d menu permissions for admin role", createdCount)
		return nil
	*/
}
