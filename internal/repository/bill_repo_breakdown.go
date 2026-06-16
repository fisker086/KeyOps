package repository

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
)

func (r *billRepository) GetBreakdownByTags(vendor, month string) (map[string]map[string]decimal.Decimal, error) {
	if month == "" {
		return nil, fmt.Errorf("month is required for tag breakdown")
	}
	var records []struct {
		Tags          string          `gorm:"column:tags"`
		ConsumeAmount decimal.Decimal `gorm:"column:consume_amount"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Select("tags, SUM(consume_amount) AS consume_amount").
		Where("cycle = ?", month).
		Where("tags IS NOT NULL AND tags != ''").
		Group("tags").
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	tagAmounts := make(map[string]map[string]decimal.Decimal)
	for _, record := range records {
		var tags map[string]string
		if record.Tags != "" {
			json.Unmarshal([]byte(record.Tags), &tags)
		}
		if tags == nil {
			tags = map[string]string{"_none_": "_none_"}
		}
		for tagKey, tagVal := range tags {
			if _, ok := tagAmounts[tagKey]; !ok {
				tagAmounts[tagKey] = make(map[string]decimal.Decimal)
			}
			tagAmounts[tagKey][tagVal] = tagAmounts[tagKey][tagVal].Add(record.ConsumeAmount)
		}
	}

	return tagAmounts, nil
}

func (r *billRepository) GetBreakdownByAccounts(vendor, month string) (map[string]decimal.Decimal, error) {
	query := r.db.Model(&model.BillRecord{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if month != "" {
		query = query.Where("cycle = ?", month)
	}
	query = query.Select("COALESCE(account_id, 'unknown') as account_id, SUM(consume_amount) as amount")
	query = query.Group("COALESCE(account_id, 'unknown')")

	var results []struct {
		AccountID string          `gorm:"column:account_id"`
		Amount    decimal.Decimal `gorm:"column:amount"`
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	amounts := make(map[string]decimal.Decimal)
	for _, rec := range results {
		amounts[rec.AccountID] = rec.Amount
	}

	return amounts, nil
}

func (r *billRepository) GetBreakdownByRegion(vendor, month string) (map[string]decimal.Decimal, error) {
	query := r.db.Model(&model.BillRecord{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if month != "" {
		query = query.Where("cycle = ?", month)
	}
	query = query.Select("COALESCE(region, 'unknown') as region, SUM(consume_amount) as amount")
	query = query.Group("COALESCE(region, 'unknown')")

	var results []struct {
		Region string          `gorm:"column:region"`
		Amount decimal.Decimal `gorm:"column:amount"`
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	amounts := make(map[string]decimal.Decimal)
	for _, rec := range results {
		amounts[rec.Region] = rec.Amount
	}

	return amounts, nil
}

func (r *billRepository) GetCostByService(month string) (map[string]float64, error) {
	var results []struct {
		Key      string          `gorm:"column:key"`
		Currency string          `gorm:"column:currency"`
		Cost     decimal.Decimal `gorm:"column:cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("COALESCE(service_code, resource_type, 'unknown') as `key`, CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END as currency, SUM(consume_amount) as cost").
		Group("COALESCE(service_code, resource_type, 'unknown')").Group("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END").
		Order("cost DESC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	costByService := make(map[string]float64, len(results))
	for _, rec := range results {
		c, _ := rec.Cost.Float64()
		costByService[rec.Key] += c
	}
	return costByService, nil
}

func (r *billRepository) GetCostByRegion(month string) (map[string]float64, error) {
	var results []struct {
		Key      string          `gorm:"column:key"`
		Currency string          `gorm:"column:currency"`
		Cost     decimal.Decimal `gorm:"column:cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("COALESCE(region, 'unknown') as `key`, CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END as currency, SUM(consume_amount) as cost").
		Group("COALESCE(region, 'unknown')").Group("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END").
		Order("cost DESC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	costByRegion := make(map[string]float64, len(results))
	for _, rec := range results {
		c, _ := rec.Cost.Float64()
		costByRegion[rec.Key] += c
	}
	return costByRegion, nil
}

func (r *billRepository) GetCostByCloudAccount(cloudAccountID uint, month string) (float64, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.BillRecord{}).
		Where("cloud_account_id = ?", cloudAccountID).
		Where("cycle = ?", month).
		Select("COALESCE(SUM(consume_amount), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	f, _ := total.Float64()
	return f, nil
}

func (r *billRepository) GetCostByCloudAccountYear(cloudAccountID uint, year string) (float64, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.BillRecord{}).
		Where("cloud_account_id = ?", cloudAccountID).
		Where("cycle >= ? AND cycle <= ?", year+"-01", year+"-12").
		Select("COALESCE(SUM(consume_amount), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	f, _ := total.Float64()
	return f, nil
}

func (r *billRepository) GetResourceCountByCloudAccount(cloudAccountID uint) (int, error) {
	var count int64
	err := r.db.Model(&model.BillResource{}).
		Where("cloud_account_id = ?", cloudAccountID).
		Count(&count).Error
	return int(count), err
}

type cloudAccountAmountRow struct {
	CloudAccountID uint            `gorm:"column:cloud_account_id"`
	Total          decimal.Decimal `gorm:"column:total"`
}

type cloudAccountCountRow struct {
	CloudAccountID uint  `gorm:"column:cloud_account_id"`
	Count          int64 `gorm:"column:count"`
}

func (r *billRepository) GetCostByCloudAccountsYear(accountIDs []uint, year string) (map[uint]float64, error) {
	out := make(map[uint]float64, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	var rows []cloudAccountAmountRow
	err := r.db.Model(&model.BillRecord{}).
		Where("cloud_account_id IN ?", accountIDs).
		Where("cycle >= ? AND cycle <= ?", year+"-01", year+"-12").
		Select("cloud_account_id, COALESCE(SUM(consume_amount), 0) AS total").
		Group("cloud_account_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		f, _ := row.Total.Float64()
		out[row.CloudAccountID] = f
	}
	return out, nil
}

func (r *billRepository) GetCostByCloudAccountsMonth(accountIDs []uint, month string) (map[uint]float64, error) {
	out := make(map[uint]float64, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	var rows []cloudAccountAmountRow
	err := r.db.Model(&model.BillRecord{}).
		Where("cloud_account_id IN ?", accountIDs).
		Where("cycle = ?", month).
		Select("cloud_account_id, COALESCE(SUM(consume_amount), 0) AS total").
		Group("cloud_account_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		f, _ := row.Total.Float64()
		out[row.CloudAccountID] = f
	}
	return out, nil
}

func (r *billRepository) GetResourceCountByCloudAccounts(accountIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	var rows []cloudAccountCountRow
	err := r.db.Model(&model.BillResource{}).
		Where("cloud_account_id IN ?", accountIDs).
		Select("cloud_account_id, COUNT(*) AS count").
		Group("cloud_account_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CloudAccountID] = int(row.Count)
	}
	return out, nil
}

func billRecordExpenseGroupExpr(groupBy string) (selectExpr, groupByExpr string) {
	switch groupBy {
	case "service_code":
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(service_code), ''), 'unknown'))"
		return e, e
	case "cloud_type":
		e := "COALESCE(NULLIF(TRIM(ca.cloud_type), ''), NULLIF(TRIM(bill_records.vendor), ''), 'unknown')"
		return e, e
	case "region":
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(region), ''), 'unknown'))"
		return e, e
	case "account":
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(bill_records.account_id), ''), 'unknown'))"
		return e, e
	default:
		name := "COALESCE(NULLIF(TRIM(service_type), ''), NULLIF(TRIM(service_code), ''), NULLIF(TRIM(resource_type), ''), 'unknown')"
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', " + name + ")"
		return e, e
	}
}

func billRecordCostColumn(costType string) string {
	if costType == "effective" {
		return "COALESCE(effective_cost, consume_amount)"
	}
	return "consume_amount"
}

func (r *billRepository) GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword, costType string) (*BreakdownResult, error) {
	if costType != "effective" {
		costType = "unblended"
	}

	var dateColumn string
	if granularity == "daily" {
		dateColumn = "usage_date"
	} else {
		dateColumn = "cycle"
	}

	query := r.db.Model(&model.BillRecord{})

	if granularity == "daily" {
		startStr := startDate.Format("2006-01-02")
		endStr := endDate.Format("2006-01-02")
		query = query.Where("usage_date >= ? AND usage_date <= ?", startStr, endStr)
	} else {
		startStr := startDate.Format("2006-01")
		endStr := endDate.Format("2006-01")
		query = query.Where("cycle >= ? AND cycle <= ?", startStr, endStr)
	}

	if groupBy == "cloud_type" {
		query = query.Joins("LEFT JOIN bill_cloud_accounts ca ON ca.id = bill_records.cloud_account_id")
	}

	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if serviceCode != "" {
		query = query.Where("service_code = ?", serviceCode)
	}
	if keyword != "" {
		query = query.Where("resource_name LIKE ? OR instance_id LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var results []struct {
		Date          string
		ResourceGroup string `gorm:"column:resource_group"`
		Amount        float64
	}

	dateExpr := r.dateBucketExpr(dateColumn, granularity)
	selG, grpG := billRecordExpenseGroupExpr(groupBy)
	costCol := billRecordCostColumn(costType)
	selectSQL := fmt.Sprintf(
		"%s AS date, %s AS resource_group, SUM(%s) AS amount",
		dateExpr,
		selG,
		costCol,
	)
	groupSQL := fmt.Sprintf("%s, %s", dateExpr, grpG)

	err := query.Select(selectSQL).Group(groupSQL).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	breakdown := make(map[string]map[string]float64)
	totals := make(map[string]float64)

	for _, rec := range results {
		if breakdown[rec.Date] == nil {
			breakdown[rec.Date] = make(map[string]float64)
		}
		breakdown[rec.Date][rec.ResourceGroup] = rec.Amount
		totals[rec.ResourceGroup] += rec.Amount
	}

	return &BreakdownResult{
		Breakdown:   breakdown,
		Totals:      totals,
		Granularity: granularity,
		GroupBy:     groupBy,
		CostType:    costType,
	}, nil
}

func (r *billRepository) GetDailyCostTrend(startDate, endDate time.Time) ([]map[string]interface{}, error) {
	startStr := startDate.Format("2006-01")
	endStr := endDate.Format("2006-01")

	var results []struct {
		Date     string
		Currency string
		Cost     decimal.Decimal
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle >= ? AND cycle <= ?", startStr, endStr).
		Select("cycle as date, CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END as currency, SUM(consume_amount) as cost").
		Group("cycle").Group("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END").
		Order("date").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	dateMap := make(map[string]float64)
	for _, rec := range results {
		cost, _ := rec.Cost.Float64()
		dateMap[rec.Date] += cost
	}

	trend := make([]map[string]interface{}, 0, len(dateMap))
	for date, cost := range dateMap {
		trend = append(trend, map[string]interface{}{
			"date": date,
			"cost": cost,
		})
	}
	sort.Slice(trend, func(i, j int) bool {
		return trend[i]["date"].(string) < trend[j]["date"].(string)
	})
	return trend, nil
}
