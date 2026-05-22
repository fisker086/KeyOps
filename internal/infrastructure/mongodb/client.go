package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	RawExpensesCollection = "raw_expenses"
	DatabaseName          = "zjump_bill"
)

type Client struct {
	*mongo.Client
	db *mongo.Database
}

func NewClient(uri string) (*Client, error) {
	return NewClientWithDatabase(uri, DatabaseName)
}

func NewClientWithDatabase(uri, databaseName string) (*Client, error) {
	if databaseName == "" {
		databaseName = DatabaseName
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}

	return &Client{
		Client: client,
		db:     client.Database(databaseName),
	}, nil
}

func (c *Client) RawExpenses() *mongo.Collection {
	return c.db.Collection(RawExpensesCollection)
}

func (c *Client) InitIndexes(ctx context.Context) error {
	coll := c.RawExpenses()
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "cloud_account_id", Value: 1},
				{Key: "report_identity", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
	})
	return err
}

func (c *Client) Close(ctx context.Context) error {
	return c.Client.Disconnect(ctx)
}
