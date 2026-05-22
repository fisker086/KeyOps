package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Connect 建立 MongoDB 客户端并 Ping（不绑定账单库；堡垒机等场景使用同一 URI 不同 Database）。
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("mongo uri is empty")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}
	return client, nil
}
