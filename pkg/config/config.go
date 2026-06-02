package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	MongoDB  MongoDBConfig  `yaml:"mongodb"`
	Redis    RedisConfig    `yaml:"redis"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
	SSH      SSHConfig      `yaml:"ssh"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	Sync     SyncConfig     `yaml:"sync"`
	// Pulumi removed, replaced by pkg/resource engine
	Deploy         DeployConfig         `yaml:"deploy"`
	Helm           HelmConfig           `yaml:"helm"`
	BastionStorage BastionStorageConfig `yaml:"bastion_storage"`
	BillStorage    BillStorageConfig    `yaml:"bill_storage"`
	Sites          []string             `yaml:"sites"`
}

// BastionStorageConfig 堡垒机存储配置
type BastionStorageConfig struct {
	Engine  string           `yaml:"engine"` // mysql / mongodb
	MongoDB BastionMongoYAML `yaml:"mongodb"`
}

// BastionMongoYAML 堡垒机专用 Mongo 配置（含集合名）
type BastionMongoYAML struct {
	MongoConfig `yaml:",inline"`
	// 以下为可选；空则 SetDefaults 填入默认集合名
	CollectionLogins     string `yaml:"collection_logins"`
	CollectionRecordings string `yaml:"collection_recordings"`
	CollectionCommands   string `yaml:"collection_commands"`
}

func (c *BastionStorageConfig) GetEngine() string {
	if c.Engine == "" {
		return "mysql"
	}
	return c.Engine
}

func (c *BastionStorageConfig) SetDefaults() {
	if c.Engine == "" {
		c.Engine = "mysql"
	}
	if c.MongoDB.Database == "" {
		c.MongoDB.Database = "keyops_bastion"
	}
	if c.Engine == "mongodb" && c.MongoDB.URI == "" {
		c.MongoDB.URI = "mongodb://localhost:27017"
	}
	if c.MongoDB.CollectionLogins == "" {
		c.MongoDB.CollectionLogins = "bastion_logins"
	}
	if c.MongoDB.CollectionRecordings == "" {
		c.MongoDB.CollectionRecordings = "bastion_recordings"
	}
	if c.MongoDB.CollectionCommands == "" {
		c.MongoDB.CollectionCommands = "bastion_commands"
	}
}

func (c *BastionStorageConfig) GetMongoURI() string {
	return c.MongoDB.GetURI()
}

// MongoConfig MongoDB 配置（内嵌到 bastion_storage 或 bill_storage）
type MongoConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

func (c *MongoConfig) GetURI() string {
	return c.URI
}

// BillStorageConfig 账单存储配置（复用 MongoConfig）
type BillStorageConfig struct {
	MongoConfig `yaml:",inline"`
}

func (c *BillStorageConfig) SetDefaults() {
	if c.Database == "" {
		c.Database = "keyops_bill"
	}
}

func (c *BillStorageConfig) GetURI() string {
	return c.URI
}

// DeployConfig 非容器部署相关配置（Git 拉取 Playbook 等）
type DeployConfig struct {
	Git DeployGitConfig `yaml:"git"`
}

// HelmConfig Helm Chart 配置
type HelmConfig struct {
	// Source Chart 来源：local（本地目录）/ remote（远程仓库）
	Source string `yaml:"source"`

	// LocalPath 本地 Chart 路径（source=local 时使用）
	// 支持绝对路径或相对路径（相对于项目根目录）
	LocalPath string `yaml:"local_path"`

	// DefaultChartName 默认 Chart 名称（当应用未指定时使用）
	// 适用于多个应用共用一个模板的场景
	DefaultChartName string `yaml:"default_chart_name"`

	// DefaultChartVersion 默认 Chart 版本（当应用未指定时使用）
	// 仅在使用远程仓库时生效，本地 Chart 不需要版本号
	// 留空则使用最新版本
	DefaultChartVersion string `yaml:"default_chart_version"`

	// RemoteRepo 远程仓库配置（source=remote 时使用）
	RemoteRepo HelmRemoteRepoConfig `yaml:"remote_repo"`
}

// HelmRemoteRepoConfig 远程 Helm 仓库配置
type HelmRemoteRepoConfig struct {
	// URL 仓库地址
	// 支持格式：
	//   - HTTP/HTTPS: https://charts.bitnami.com/bitnami
	//   - OCI: oci://registry-1.docker.io/bitnamicharts
	//   - Harbor: https://harbor.company.com/chartrepo/library
	URL string `yaml:"url"`

	// Username 仓库用户名（私有仓库需要）
	Username string `yaml:"username"`

	// Password 仓库密码或 Token（私有仓库需要）
	Password string `yaml:"password"`

	// InsecureSkipTLSVerify 是否跳过 TLS 验证（自签名证书时使用）
	InsecureSkipTLSVerify bool `yaml:"insecure_skip_tls_verify"`
}

// SetDefaults 设置 Helm 配置默认值
func (c *HelmConfig) SetDefaults() {
	if c.Source == "" {
		c.Source = "local" // 默认使用本地目录
	}
	if c.LocalPath == "" {
		c.LocalPath = "./charts/standard-app" // 默认本地路径
	}
	if c.DefaultChartName == "" {
		c.DefaultChartName = "standard-app" // 默认 Chart 名称
	}
}

// GetChartPath 获取 Chart 路径（根据配置返回本地路径或空字符串）
func (c *HelmConfig) GetChartPath() string {
	if c.Source == "local" {
		return c.LocalPath
	}
	return "" // remote 模式返回空，由数据库配置决定
}

// IsRemote 是否使用远程仓库
func (c *HelmConfig) IsRemote() bool {
	return c.Source == "remote"
}

// DeployGitConfig Git 拉取认证与缓存
// 拉私有仓库时使用 Username + Password（或 Personal Access Token）
type DeployGitConfig struct {
	Username string `yaml:"username"`  // Git 用户名
	Password string `yaml:"password"`  // 密码或 token
	CacheDir string `yaml:"cache_dir"` // 仓库本地缓存目录
}

// SetDefaults 设置非容器部署配置默认值
func (c *DeployConfig) SetDefaults() {
	if c.Git.CacheDir == "" {
		c.Git.CacheDir = "./data/deploy_repos"
	}
}

type ServerConfig struct {
	APIPort          int    `yaml:"api_port"`
	SSHPort          int    `yaml:"ssh_port"` // SSH Gateway 端口
	LinuxProxyPort   int    `yaml:"linux_proxy_port"`
	WindowsProxyPort int    `yaml:"windows_proxy_port"` // WIP: 计划支持 RDP
	BackendURL       string `yaml:"backend_url"`
	Mode             string `yaml:"mode"`
	ProxyID          string `yaml:"proxy_id"` // 可选：指定固定的 Proxy ID
}

type DatabaseConfig struct {
	Driver          string `yaml:"driver"` // 数据库驱动: mysql, postgres (默认: mysql)
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"dbname"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	// Enabled 是否启用Redis
	// - true: 启用Redis，支持分布式特性（如Casbin多机器同步、分布式锁等）
	// - false: 禁用Redis，使用数据库模式（单机部署或不需要分布式特性时）
	Enabled bool `yaml:"enabled"`

	// Host Redis服务器地址（仅在enabled=true时有效）
	Host string `yaml:"host"`

	// Port Redis服务器端口（仅在enabled=true时有效）
	Port int `yaml:"port"`

	// Password Redis密码（可选，如果Redis未设置密码则留空）
	Password string `yaml:"password"`

	// DB Redis数据库编号（默认0）
	DB int `yaml:"db"`

	// ConnectTimeout 连接超时时间（秒，默认5秒）
	ConnectTimeout int `yaml:"connect_timeout"`

	// ReadTimeout 读取超时时间（秒，默认3秒）
	ReadTimeout int `yaml:"read_timeout"`

	// WriteTimeout 写入超时时间（秒，默认3秒）
	WriteTimeout int `yaml:"write_timeout"`

	// PoolSize 连接池大小（默认10）
	PoolSize int `yaml:"pool_size"`

	// MinIdleConns 最小空闲连接数（默认5）
	MinIdleConns int `yaml:"min_idle_conns"`
}

// Validate 验证Redis配置
func (c *RedisConfig) Validate() error {
	if !c.Enabled {
		return nil // Redis未启用，无需验证
	}

	if c.Host == "" {
		return fmt.Errorf("redis host is required when enabled=true")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Port)
	}

	return nil
}

// SetDefaults 设置默认值
func (c *RedisConfig) SetDefaults() {
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 5
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.MinIdleConns == 0 {
		c.MinIdleConns = 5
	}
}

type SecurityConfig struct {
	// JWTSecret JWT签名密钥（建议64字节或更长，更安全）
	// AES-256加密密钥会自动从此密钥提取前32字节用于加密SSH私钥等敏感数据
	JWTSecret string `yaml:"jwt_secret"`

	// SessionTimeout 会话超时时间（秒）
	SessionTimeout int `yaml:"session_timeout"`

	// AdminWhitelist 自动管理员白名单：英文逗号分隔完整邮箱，与 users.email 比对（不区分大小写）；须含 @。
	// 密码 / LDAP / SSO 登录成功且邮箱命中时提升为 admin。环境变量 ADMIN_WHITELIST 非空时覆盖本字段。
	AdminWhitelist string `yaml:"admin_whitelist"`

	// AuthMethod 认证方式：password / ldap / sso。非空时覆盖数据库中的认证方式（与 AUTH_METHOD 一致，AUTH_METHOD 优先）。
	// 留空则完全以系统设置（数据库）为准。用于 config 或环境强制恢复登录方式。
	AuthMethod string `yaml:"auth_method"`
}

// SetDefaults 设置安全配置的默认值
func (c *SecurityConfig) SetDefaults() {
	if c.JWTSecret == "" {
		// 默认JWT密钥（64字节，使用openssl生成的随机字符串，仅用于开发环境）
		// 生产环境必须修改为强随机字符串
		c.JWTSecret = "DdzI7wyean0JDT86fIEY+XEPKa+swZRkAlDUojBhnUQUta4KY/EG3JnnI6mDSrxV"
	}
}

type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug / info / warn / error
	Output     string `yaml:"output"`      // console / file / both
	File       string `yaml:"file"`        // 日志文件路径
	MaxSize    int    `yaml:"max_size"`    // 单个文件最大大小（MB）
	MaxBackups int    `yaml:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `yaml:"max_age"`     // 保留日志的最大天数
	Compress   bool   `yaml:"compress"`    // 是否压缩旧日志
}

type SSHConfig struct {
	Timeout           int `yaml:"timeout"`
	KeepaliveInterval int `yaml:"keepalive_interval"`
	MaxSessions       int `yaml:"max_sessions"`
}

type ProxyConfig struct {
	// Enabled 是否启用Proxy功能
	// - true: 启用Proxy功能，会启动ProxyMonitor监控代理服务器状态
	// - false: 禁用Proxy功能，不启动ProxyMonitor（适用于不使用代理模式的部署）
	// 默认值为 false，如果配置文件中没有 proxy 配置项，则默认禁用
	Enabled bool `yaml:"enabled"`
}

// SetDefaults 设置Proxy配置的默认值
func (c *ProxyConfig) SetDefaults() {
	// Enabled 默认为 false，如果配置文件中没有指定，则不启用 Proxy
	// 这样即使配置文件中没有 proxy 配置项，也不会启动 ProxyMonitor
}

// RDP 配置已移至数据库 setting 表，不再从 config.yaml 读取
// 所有 RDP 相关配置（guacd_host, guacd_port, recording_enabled 等）都通过数据库管理

type SyncConfig struct {
	Interval       int    `yaml:"interval"`         // 同步间隔（秒），默认60秒
	CleanupDays    int    `yaml:"cleanup_days"`     // 清理已同步数据的天数，默认7天
	BatchSize      int    `yaml:"batch_size"`       // 每次同步的批量大小，默认1000
	BillSyncHour   int    `yaml:"bill_sync_hour"`   // 账单同步执行小时，默认2点
	BillSyncMinute int    `yaml:"bill_sync_minute"` // 账单同步执行分钟，默认0分
	BillSyncCron   string `yaml:"bill_sync_cron"`   // Cron表达式，支持5段或6段；为空时由hour/minute生成
}

func (c *SyncConfig) SetDefaults() {
	if c.Interval == 0 {
		c.Interval = 60
	}
	if c.CleanupDays == 0 {
		c.CleanupDays = 7
	}
	if c.BatchSize == 0 {
		c.BatchSize = 1000
	}
	if c.BillSyncHour == 0 {
		c.BillSyncHour = 2
	}
	if c.BillSyncHour < 0 || c.BillSyncHour > 23 {
		c.BillSyncHour = 2
	}
	if c.BillSyncMinute < 0 || c.BillSyncMinute > 59 {
		c.BillSyncMinute = 0
	}
	if c.BillSyncCron == "" {
		c.BillSyncCron = fmt.Sprintf("0 %d %d * * *", c.BillSyncMinute, c.BillSyncHour)
	}
}

type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

func (c *MongoDBConfig) SetDefaults() {
	if c.Database == "" {
		c.Database = "zjump_bill"
	}
}

func (c *MongoDBConfig) GetURI() string {
	return c.URI
}

var GlobalConfig *Config

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值（数据库默认值需要在环境变量处理之前设置）
	config.Database.SetDefaults()
	config.Redis.SetDefaults()
	config.Security.SetDefaults()
	config.Proxy.SetDefaults()
	config.Deploy.SetDefaults()
	config.Helm.SetDefaults()
	config.MongoDB.SetDefaults()
	config.BastionStorage.SetDefaults()
	config.BillStorage.SetDefaults()
	config.Sync.SetDefaults()

	// 验证配置
	if err := config.Redis.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	// 支持通过环境变量覆盖数据库配置（Docker 部署时使用）
	// 数据库驱动类型: mysql, postgres (默认: mysql)
	if dbDriver := os.Getenv("DB_DRIVER"); dbDriver != "" {
		config.Database.Driver = dbDriver
	}
	// 数据库地址
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	// 数据库端口
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			config.Database.Port = port
		}
	}
	// 数据库用户名
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.User = dbUser
	}
	// 数据库密码
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		config.Database.Password = dbPassword
	}
	// 数据库名称
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.DBName = dbName
	}

	// 设置数据库默认值（包括 driver 的默认值）
	config.Database.SetDefaults()

	// 支持通过环境变量覆盖Redis配置（Docker 部署时使用）
	// Redis是否启用
	if redisEnabled := os.Getenv("REDIS_ENABLED"); redisEnabled != "" {
		if enabled, err := strconv.ParseBool(redisEnabled); err == nil {
			config.Redis.Enabled = enabled
		}
	}
	// Redis地址
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		config.Redis.Host = redisHost
	}
	// Redis端口
	if redisPort := os.Getenv("REDIS_PORT"); redisPort != "" {
		if port, err := strconv.Atoi(redisPort); err == nil {
			config.Redis.Port = port
		}
	}
	// Redis密码
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		config.Redis.Password = redisPassword
	}
	// Redis数据库编号
	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			config.Redis.DB = db
		}
	}

	// 重新设置Redis默认值（环境变量可能覆盖了某些值）
	config.Redis.SetDefaults()

	// 重新验证Redis配置（环境变量可能改变了配置）
	if err := config.Redis.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	// 支持通过环境变量覆盖 MongoDB 配置
	if v := os.Getenv("MONGO_URI"); v != "" {
		config.MongoDB.URI = v
	}
	if v := os.Getenv("MONGO_DATABASE"); v != "" {
		config.MongoDB.Database = v
	}
	if v := os.Getenv("BILL_MONGO_URI"); v != "" {
		config.BillStorage.URI = v
	}
	if v := os.Getenv("BILL_MONGO_DATABASE"); v != "" {
		config.BillStorage.Database = v
	}
	if v := os.Getenv("BILL_SYNC_CRON"); v != "" {
		config.Sync.BillSyncCron = v
	}

	// 堡垒机存储配置环境变量
	if v := os.Getenv("BASTION_STORAGE_ENGINE"); v != "" {
		config.BastionStorage.Engine = v
	}
	if v := os.Getenv("BASTION_MONGO_URI"); v != "" {
		config.BastionStorage.MongoDB.URI = v
	}

	// 非容器部署 - Git 拉取认证（环境变量覆盖，敏感信息可不写进配置文件）
	if v := os.Getenv("DEPLOY_GIT_USERNAME"); v != "" {
		config.Deploy.Git.Username = v
	}
	if v := os.Getenv("DEPLOY_GIT_PASSWORD"); v != "" {
		config.Deploy.Git.Password = v
	}
	if v := os.Getenv("DEPLOY_GIT_CACHE_DIR"); v != "" {
		config.Deploy.Git.CacheDir = v
	}

	// Helm 配置环境变量覆盖
	if v := os.Getenv("HELM_SOURCE"); v != "" {
		config.Helm.Source = v
	}
	if v := os.Getenv("HELM_LOCAL_PATH"); v != "" {
		config.Helm.LocalPath = v
	}
	if v := os.Getenv("HELM_REMOTE_URL"); v != "" {
		config.Helm.RemoteRepo.URL = v
	}
	if v := os.Getenv("HELM_REMOTE_USERNAME"); v != "" {
		config.Helm.RemoteRepo.Username = v
	}
	if v := os.Getenv("HELM_REMOTE_PASSWORD"); v != "" {
		config.Helm.RemoteRepo.Password = v
	}

	// ADMIN_WHITELIST：与 admin_whitelist 相同（逗号分隔完整邮箱，见 SecurityConfig.AdminWhitelist）
	if v := os.Getenv("ADMIN_WHITELIST"); v != "" {
		config.Security.AdminWhitelist = v
	}

	// AUTH_METHOD：password / ldap / sso，覆盖 config.yaml 的 security.auth_method（用于容器紧急指定）
	if v := os.Getenv("AUTH_METHOD"); v != "" {
		config.Security.AuthMethod = v
	}

	GlobalConfig = &config
	return &config, nil
}

func (c *DatabaseConfig) DSN() string {
	if c.Driver == "postgres" || c.Driver == "postgresql" {
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.User, c.Password, c.DBName)
	}
	// 默认 MySQL
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

// SetDefaults 设置默认值
func (c *DatabaseConfig) SetDefaults() {
	if c.Driver == "" {
		c.Driver = "mysql"
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 10
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 100
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 3600 // 1 hour
	}
}
