package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/go-redis/redis/v8"
)

var (
	// Client 全局 Redis 客户端（nil表示Redis未启用）
	Client *redis.Client

	// IsRedisEnabled 标记Redis是否已启用（即使连接失败，如果配置中启用了，这个值也为true）
	isRedisEnabled bool

	// cleanupFns 在 Close 前执行的清理函数（如停止 PubSub），避免 "use of closed network connection"
	cleanupFns   []func()
	cleanupFnsMu sync.Mutex
)

// Init 初始化 Redis 连接
// 如果Redis未启用或连接失败，会优雅降级，不影响主服务启动
func Init(cfg *config.RedisConfig) error {
	if !cfg.Enabled {
		logger.Info("[Redis] Redis is disabled in config - using database mode")
		isRedisEnabled = false
		return nil
	}

	// 设置默认值
	cfg.SetDefaults()

	// 创建Redis客户端
	Client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  time.Duration(cfg.ConnectTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// 测试连接（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		// Redis连接失败，关闭客户端并降级
		Client.Close()
		Client = nil
		isRedisEnabled = false
		return fmt.Errorf("failed to connect to Redis at %s:%d: %w (will use database mode)", cfg.Host, cfg.Port, err)
	}

	isRedisEnabled = true
	logger.Infof("[Redis] Connected to Redis at %s:%d (DB: %d, PoolSize: %d)",
		cfg.Host, cfg.Port, cfg.DB, cfg.PoolSize)
	return nil
}

// RegisterCleanup 注册在 Close 前执行的清理函数（如停止 PubSub 监听）。
// 避免先关闭 Client 导致仍在运行的 PubSub 报 "use of closed network connection"。
func RegisterCleanup(fn func()) {
	cleanupFnsMu.Lock()
	defer cleanupFnsMu.Unlock()
	cleanupFns = append(cleanupFns, fn)
}

// Close 关闭 Redis 连接（先执行所有已注册的清理函数，再关闭客户端）
func Close() error {
	cleanupFnsMu.Lock()
	fns := cleanupFns
	cleanupFns = nil
	cleanupFnsMu.Unlock()
	for _, fn := range fns {
		fn()
	}
	if Client != nil {
		err := Client.Close()
		Client = nil
		isRedisEnabled = false
		return err
	}
	return nil
}

// IsEnabled 检查 Redis 是否已启用且连接正常
func IsEnabled() bool {
	return Client != nil && isRedisEnabled
}

// IsConfigured 检查 Redis 是否在配置中启用（即使连接失败也返回true）
func IsConfigured() bool {
	return isRedisEnabled
}

// GetClient 获取Redis客户端（如果未启用则返回nil）
func GetClient() *redis.Client {
	return Client
}
