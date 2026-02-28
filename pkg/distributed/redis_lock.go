package distributed

import (
	"context"
	"fmt"
	"time"

	"github.com/fisker086/keyops/pkg/logger"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// RedisLock Redis 分布式锁
type RedisLock struct {
	client   *redis.Client
	key      string
	value    string
	expiry   time.Duration
	ctx      context.Context
	cancelFn context.CancelFunc
}

// NewRedisLock 创建 Redis 分布式锁
// 如果client为nil（Redis未启用），返回的锁会立即失败，但不影响主流程
func NewRedisLock(client *redis.Client, key string, expiry time.Duration) *RedisLock {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisLock{
		client:   client,
		key:      key,
		value:    uuid.New().String(), // 使用 UUID 作为锁的值，防止误释放
		expiry:   expiry,
		ctx:      ctx,
		cancelFn: cancel,
	}
}

// TryLock 尝试获取锁（非阻塞）
// 如果Redis未启用（client为nil），返回false但不报错（优雅降级）
func (l *RedisLock) TryLock() (bool, error) {
	if l.client == nil {
		// Redis未启用，降级为单机模式（不获取锁，直接返回false）
		logger.Infof("[RedisLock] Redis not available, lock %s will not be acquired (single-server mode)", l.key)
		return false, nil
	}

	// 使用 SET NX EX 命令：如果 key 不存在则设置，并设置过期时间
	result, err := l.client.SetNX(l.ctx, l.key, l.value, l.expiry).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// 如果获取到锁，启动自动续期
	if result {
		go l.autoRenew()
	}

	return result, nil
}

// Unlock 释放锁
// 如果Redis未启用（client为nil），直接返回nil（优雅降级）
func (l *RedisLock) Unlock() error {
	if l.client == nil {
		// Redis未启用，无需释放锁
		// 仍然取消上下文以避免资源泄漏
		l.cancelFn()
		return nil
	}

	// 使用 Lua 脚本保证原子性：只有持有锁的实例才能释放
	// 使用 context.Background() 而不是 l.ctx，因为我们需要在取消上下文之前完成解锁操作
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.Eval(context.Background(), script, []string{l.key}, l.value).Result()
	if err != nil {
		// 即使解锁失败，也要取消上下文以停止自动续期
		l.cancelFn()
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// 解锁操作完成后，取消上下文以停止自动续期
	l.cancelFn()

	if result == int64(0) {
		logger.Warnf("[RedisLock] Lock %s was not held by this instance", l.key)
	}

	return nil
}

// autoRenew 自动续期锁（每隔 expiry/3 续期一次）
func (l *RedisLock) autoRenew() {
	ticker := time.NewTicker(l.expiry / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 使用 Lua 脚本续期：只有持有锁的实例才能续期
			script := `
				if redis.call("get", KEYS[1]) == ARGV[1] then
					return redis.call("expire", KEYS[1], ARGV[2])
				else
					return 0
				end
			`

			result, err := l.client.Eval(l.ctx, script, []string{l.key}, l.value, int(l.expiry.Seconds())).Result()
			if err != nil {
				logger.Warnf("[RedisLock] Failed to renew lock %s: %v", l.key, err)
				return
			}

			if result == int64(0) {
				logger.Warnf("[RedisLock] Lost lock %s, stopping auto-renew", l.key)
				return
			}

		case <-l.ctx.Done():
			return
		}
	}
}

// IsLocked 检查锁是否存在
// 如果Redis未启用（client为nil），返回false（优雅降级）
func (l *RedisLock) IsLocked() (bool, error) {
	if l.client == nil {
		return false, nil
	}

	result, err := l.client.Exists(l.ctx, l.key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
