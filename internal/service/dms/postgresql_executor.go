package dms

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/crypto"
	_ "github.com/lib/pq"
)

type PostgreSQLExecutor struct {
	instance *model.DBInstance
	crypto   *crypto.Crypto
	db       *sql.DB
	password string
}

func NewPostgreSQLExecutor(instance *model.DBInstance, crypto *crypto.Crypto) (*PostgreSQLExecutor, error) {
	password, err := crypto.Decrypt(instance.Password)
	if err != nil {
		return nil, fmt.Errorf("解密密码失败: %w", err)
	}

	return &PostgreSQLExecutor{
		instance: instance,
		crypto:   crypto,
		password: password,
	}, nil
}

func (e *PostgreSQLExecutor) getConnection(dbName string) (*sql.DB, error) {
	if e.db != nil {
		return e.db, nil
	}

	if dbName == "" {
		dbName = "postgres"
	}

	// 使用 URL 格式的连接字符串，更安全地处理特殊字符
	// PostgreSQL 连接字符串格式: postgres://user:password@host:port/dbname?sslmode=disable
	var dsn string
	if e.instance.Username != "" && e.password != "" {
		// 使用 url.UserPassword 来正确处理用户名和密码中的特殊字符
		userInfo := url.UserPassword(e.instance.Username, e.password)
		// 对数据库名进行路径编码（PathEscape 更适合 URL 路径部分）
		encodedDbName := url.PathEscape(dbName)
		dsn = fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=disable",
			userInfo.String(), e.instance.Host, e.instance.Port, encodedDbName)
	} else {
		// 无认证连接
		encodedDbName := url.PathEscape(dbName)
		dsn = fmt.Sprintf("postgres://%s:%d/%s?sslmode=disable",
			e.instance.Host, e.instance.Port, encodedDbName)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	e.db = db
	return db, nil
}

func (e *PostgreSQLExecutor) TestConnection(ctx context.Context) error {
	db, err := e.getConnection("postgres")
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		// 提供更详细的错误信息
		if strings.Contains(err.Error(), "password authentication failed") || strings.Contains(err.Error(), "authentication failed") {
			return fmt.Errorf("认证失败: 请检查用户名和密码是否正确。用户名: %s。错误详情: %v", e.instance.Username, err)
		}
		return fmt.Errorf("连接测试失败: %w", err)
	}
	return nil
}

func (e *PostgreSQLExecutor) GetDatabases(ctx context.Context) ([]string, error) {
	db, err := e.getConnection("postgres")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := "SELECT datname FROM pg_database WHERE datistemplate = false"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("扫描失败: %w", err)
		}
		databases = append(databases, dbName)
	}

	return databases, nil
}

func (e *PostgreSQLExecutor) GetTables(ctx context.Context, databaseName string) ([]string, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("扫描失败: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

func (e *PostgreSQLExecutor) GetColumns(ctx context.Context, databaseName, tableName string) ([]ColumnInfo, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default,
			COALESCE(col_description((table_schema||'.'||table_name)::regclass, ordinal_position), '') as comment
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultValue, &col.Comment); err != nil {
			return nil, fmt.Errorf("扫描失败: %w", err)
		}
		col.Nullable = nullable == "YES"
		if defaultValue.Valid {
			col.DefaultValue = defaultValue.String
		}
		columns = append(columns, col)
	}

	return columns, nil
}

func (e *PostgreSQLExecutor) ExecuteQuery(ctx context.Context, databaseName, query string, limit int) (*QueryResult, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}
	defer db.Close()

	if limit > 0 && !strings.Contains(strings.ToUpper(query), "LIMIT") {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	// 根据SQL大小动态设置超时时间
	// 对于大SQL（>1MB），增加超时时间到5分钟
	sqlSize := len(query)
	timeout := 30 * time.Second
	if sqlSize > 1024*1024 { // 大于1MB
		timeout = 300 * time.Second // 5分钟
	} else if sqlSize > 100*1024 { // 大于100KB
		timeout = 120 * time.Second // 2分钟
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	var resultRows [][]interface{}
	var resultCount int
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		rowData := make([]interface{}, len(values))
		for i, v := range values {
			if v == nil {
				rowData[i] = nil
			} else {
				switch val := v.(type) {
				case []byte:
					rowData[i] = string(val)
				case time.Time:
					rowData[i] = val.Format("2006-01-02 15:04:05")
				default:
					rowData[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		resultRows = append(resultRows, rowData)
		resultCount++
	}

	return &QueryResult{
		Success:      true,
		Columns:      columns,
		Rows:         resultRows,
		ResultCount:  resultCount,
		AffectedRows: int64(resultCount),
	}, nil
}

func (e *PostgreSQLExecutor) ExecuteUpdate(ctx context.Context, databaseName, query string) (*QueryResult, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}
	defer db.Close()

	// 根据SQL大小动态设置超时时间
	sqlSize := len(query)
	timeout := 30 * time.Second
	if sqlSize > 1024*1024 { // 大于1MB
		timeout = 300 * time.Second // 5分钟
	} else if sqlSize > 100*1024 { // 大于100KB
		timeout = 120 * time.Second // 2分钟
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	affectedRows, _ := result.RowsAffected()
	return &QueryResult{
		Success:      true,
		AffectedRows: affectedRows,
		ResultCount:  0,
	}, nil
}

func (e *PostgreSQLExecutor) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
