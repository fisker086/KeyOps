package dms

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/crypto"
	"github.com/fisker086/keyops/pkg/logger"
)

type QueryService struct {
	instanceRepo   *repository.DBInstanceRepository
	queryLogRepo   *repository.QueryLogRepository
	permissionSvc  *PermissionService
	crypto         *crypto.Crypto
}

func NewQueryService(
	instanceRepo *repository.DBInstanceRepository,
	queryLogRepo *repository.QueryLogRepository,
	permissionSvc *PermissionService,
	crypto *crypto.Crypto,
) *QueryService {
	return &QueryService{
		instanceRepo:  instanceRepo,
		queryLogRepo:  queryLogRepo,
		permissionSvc: permissionSvc,
		crypto:        crypto,
	}
}

// ExecuteQuery 执行查询
func (s *QueryService) ExecuteQuery(req *ExecuteQueryRequest, userID, username, clientIP, userAgent string) (*QueryResult, error) {
	// 1. 获取实例
	instance, err := s.instanceRepo.GetByID(req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}

	// 2. 检测查询类型并验证权限
	var sqlType string
	var requiredPermission string
	if instance.DBType == "redis" {
		sqlType = s.detectRedisType(req.Query)
		switch sqlType {
		case "READ":
			requiredPermission = "read"
		case "WRITE":
			requiredPermission = "write"
		case "ADMIN":
			requiredPermission = "admin"
		default:
			requiredPermission = "read" // 默认只读
		}
	} else if instance.DBType == "mongodb" {
		sqlType = s.detectMongoType(req.Query)
		switch sqlType {
		case "FIND", "FINDONE":
			requiredPermission = "read"
		case "INSERTONE", "INSERTMANY", "UPDATEONE", "UPDATEMANY", "DELETEONE", "DELETEMANY":
			requiredPermission = "write"
		default:
			requiredPermission = "read" // 默认只读
		}
	} else {
		sqlType = s.detectSQLType(req.Query)
		switch sqlType {
		case "SELECT", "SHOW", "DESCRIBE", "EXPLAIN":
			requiredPermission = "read"
		case "INSERT", "UPDATE", "DELETE", "REPLACE":
			requiredPermission = "write"
		case "CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME":
			requiredPermission = "admin"
		default:
			requiredPermission = "read" // 默认只读
		}
	}

	// 3. 提取表名（简化版）
	tableName := s.extractTableName(req.Query)

	// 4. 检查权限
	hasPermission, err := s.permissionSvc.CheckPermission(userID, req.InstanceID, req.DatabaseName, tableName, requiredPermission)
	if err != nil {
		return nil, fmt.Errorf("权限检查失败: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("无%s权限执行%s操作", requiredPermission, sqlType)
	}

	// 5. 创建执行器并执行查询
	executor, err := NewExecutor(instance, s.crypto)
	if err != nil {
		return nil, fmt.Errorf("创建执行器失败: %w", err)
	}
	defer executor.Close()

	startTime := time.Now()
	var isSelect bool
	if instance.DBType == "redis" {
		isSelect = sqlType == "READ"
	} else if instance.DBType == "mongodb" {
		isSelect = sqlType == "FIND" || sqlType == "FINDONE"
	} else {
		isSelect = sqlType == "SELECT" || sqlType == "SHOW" || sqlType == "DESCRIBE" || sqlType == "EXPLAIN"
	}

	// 根据SQL大小动态设置超时时间
	// 对于大SQL（>1MB），增加超时时间到5分钟
	sqlSize := len(req.Query)
	timeout := 30 * time.Second
	if sqlSize > 1024*1024 { // 大于1MB
		timeout = 300 * time.Second // 5分钟
	} else if sqlSize > 100*1024 { // 大于100KB
		timeout = 120 * time.Second // 2分钟
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	var result *QueryResult
	if isSelect {
		result, err = executor.ExecuteQuery(ctx, req.DatabaseName, req.Query, 0) // limit 0 表示不限制
	} else {
		result, err = executor.ExecuteUpdate(ctx, req.DatabaseName, req.Query)
	}
	executionTime := time.Since(startTime).Milliseconds()

	if err != nil {
		return nil, err
	}

	// 6. 记录日志（异步）
	go s.logQueryAsync(&model.QueryLog{
		UserID:        userID,
		Username:      username,
		InstanceID:    req.InstanceID,
		InstanceName:  instance.Name,
		DBType:        instance.DBType,
		DatabaseName:  req.DatabaseName,
		QueryContent:  req.Query,
		QueryType:     sqlType,
		AffectedRows:  int(result.AffectedRows),
		ResultCount:   result.ResultCount,
		ExecutionTimeMs: int(executionTime),
		Status:        getStatus(result),
		ErrorMessage:  result.Error,
		ResultPreview: truncateString(formatResultPreview(result), 1000),
		ClientIP:      clientIP,
		UserAgent:     userAgent,
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// executeQueryByType 已废弃，使用 Executor 模式

// GetDatabases 获取数据库列表（使用 Executor）
func (s *QueryService) GetDatabases(instanceID uint) ([]string, error) {
	instance, err := s.instanceRepo.GetByID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}

	executor, err := NewExecutor(instance, s.crypto)
	if err != nil {
		return nil, err
	}
	defer executor.Close()

	ctx := context.Background()
	return executor.GetDatabases(ctx)
}

// GetTables 获取表列表（使用 Executor）
func (s *QueryService) GetTables(instanceID uint, databaseName string) ([]string, error) {
	instance, err := s.instanceRepo.GetByID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}

	executor, err := NewExecutor(instance, s.crypto)
	if err != nil {
		return nil, err
	}
	defer executor.Close()

	ctx := context.Background()
	return executor.GetTables(ctx, databaseName)
}

// detectSQLType 检测 SQL 类型
func (s *QueryService) detectSQLType(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return ""
	}

	upperSQL := strings.ToUpper(sql)
	words := strings.Fields(upperSQL)
	if len(words) == 0 {
		return ""
	}

	firstWord := words[0]

	readOps := []string{"SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "USE"}
	for _, op := range readOps {
		if firstWord == op {
			return "SELECT"
		}
	}

	writeOps := []string{"INSERT", "UPDATE", "DELETE", "REPLACE"}
	for _, op := range writeOps {
		if firstWord == op {
			return firstWord
		}
	}

	adminOps := []string{"CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME"}
	for _, op := range adminOps {
		if firstWord == op {
			return firstWord
		}
	}

	return "UNKNOWN"
}

// detectMongoType 检测 MongoDB 查询类型
func (s *QueryService) detectMongoType(query string) string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return "FIND"
	}

	// 检测更新操作
	if strings.Contains(query, ".insertone(") {
		return "INSERTONE"
	}
	if strings.Contains(query, ".insertmany(") {
		return "INSERTMANY"
	}
	if strings.Contains(query, ".updateone(") {
		return "UPDATEONE"
	}
	if strings.Contains(query, ".updatemany(") {
		return "UPDATEMANY"
	}
	if strings.Contains(query, ".deleteone(") {
		return "DELETEONE"
	}
	if strings.Contains(query, ".deletemany(") {
		return "DELETEMANY"
	}

	// 检测查询操作
	if strings.Contains(query, ".findone(") {
		return "FINDONE"
	}
	if strings.Contains(query, ".find(") {
		return "FIND"
	}

	return "FIND" // 默认查询
}

// detectRedisType 检测 Redis 命令类型
func (s *QueryService) detectRedisType(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "READ"
	}

	upperQuery := strings.ToUpper(query)
	words := strings.Fields(upperQuery)
	if len(words) == 0 {
		return "READ"
	}

	command := words[0]

	// 只读命令
	readCommands := map[string]bool{
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
	if readCommands[command] {
		return "READ"
	}

	// 管理命令（需要 admin 权限）
	adminCommands := map[string]bool{
		"FLUSHDB": true, "FLUSHALL": true, "CONFIG": true, "SHUTDOWN": true,
		"SAVE": true, "BGSAVE": true, "BGREWRITEAOF": true,
		"REPLICAOF": true, "SLAVEOF": true, "CLUSTER": true,
		"MODULE": true, "ACL": true, "DEBUG": true, "MIGRATE": true,
		"RESTORE": true, "SYNC": true, "PSYNC": true,
	}
	if adminCommands[command] {
		return "ADMIN"
	}

	// 写命令（需要 write 权限）
	// 包括 SET, DEL, HSET, SADD, LPUSH, RPUSH, ZADD, EXPIRE 等
	return "WRITE"
}

// extractTableName 提取表名（简化版）
func (s *QueryService) extractTableName(sql string) string {
	// 简化实现，实际应该使用 SQL 解析器
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	
	// 简单的表名提取逻辑
	if strings.HasPrefix(upperSQL, "SELECT") {
		// SELECT * FROM table_name
		if idx := strings.Index(upperSQL, "FROM"); idx > 0 {
			parts := strings.Fields(upperSQL[idx+4:])
			if len(parts) > 0 {
				return strings.Trim(parts[0], "`\"'")
			}
		}
	} else if strings.HasPrefix(upperSQL, "INSERT") {
		// INSERT INTO table_name
		if idx := strings.Index(upperSQL, "INTO"); idx > 0 {
			parts := strings.Fields(upperSQL[idx+4:])
			if len(parts) > 0 {
				return strings.Trim(parts[0], "`\"'")
			}
		}
	} else if strings.HasPrefix(upperSQL, "UPDATE") {
		// UPDATE table_name
		parts := strings.Fields(upperSQL[6:])
		if len(parts) > 0 {
			return strings.Trim(parts[0], "`\"'")
		}
	} else if strings.HasPrefix(upperSQL, "DELETE") {
		// DELETE FROM table_name
		if idx := strings.Index(upperSQL, "FROM"); idx > 0 {
			parts := strings.Fields(upperSQL[idx+4:])
			if len(parts) > 0 {
				return strings.Trim(parts[0], "`\"'")
			}
		}
	}

	return ""
}

// logQueryAsync 异步记录查询日志
func (s *QueryService) logQueryAsync(log *model.QueryLog) {
	if err := s.queryLogRepo.Create(log); err != nil {
		logger.Errorf("Failed to log query: %v", err)
	}
}

func getStatus(result *QueryResult) string {
	if !result.Success {
		return "error"
	}
	return "success"
}

func formatResultPreview(result *QueryResult) string {
	if result.Error != "" {
		return result.Error
	}
	if len(result.Rows) > 0 {
		return fmt.Sprintf("返回 %d 行数据", len(result.Rows))
	}
	return "执行成功"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type ExecuteQueryRequest struct {
	InstanceID   uint   `json:"instanceId" binding:"required"`
	DatabaseName string `json:"databaseName" binding:"required"`
	Query        string `json:"query" binding:"required"`
}

type QueryResult struct {
	Success      bool                   `json:"success"`
	Columns      []string               `json:"columns,omitempty"`
	Rows         [][]interface{}        `json:"rows,omitempty"`
	Documents    []map[string]interface{} `json:"documents,omitempty"`
	RedisResult  interface{}            `json:"redisResult,omitempty"`
	AffectedRows int64                  `json:"affectedRows"`
	ResultCount  int                    `json:"resultCount"`
	ExecutionTime int64                 `json:"executionTimeMs"`
	Error        string                 `json:"error,omitempty"`
}
