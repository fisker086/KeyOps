package bill

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// -----------------------------------------------------------------------------------
// MongoDB 查询方法（AWS 账单从 MongoDB 查询，减轻 MySQL 压力）
// -----------------------------------------------------------------------------------

// QueryMongoRaw 从 MongoDB 查询 AWS CUR 原始数据
// filters: 可选过滤条件，如 cloud_account_id、lineItem/ProductCode 等
// startAfter/endBefore: 可选时间范围（lineItem/UsageStartDate）
func (s *BillService) QueryMongoRaw(filters map[string]interface{}, startAfter, endBefore *time.Time, limit int) ([]map[string]interface{}, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized")
	}

	ctx := context.Background()
	query := bson.M{}
	for k, v := range filters {
		query[k] = v
	}
	if startAfter != nil {
		query["lineItem/UsageStartDate"] = bson.M{"$gte": startAfter.Format(time.RFC3339)}
	}
	if endBefore != nil {
		if _, ok := query["lineItem/UsageStartDate"]; !ok {
			query["lineItem/UsageStartDate"] = bson.M{}
		}
		query["lineItem/UsageStartDate"].(bson.M)["$lt"] = endBefore.Format(time.RFC3339)
	}

	findOptions := options.Find()
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	findOptions.SetSort(bson.D{{Key: "lineItem/UsageStartDate", Value: -1}})

	cur, err := s.mongoColl.Find(ctx, query, findOptions)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []map[string]interface{}
	if err := cur.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// AggregateMongoByField 按字段聚合费用（类似 MySQL 的 GROUP BY）
// fieldPath: MongoDB 字段路径，如 "lineItem/ProductCode"、"product/region"
// filters: 额外过滤条件
func (s *BillService) AggregateMongoByField(fieldPath string, filters map[string]interface{}, startAfter, endBefore *time.Time) (map[string]float64, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized")
	}

	ctx := context.Background()
	pipeline := mongo.Pipeline{}

	// match 阶段
	match := bson.D{{Key: "$match", Value: bson.M{}}}
	for k, v := range filters {
		match[0].Value.(bson.M)[k] = v
	}
	if startAfter != nil || endBefore != nil {
		dateFilter := bson.M{}
		if startAfter != nil {
			dateFilter["$gte"] = startAfter.Format(time.RFC3339)
		}
		if endBefore != nil {
			dateFilter["$lt"] = endBefore.Format(time.RFC3339)
		}
		match[0].Value.(bson.M)["lineItem/UsageStartDate"] = dateFilter
	}
	pipeline = append(pipeline, match)

	// group 阶段
	group := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$" + fieldPath},
			{Key: "totalCost", Value: bson.D{{Key: "$sum", Value: "$lineItem/BlendedCost"}}},
		}},
	}
	pipeline = append(pipeline, group)

	cur, err := s.mongoColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	results := make(map[string]float64)
	for cur.Next(ctx) {
		var doc struct {
			ID        interface{} `bson:"_id"`
			TotalCost float64     `bson:"totalCost"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		key := fmt.Sprintf("%v", doc.ID)
		if key == "" {
			key = "unknown"
		}
		results[key] = doc.TotalCost
	}
	return results, nil
}
