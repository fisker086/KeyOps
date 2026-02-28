package dms

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/crypto"
	"github.com/go-redis/redis/v8"
)

type RedisExecutor struct {
	instance *model.DBInstance
	crypto   *crypto.Crypto
	client   redis.UniversalClient
	password string
}

func NewRedisExecutor(instance *model.DBInstance, crypto *crypto.Crypto) (*RedisExecutor, error) {
	password, err := crypto.Decrypt(instance.Password)
	if err != nil {
		return nil, fmt.Errorf("解密密码失败: %w", err)
	}

	return &RedisExecutor{
		instance: instance,
		crypto:   crypto,
		password: password,
	}, nil
}

func (e *RedisExecutor) getClient(ctx context.Context) (redis.UniversalClient, error) {
	if e.client != nil {
		return e.client, nil
	}

	options := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", e.instance.Host, e.instance.Port),
		Password:     e.password,
		DB:           0,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if e.instance.Username != "" {
		options.Username = e.instance.Username
	}

	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	e.client = client
	return client, nil
}

func (e *RedisExecutor) TestConnection(ctx context.Context) error {
	client, err := e.getClient(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return client.Ping(ctx).Err()
}

func (e *RedisExecutor) GetDatabases(ctx context.Context) ([]string, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	config, err := client.ConfigGet(ctx, "databases").Result()
	if err != nil {
		var dbs []string
		for i := 0; i < 16; i++ {
			dbs = append(dbs, strconv.Itoa(i))
		}
		return dbs, nil
	}

	dbCount := 16
	for i := 0; i < len(config); i += 2 {
		if config[i] == "databases" {
			if val, ok := config[i+1].(string); ok {
				if count, err := strconv.Atoi(val); err == nil {
					dbCount = count
				}
			}
			break
		}
	}

	var databases []string
	for i := 0; i < dbCount; i++ {
		databases = append(databases, strconv.Itoa(i))
	}

	return databases, nil
}

func (e *RedisExecutor) GetTables(ctx context.Context, databaseName string) ([]string, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dbNum, _ := strconv.Atoi(databaseName)
	if dbNum > 0 {
		if err := client.Do(ctx, "SELECT", dbNum).Err(); err != nil {
			return nil, fmt.Errorf("选择数据库失败: %w", err)
		}
	}

	var keys []string
	cursor := uint64(0)
	maxResults := 100

	for {
		var batch []string
		var err error
		batch, cursor, err = client.Scan(ctx, cursor, "", 20).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描 keys 失败: %w", err)
		}

		keys = append(keys, batch...)
		if cursor == 0 || len(keys) >= maxResults {
			break
		}
	}

	return keys[:min(len(keys), maxResults)], nil
}

func (e *RedisExecutor) GetColumns(ctx context.Context, databaseName, tableName string) ([]ColumnInfo, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	keyType, err := client.Type(ctx, tableName).Result()
	if err != nil {
		return nil, fmt.Errorf("获取 key 类型失败: %w", err)
	}

	return []ColumnInfo{
		{
			Name:     "key",
			Type:     keyType,
			Nullable: false,
			Comment:  fmt.Sprintf("Redis key 类型: %s", keyType),
		},
	}, nil
}

func (e *RedisExecutor) ExecuteQuery(ctx context.Context, databaseName, query string, limit int) (*QueryResult, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	parts := strings.Fields(query)
	if len(parts) == 0 {
		return &QueryResult{Success: false, Error: "命令不能为空"}, nil
	}

	command := strings.ToUpper(parts[0])

	// 只读命令列表（与 detectRedisType 保持一致）
	safeCommands := map[string]bool{
		// String 操作
		"GET": true, "MGET": true, "STRLEN": true,
		// Hash 操作
		"HGET": true, "HGETALL": true, "HMGET": true, "HKEYS": true,
		"HVALS": true, "HEXISTS": true, "HLEN": true, "HSCAN": true,
		// Set 操作
		"SMEMBERS": true, "SCARD": true, "SISMEMBER": true,
		"SDIFF": true, "SUNION": true, "SSCAN": true,
		// List 操作
		"LRANGE": true, "LLEN": true, "LINDEX": true,
		// Sorted Set 操作
		"ZRANGE": true, "ZRANGEBYSCORE": true, "ZSCORE": true,
		"ZCARD": true, "ZCOUNT": true, "ZRANK": true, "ZSCAN": true,
		// Key 操作
		"EXISTS": true, "TTL": true, "PTTL": true, "TYPE": true,
		"KEYS": true, "SCAN": true,
		// Server 操作（只读）
		"INFO": true, "CLIENT": true, "TIME": true, "DBSIZE": true,
	}

	if !safeCommands[command] {
		return &QueryResult{
			Success: false,
			Error:   fmt.Sprintf("禁止执行该命令: %s，仅允许只读命令", command),
		}, nil
	}

	args := make([]interface{}, len(parts))
	for i, v := range parts {
		args[i] = v
	}
	cmd := client.Do(ctx, args...)
	if cmd.Err() != nil {
		return &QueryResult{Success: false, Error: cmd.Err().Error()}, nil
	}

	result := cmd.Val()
	var rows [][]interface{}
	var columns []string

	switch v := result.(type) {
	case []interface{}:
		columns = []string{"Result"}
		for _, item := range v {
			rows = append(rows, []interface{}{fmt.Sprintf("%v", item)})
		}
	case map[string]interface{}:
		columns = []string{"Field", "Value"}
		for k, val := range v {
			rows = append(rows, []interface{}{k, fmt.Sprintf("%v", val)})
		}
	case string:
		columns = []string{"Result"}
		rows = append(rows, []interface{}{v})
	case int64:
		columns = []string{"Result"}
		rows = append(rows, []interface{}{v})
	default:
		columns = []string{"Result"}
		rows = append(rows, []interface{}{fmt.Sprintf("%v", result)})
	}

	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	return &QueryResult{
		Success:      true,
		Columns:      columns,
		Rows:         rows,
		ResultCount:  len(rows),
		AffectedRows: int64(len(rows)),
	}, nil
}

func (e *RedisExecutor) ExecuteUpdate(ctx context.Context, databaseName, query string) (*QueryResult, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	// 根据命令大小动态设置超时时间
	cmdSize := len(query)
	timeout := 30 * time.Second
	if cmdSize > 1024*1024 { // 大于1MB
		timeout = 300 * time.Second // 5分钟
	} else if cmdSize > 100*1024 { // 大于100KB
		timeout = 120 * time.Second // 2分钟
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	parts := strings.Fields(query)
	if len(parts) == 0 {
		return &QueryResult{Success: false, Error: "命令不能为空"}, nil
	}

	command := strings.ToUpper(parts[0])

	// 禁止的危险命令
	dangerousCommands := map[string]bool{
		"FLUSHDB": true, "FLUSHALL": true, "SHUTDOWN": true,
		"CONFIG": true, "DEBUG": true, "EVAL": true, "EVALSHA": true,
		"SCRIPT": true, "REPLICAOF": true, "SLAVEOF": true,
	}
	if dangerousCommands[command] {
		return &QueryResult{
			Success:      false,
			Error:        fmt.Sprintf("禁止执行危险命令: %s", command),
			AffectedRows: 0,
		}, nil
	}

	// 选择数据库
	dbNum, _ := strconv.Atoi(databaseName)
	if dbNum > 0 {
		if err := client.Do(ctx, "SELECT", dbNum).Err(); err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("选择数据库失败: %v", err)}, nil
		}
	}

	// 执行命令
	args := make([]interface{}, len(parts))
	for i, v := range parts {
		args[i] = v
	}
	cmd := client.Do(ctx, args...)
	if cmd.Err() != nil {
		return &QueryResult{Success: false, Error: cmd.Err().Error()}, nil
	}

	result := cmd.Val()
	var affectedRows int64 = 0

	// 根据命令类型判断影响行数
	switch command {
	case "SET", "SETEX", "SETNX", "MSET", "HSET", "HSETNX", "HMSET",
		"SADD", "ZADD", "LPUSH", "RPUSH", "LPUSHX", "RPUSHX",
		"LINSERT", "LSET", "APPEND", "INCR", "DECR", "INCRBY", "DECRBY",
		"INCRBYFLOAT", "HINCRBY", "HINCRBYFLOAT", "ZINCRBY":
		affectedRows = 1
	case "DEL", "HDEL", "SREM", "ZREM", "LREM", "LTRIM", "ZREMRANGEBYRANK",
		"ZREMRANGEBYSCORE", "ZREMRANGEBYLEX":
		// DEL 等命令返回删除的数量
		if val, ok := result.(int64); ok {
			affectedRows = val
		} else {
			affectedRows = 1
		}
	case "EXPIRE", "EXPIREAT", "PEXPIRE", "PEXPIREAT", "PERSIST":
		// 返回 1 表示成功，0 表示 key 不存在或操作失败
		if val, ok := result.(int64); ok {
			affectedRows = val
		}
	}

	return &QueryResult{
		Success:      true,
		AffectedRows: affectedRows,
		ResultCount:  0,
		Rows:         [][]interface{}{{fmt.Sprintf("%v", result)}},
		Columns:      []string{"Result"},
	}, nil
}

func (e *RedisExecutor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
