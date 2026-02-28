package dms

import (
	"context"
	"fmt"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/crypto"
)

type QueryExecutor interface {
	TestConnection(ctx context.Context) error
	GetDatabases(ctx context.Context) ([]string, error)
	GetTables(ctx context.Context, databaseName string) ([]string, error)
	GetColumns(ctx context.Context, databaseName, tableName string) ([]ColumnInfo, error)
	ExecuteQuery(ctx context.Context, databaseName, query string, limit int) (*QueryResult, error)
	ExecuteUpdate(ctx context.Context, databaseName, query string) (*QueryResult, error)
	Close() error
}

type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"defaultValue"`
	Comment      string `json:"comment"`
}

func NewExecutor(instance *model.DBInstance, crypto *crypto.Crypto) (QueryExecutor, error) {
	switch instance.DBType {
	case "mysql":
		return NewMySQLExecutor(instance, crypto)
	case "postgresql":
		return NewPostgreSQLExecutor(instance, crypto)
	case "mongodb":
		return NewMongoDBExecutor(instance, crypto)
	case "redis":
		return NewRedisExecutor(instance, crypto)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", instance.DBType)
	}
}
