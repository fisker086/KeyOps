package bill

import (
	"context"
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetBreakdownByTags 按标签分解费用
func (s *BillService) GetBreakdownByTags(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByTags(vendor, month)
}

// GetBreakdownByAccounts 按云账户分解费用
func (s *BillService) GetBreakdownByAccounts(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByAccounts(vendor, month)
}

// GetBreakdownByRegion 按区域分解费用
func (s *BillService) GetBreakdownByRegion(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByRegion(vendor, month)
}

// GetBreakdownByService 按服务分解费用
func (s *BillService) GetBreakdownByService(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByService(vendor, month)
}

// GetRegionExpenses 按区域聚合费用
func (s *BillService) GetRegionExpenses(startDate, endDate time.Time) (interface{}, error) {
	return s.repo.GetRegionExpenses(startDate, endDate)
}

// GetTrafficExpenses 按流量聚合费用（目前使用 region 聚合作为占位，后续可接 CUR 流量字段）
func (s *BillService) GetTrafficExpenses(startDate, endDate time.Time, resourceID string) (interface{}, error) {
	return s.repo.GetTrafficExpenses(startDate, endDate, resourceID)
}

// GetExpensesBreakdown 获取费用分解
func (s *BillService) GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword string) (*repository.BreakdownResult, error) {
	if granularity == "daily" {
		return s.getMongoDailyBreakdown(startDate, endDate, groupBy, vendor)
	}
	return s.repo.GetExpensesBreakdown(startDate, endDate, granularity, groupBy, vendor, serviceCode, keyword)
}

// getMongoDailyBreakdown 从 MongoDB 按日聚合费用（用于日粒度图表）
func (s *BillService) getMongoDailyBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*repository.BreakdownResult, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized, daily breakdown unavailable")
	}

	ctx := context.Background()
	pipeline := mongo.Pipeline{}

	// match 阶段
	match := bson.D{{Key: "$match", Value: bson.M{}}}
	if vendor != "" {
		match[0].Value.(bson.M)["lineItem/ProductCode"] = vendor
	}
	dateFilter := bson.M{}
	if !startDate.IsZero() {
		dateFilter["$gte"] = startDate.Format("2006-01-02")
	}
	if !endDate.IsZero() {
		dateFilter["$lte"] = endDate.Format("2006-01-02")
	}
	if len(dateFilter) > 0 {
		match[0].Value.(bson.M)["lineItem/UsageStartDate"] = dateFilter
	}
	pipeline = append(pipeline, match)

	// group 阶段：按日期 + groupBy 字段聚合 cost
	groupField := "$lineItem/ProductCode"
	dateFormat := "%Y-%m-%d"
	switch groupBy {
	case "region":
		groupField = "$product/region"
	case "service_name", "service_code":
		groupField = "$lineItem/ProductCode"
	case "cloud_type":
		groupField = "$lineItem/ProductCode"
	}

	groupID := bson.D{
		{Key: "date", Value: bson.D{{Key: "$dateToString", Value: bson.D{
			{Key: "format", Value: dateFormat},
			{Key: "date", Value: "$lineItem/UsageStartDate"},
		}}}},
		{Key: "group", Value: groupField},
	}
	groupStage := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "totalCost", Value: bson.D{{Key: "$sum", Value: "$lineItem/BlendedCost"}}},
		}},
	}
	pipeline = append(pipeline, groupStage)

	// sort by date
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "_id.date", Value: 1}}},
	})

	cur, err := s.mongoColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	breakdown := make(map[string]map[string]float64)
	totals := make(map[string]float64)

	type mongoDoc struct {
		ID        struct {
			Date  string `bson:"date"`
			Group string `bson:"group"`
		} `bson:"_id"`
		TotalCost float64 `bson:"totalCost"`
	}

	for cur.Next(ctx) {
		var doc mongoDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		date := doc.ID.Date
		group := doc.ID.Group
		if group == "" {
			group = "unknown"
		}
		if breakdown[date] == nil {
			breakdown[date] = make(map[string]float64)
		}
		breakdown[date][group] += doc.TotalCost
		totals[group] += doc.TotalCost
	}

	return &repository.BreakdownResult{
		Breakdown:   breakdown,
		Totals:      totals,
		Granularity: "daily",
		GroupBy:     groupBy,
	}, nil
}

// GetResourceCountBreakdown 获取资源数量分解
func (s *BillService) GetResourceCountBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*repository.BreakdownResult, error) {
	return s.repo.GetResourceCountBreakdown(startDate, endDate, groupBy, vendor)
}
