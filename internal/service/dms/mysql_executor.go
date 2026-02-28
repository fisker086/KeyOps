package dms

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/crypto"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLExecutor struct {
	instance *model.DBInstance
	crypto   *crypto.Crypto
	db       *sql.DB
	password string
}

func NewMySQLExecutor(instance *model.DBInstance, crypto *crypto.Crypto) (*MySQLExecutor, error) {
	password, err := crypto.Decrypt(instance.Password)
	if err != nil {
		return nil, fmt.Errorf("解密密码失败: %w", err)
	}

	return &MySQLExecutor{
		instance: instance,
		crypto:   crypto,
		password: password,
	}, nil
}

func (e *MySQLExecutor) getConnection(dbName string) (*sql.DB, error) {
	if e.db != nil {
		return e.db, nil
	}

	var dsn string
	if dbName == "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local",
			e.instance.Username, e.password, e.instance.Host, e.instance.Port, e.instance.Charset)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			e.instance.Username, e.password, e.instance.Host, e.instance.Port,
			dbName, e.instance.Charset)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	e.db = db
	return db, nil
}

func (e *MySQLExecutor) TestConnection(ctx context.Context) error {
	db, err := e.getConnection("")
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}

func (e *MySQLExecutor) GetDatabases(ctx context.Context) ([]string, error) {
	db, err := e.getConnection("information_schema")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var databases []string
	excludeDbs := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("扫描失败: %w", err)
		}
		if !excludeDbs[dbName] {
			databases = append(databases, dbName)
		}
	}

	return databases, nil
}

func (e *MySQLExecutor) GetTables(ctx context.Context, databaseName string) ([]string, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW TABLES FROM `%s`", databaseName)
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

func (e *MySQLExecutor) GetColumns(ctx context.Context, databaseName, tableName string) ([]ColumnInfo, error) {
	db, err := e.getConnection(databaseName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := db.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.DefaultValue, &col.Comment); err != nil {
			return nil, fmt.Errorf("扫描失败: %w", err)
		}
		col.Nullable = nullable == "YES"
		columns = append(columns, col)
	}

	return columns, nil
}

func (e *MySQLExecutor) ExecuteQuery(ctx context.Context, databaseName, query string, limit int) (*QueryResult, error) {
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

func (e *MySQLExecutor) ExecuteUpdate(ctx context.Context, databaseName, query string) (*QueryResult, error) {
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

func (e *MySQLExecutor) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
