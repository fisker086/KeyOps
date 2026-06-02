package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BreakdownResult struct {
	Breakdown   map[string]map[string]float64 `json:"breakdown"`
	Totals      map[string]float64            `json:"totals"`
	Granularity string                        `json:"granularity"`
	GroupBy     string                        `json:"group_by"`
}

type BillRepository interface {
	GetRecords(vendor, month, resourceCode, serviceCode string, page, pageSize int) (total int64, records []model.BillRecord, err error)
	GetSummary(vendor, month string) (summary model.BillSummary, details []model.BillSummaryDetail, err error)
	GetSummaryCount(month string) (map[string]interface{}, error)
	GetSummaryTrend(vendor, year string) (map[string][]model.BillSummary, error)
	GetSummaryTrendMonth(year string) ([]string, error)
	GetSummaryByCloud(startDate, endDate time.Time) (map[string]decimal.Decimal, error)
	CreateRecord(record *model.BillRecord) error
	ReplaceBillingRecordsForAccount(cloudAccountID uint, cycle string, records []model.BillRecord) error
	UpsertBillResources(resources []model.BillResource) error
	RebuildSummary(vendor, cycle string) error
	ListBillResources(vendor string, page, pageSize int) (int64, []model.BillResource, error)
	GetRecordsByCloudAccount(cloudAccountID uint, month string) ([]model.BillRecord, error)
	GetBreakdownByTags(vendor, month string) (map[string]map[string]decimal.Decimal, error)
	GetBreakdownByAccounts(vendor, month string) (map[string]decimal.Decimal, error)
	GetBreakdownByRegion(vendor, month string) (map[string]decimal.Decimal, error)
	GetCostByService(month string) (map[string]float64, error)
	GetCostByRegion(month string) (map[string]float64, error)
	GetBreakdownByService(vendor, month string) (map[string]decimal.Decimal, error)
	GetRegionExpenses(startDate, endDate time.Time) (map[string]interface{}, error)
	GetTrafficExpenses(startDate, endDate time.Time, resourceID string) (map[string]interface{}, error)
	GetCostByCloudAccount(cloudAccountID uint, month string) (float64, error)
	GetCostByCloudAccountYear(cloudAccountID uint, year string) (float64, error)
	GetResourceCountByCloudAccount(cloudAccountID uint) (int, error)
	GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword string) (*BreakdownResult, error)
	GetResourceCountBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*BreakdownResult, error)
	GetTotalCostByMonth(month string) (float64, error)
	GetTopResources(month string, limit int) ([]map[string]interface{}, error)
	GetCostByVendor(month string) (map[string]VendorCost, error)
	GetDailyCostTrend(startDate, endDate time.Time) ([]map[string]interface{}, error)
	GetVMRecords(vendor, month string) ([]model.BillRecord, error)
	GetVMRecordsByGroup(vendor, month, groupBy string) (map[string]float64, error)
	GetIdleResources() ([]IdleResource, error)
	GetLargeResources() ([]IdleResource, error)
}

type billRepository struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepository{db: db}
}

func (r *billRepository) dateBucketExpr(column, granularity string) string {
	if r.db != nil && r.db.Dialector.Name() == "postgres" {
		if granularity == "daily" {
			return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD')", column)
		}
		return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM')", column)
	}

	dateFormat := "%Y-%m"
	if granularity == "daily" {
		dateFormat = "%Y-%m-%d"
	}
	return fmt.Sprintf("DATE_FORMAT(%s, '%s')", column, dateFormat)
}

func (r *billRepository) GetRecords(vendor, month, resourceCode, serviceCode string, page, pageSize int) (total int64, records []model.BillRecord, err error) {
	query := r.db.Model(&model.BillRecord{}).
		Where("vendor = ? AND cycle = ?", vendor, month)

	if resourceCode != "" {
		query = query.Where("resource_code = ?", resourceCode)
	}
	if serviceCode != "" {
		query = query.Where("service_code = ?", serviceCode)
	}

	err = query.Count(&total).Error
	if err != nil {
		return
	}

	if total == 0 {
		return 0, []model.BillRecord{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Find(&records).Error
	return
}

func (r *billRepository) GetSummary(vendor, month string) (summary model.BillSummary, details []model.BillSummaryDetail, err error) {
	err = r.db.Model(&model.BillSummary{}).
		Where("vendor = ? AND cycle = ?", vendor, month).
		First(&summary).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BillSummary{
				Vendor:        vendor,
				Cycle:         month,
				ConsumeAmount: decimal.Zero,
			}, []model.BillSummaryDetail{}, nil
		}
		return summary, details, err
	}

	err = r.db.Model(&model.BillSummaryDetail{}).
		Where("summary_id = ?", summary.ID).
		Find(&details).Error
	return summary, details, err
}

func (r *billRepository) GetSummaryCount(month string) (map[string]interface{}, error) {
	var summaries []model.BillSummary
	err := r.db.Model(&model.BillSummary{}).
		Where("cycle = ?", month).
		Find(&summaries).Error
	if err != nil {
		return nil, err
	}

	count := decimal.NewFromInt(0)
	for _, item := range summaries {
		count = count.Add(item.ConsumeAmount)
	}

	var recordTotal int64
	_ = r.db.Model(&model.BillRecord{}).Where("cycle = ?", month).Count(&recordTotal).Error

	type vendorRecCount struct {
		Vendor string `gorm:"column:vendor"`
		Cnt    int64  `gorm:"column:cnt"`
	}
	var recByVendor []vendorRecCount
	_ = r.db.Model(&model.BillRecord{}).
		Select("vendor", "count(*) as cnt").
		Where("cycle = ?", month).
		Group("vendor").
		Scan(&recByVendor).Error
	recCountMap := make(map[string]int64, len(recByVendor))
	for _, row := range recByVendor {
		recCountMap[row.Vendor] = row.Cnt
	}

	result := map[string]interface{}{
		"count":                  count,
		"vendor":                 summaries,
		"record_total":           recordTotal,
		"record_count_by_vendor": recCountMap,
	}
	return result, nil
}

func (r *billRepository) GetSummaryTrend(vendor, year string) (map[string][]model.BillSummary, error) {
	var summaries []model.BillSummary

	query := r.db.Model(&model.BillSummary{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if year != "" {
		query = query.Where("cycle LIKE ?", year+"-%")
	}

	err := query.Find(&summaries).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string][]model.BillSummary)
	for _, item := range summaries {
		v := item.Vendor
		s := model.BillSummary{
			Vendor:        v,
			Cycle:         item.Cycle,
			ConsumeAmount: item.ConsumeAmount,
		}

		if _, ok := result[v]; ok {
			result[v] = append(result[v], s)
		} else {
			result[v] = []model.BillSummary{s}
		}
	}
	return result, nil
}

func (r *billRepository) GetSummaryTrendMonth(year string) ([]string, error) {
	var monthList []string
	query := r.db.Model(&model.BillSummary{}).Select("DISTINCT cycle")

	if year != "" {
		query = query.Where("cycle LIKE ?", year+"-%").Order("cycle ASC")
	} else {
		query = query.Order("cycle DESC").Limit(6)
	}

	err := query.Scan(&monthList).Error
	if err != nil {
		return nil, err
	}

	if year == "" && len(monthList) > 0 {
		for i, j := 0, len(monthList)-1; i < j; i, j = i+1, j-1 {
			monthList[i], monthList[j] = monthList[j], monthList[i]
		}
	}

	return monthList, nil
}

func billingCyclesBetween(startDate, endDate time.Time) []string {
	if startDate.After(endDate) {
		startDate, endDate = endDate, startDate
	}
	start := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	var cycles []string
	for d := start; !d.After(end); d = d.AddDate(0, 1, 0) {
		cycles = append(cycles, d.Format("2006-01"))
	}
	return cycles
}

func (r *billRepository) GetSummaryByCloud(startDate, endDate time.Time) (map[string]decimal.Decimal, error) {
	cycles := billingCyclesBetween(startDate, endDate)
	if len(cycles) == 0 {
		return map[string]decimal.Decimal{}, nil
	}

	type Result struct {
		Vendor string
		Total  decimal.Decimal `gorm:"column:total"`
	}

	var results []Result
	err := r.db.Model(&model.BillRecord{}).
		Select("vendor, SUM(consume_amount) as total").
		Where("cycle IN ?", cycles).
		Group("vendor").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	result := make(map[string]decimal.Decimal)
	for _, row := range results {
		result[row.Vendor] = row.Total
	}
	return result, nil
}

func (r *billRepository) CreateRecord(record *model.BillRecord) error {
	return r.db.Create(record).Error
}

func (r *billRepository) ReplaceBillingRecordsForAccount(cloudAccountID uint, cycle string, records []model.BillRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cloud_account_id = ? AND cycle = ?", cloudAccountID, cycle).Delete(&model.BillRecord{}).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(records, 1000).Error
	})
}

func (r *billRepository) UpsertBillResources(resources []model.BillResource) error {
	if len(resources) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, resource := range resources {
			var existing model.BillResource
			err := tx.Where("cloud_account_id = ? AND resource_id = ?", resource.CloudAccountID, resource.ResourceID).First(&existing).Error
			if err == nil {
				resource.ID = existing.ID
				resource.CreatedAt = existing.CreatedAt
				if !existing.FirstSeen.IsZero() {
					resource.FirstSeen = existing.FirstSeen
				}
				if resource.AccountID == "" {
					resource.AccountID = existing.AccountID
				}
				if strings.TrimSpace(resource.ResourceName) == "" && strings.TrimSpace(existing.ResourceName) != "" {
					en := strings.TrimSpace(existing.ResourceName)
					eid := strings.TrimSpace(existing.ResourceID)
					if en != eid {
						resource.ResourceName = existing.ResourceName
					}
				}
				if err := tx.Save(&resource).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Create(&resource).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *billRepository) RebuildSummary(vendor, cycle string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var total decimal.Decimal
		if err := tx.Model(&model.BillRecord{}).
			Where("vendor = ? AND cycle = ?", vendor, cycle).
			Select("COALESCE(SUM(consume_amount), 0)").
			Scan(&total).Error; err != nil {
			return err
		}

		var oldSummaries []model.BillSummary
		if err := tx.Where("vendor = ? AND cycle = ?", vendor, cycle).Find(&oldSummaries).Error; err != nil {
			return err
		}
		oldSummaryIDs := make([]uint, 0, len(oldSummaries))
		for _, summary := range oldSummaries {
			oldSummaryIDs = append(oldSummaryIDs, summary.ID)
		}
		if len(oldSummaryIDs) > 0 {
			if err := tx.Where("summary_id IN ?", oldSummaryIDs).Delete(&model.BillSummaryDetail{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("vendor = ? AND cycle = ?", vendor, cycle).Delete(&model.BillSummary{}).Error; err != nil {
			return err
		}

		summary := model.BillSummary{
			Vendor:        vendor,
			Cycle:         cycle,
			ConsumeAmount: total,
		}
		if err := tx.Create(&summary).Error; err != nil {
			return err
		}

		var rows []struct {
			ResourceType  string          `gorm:"column:resource_type"`
			ResourceCode  string          `gorm:"column:resource_code"`
			ServiceType   string          `gorm:"column:service_type"`
			ServiceCode   string          `gorm:"column:service_code"`
			ConsumeAmount decimal.Decimal `gorm:"column:consume_amount"`
		}
		if err := tx.Model(&model.BillRecord{}).
			Select("resource_type, resource_code, service_type, service_code, SUM(consume_amount) AS consume_amount").
			Where("vendor = ? AND cycle = ?", vendor, cycle).
			Group("resource_type, resource_code, service_type, service_code").
			Scan(&rows).Error; err != nil {
			return err
		}

		details := make([]model.BillSummaryDetail, 0, len(rows))
		for _, row := range rows {
			details = append(details, model.BillSummaryDetail{
				ResourceType:  row.ResourceType,
				ResourceCode:  row.ResourceCode,
				ServiceType:   row.ServiceType,
				ServiceCode:   row.ServiceCode,
				ConsumeAmount: row.ConsumeAmount,
				SummaryID:     summary.ID,
			})
		}
		if len(details) == 0 {
			return nil
		}
		return tx.CreateInBatches(details, 1000).Error
	})
}

func (r *billRepository) ListBillResources(vendor string, page, pageSize int) (int64, []model.BillResource, error) {
	var list []model.BillResource
	query := r.db.Model(&model.BillResource{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err := query.Order("id desc").Find(&list).Error
	return total, list, err
}

func (r *billRepository) GetRecordsByCloudAccount(cloudAccountID uint, month string) ([]model.BillRecord, error) {
	var records []model.BillRecord
	query := r.db.Where("cloud_account_id = ? OR extra LIKE ?", cloudAccountID, fmt.Sprintf("%%cloud_account_id:%d%%", cloudAccountID))
	if month != "" {
		query = query.Where("cycle = ?", month)
	}
	err := query.Find(&records).Error
	return records, err
}

func (r *billRepository) GetBreakdownByTags(vendor, month string) (map[string]map[string]decimal.Decimal, error) {
	query := r.db.Model(&model.BillRecord{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if month != "" {
		query = query.Where("cycle = ?", month)
	}
	query = query.Where("tags IS NOT NULL AND tags != ''")

	var records []model.BillRecord
	err := query.Find(&records).Error
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
		Select("COALESCE(service_code, resource_type, 'unknown') as `key`, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD') as currency, SUM(consume_amount) as cost").
		Group("COALESCE(service_code, resource_type, 'unknown')").Group("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')").
		Order("cost DESC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	costByService := make(map[string]float64, len(results))
	for _, rec := range results {
		c, _ := rec.Cost.Float64()
		if rec.Currency == "CNY" {
			c /= 7.2
		}
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
		Select("COALESCE(region, 'unknown') as `key`, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD') as currency, SUM(consume_amount) as cost").
		Group("COALESCE(region, 'unknown')").Group("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')").
		Order("cost DESC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	costByRegion := make(map[string]float64, len(results))
	for _, rec := range results {
		c, _ := rec.Cost.Float64()
		if rec.Currency == "CNY" {
			c /= 7.2
		}
		costByRegion[rec.Key] += c
	}
	return costByRegion, nil
}

func (r *billRepository) GetBreakdownByService(vendor, month string) (map[string]decimal.Decimal, error) {
	query := r.db.Model(&model.BillRecord{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if month != "" {
		query = query.Where("cycle = ?", month)
	}
	query = query.Select("COALESCE(service_code, resource_type, 'unknown') as service_code, SUM(consume_amount) as amount")
	query = query.Group("COALESCE(service_code, resource_type, 'unknown')")

	var results []struct {
		ServiceCode string          `gorm:"column:service_code"`
		Amount      decimal.Decimal `gorm:"column:amount"`
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	amounts := make(map[string]decimal.Decimal)
	for _, rec := range results {
		amounts[rec.ServiceCode] = rec.Amount
	}

	return amounts, nil
}

func (r *billRepository) GetRegionExpenses(startDate, endDate time.Time) (map[string]interface{}, error) {
	startStr := startDate.Format("2006-01")
	endStr := endDate.Format("2006-01")

	query := r.db.Model(&model.BillRecord{}).
		Where("cycle >= ? AND cycle <= ?", startStr, endStr)

	query = query.Select("COALESCE(region, 'unknown') as region, SUM(consume_amount) as amount")
	query = query.Group("COALESCE(region, 'unknown')")

	var results []struct {
		Region string          `gorm:"column:region"`
		Amount decimal.Decimal `gorm:"column:amount"`
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	expenses := make(map[string]interface{})
	for _, rec := range results {
		expenses[rec.Region] = rec.Amount.String()
	}

	return expenses, nil
}

func (r *billRepository) GetTrafficExpenses(startDate, endDate time.Time, resourceID string) (map[string]interface{}, error) {
	startStr := startDate.Format("2006-01")
	endStr := endDate.Format("2006-01")

	query := r.db.Model(&model.BillRecord{}).
		Where("cycle >= ? AND cycle <= ?", startStr, endStr)

	if resourceID != "" {
		query = query.Where("instance_id = ?", resourceID)
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

	expenses := make(map[string]interface{})
	for _, rec := range results {
		expenses[rec.Region] = rec.Amount.String()
	}

	return expenses, nil
}

func (r *billRepository) GetCostByCloudAccount(cloudAccountID uint, month string) (float64, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.BillRecord{}).
		Where("cloud_account_id = ? OR extra LIKE ?", cloudAccountID, "%cloud_account_id:"+strconv.FormatUint(uint64(cloudAccountID), 10)+"%").
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
		Where("cloud_account_id = ? OR extra LIKE ?", cloudAccountID, "%cloud_account_id:"+strconv.FormatUint(uint64(cloudAccountID), 10)+"%").
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

func billRecordExpenseGroupExpr(groupBy string) (selectExpr, groupByExpr string) {
	switch groupBy {
	case "service_code":
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(service_code), ''), 'unknown'))"
		return e, e
	case "cloud_type":
		e := "COALESCE(vendor, 'unknown')"
		return e, e
	case "region":
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(region), ''), 'unknown'))"
		return e, e
	default:
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(service_type), ''), NULLIF(TRIM(service_code), ''), 'unknown'))"
		return e, e
	}
}

func (r *billRepository) GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword string) (*BreakdownResult, error) {
	startStr := startDate.Format("2006-01")
	endStr := endDate.Format("2006-01")

	query := r.db.Model(&model.BillRecord{}).
		Where("cycle >= ? AND cycle <= ?", startStr, endStr)

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

	dateExpr := r.dateBucketExpr("cycle", "monthly")
	selG, grpG := billRecordExpenseGroupExpr(groupBy)
	selectSQL := fmt.Sprintf(
		"%s AS date, %s AS resource_group, SUM(consume_amount) AS amount",
		dateExpr,
		selG,
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
	}, nil
}

func resourceCountGroupExpr(groupBy string) (selectAndAlias, groupByExpr string) {
	switch groupBy {
	case "service_code", "service_name", "resource_type":
		e := "COALESCE(NULLIF(TRIM(br.resource_type), ''), 'unknown')"
		return e + " AS resource_group", e
	case "instance_type":
		e := "COALESCE(NULLIF(TRIM(br.instance_type), ''), 'unknown')"
		return e + " AS resource_group", e
	case "region":
		e := "COALESCE(NULLIF(TRIM(br.region), ''), 'unknown')"
		return e + " AS resource_group", e
	case "cloud_type":
		e := "COALESCE(NULLIF(TRIM(c.cloud_type), ''), NULLIF(TRIM(br.vendor), ''), 'unknown')"
		return e + " AS resource_group", e
	default:
		e := "COALESCE(NULLIF(TRIM(br.resource_type), ''), 'unknown')"
		return e + " AS resource_group", e
	}
}

func (r *billRepository) GetResourceCountBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*BreakdownResult, error) {
	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	var results []struct {
		ResourceGroup string `gorm:"column:resource_group"`
		Count         int64
	}

	var err error
	seenExpr := "date(COALESCE(br.first_seen, br.created_at))"
	if groupBy == "cloud_type" {
		sel, grp := resourceCountGroupExpr(groupBy)
		q := r.db.Table("bill_resources AS br").
			Select(sel+", COUNT(*) AS count").
			Joins("LEFT JOIN bill_cloud_accounts AS c ON c.id = br.cloud_account_id").
			Where(seenExpr+" >= ? AND "+seenExpr+" <= ?", startStr, endStr)
		if vendor != "" {
			q = q.Where("COALESCE(NULLIF(TRIM(c.cloud_type), ''), NULLIF(TRIM(br.vendor), '')) = ?", vendor)
		}
		err = q.Group(grp).Scan(&results).Error
	} else {
		q := r.db.Table("bill_resources AS br").
			Where(seenExpr+" >= ? AND "+seenExpr+" <= ?", startStr, endStr)
		if vendor != "" {
			q = q.Joins("LEFT JOIN bill_cloud_accounts AS c ON c.id = br.cloud_account_id").
				Where("COALESCE(NULLIF(TRIM(c.cloud_type), ''), NULLIF(TRIM(br.vendor), '')) = ?", vendor)
		}
		sel, grp := resourceCountGroupExpr(groupBy)
		err = q.Select(sel + ", COUNT(*) AS count").Group(grp).Scan(&results).Error
	}
	if err != nil {
		return nil, err
	}

	breakdown := make(map[string]map[string]float64)
	totals := make(map[string]float64)
	breakdown["total"] = make(map[string]float64)

	for _, rec := range results {
		breakdown["total"][rec.ResourceGroup] = float64(rec.Count)
		totals[rec.ResourceGroup] = float64(rec.Count)
	}

	return &BreakdownResult{
		Breakdown:   breakdown,
		Totals:      totals,
		Granularity: "total",
		GroupBy:     groupBy,
	}, nil
}

func (r *billRepository) GetTotalCostByMonth(month string) (float64, error) {
	var results []struct {
		Currency string          `gorm:"column:currency"`
		Amount   decimal.Decimal `gorm:"column:amount"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD') as currency, SUM(consume_amount) as amount").
		Group("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')").
		Scan(&results).Error
	if err != nil {
		return 0, err
	}

	var totalUSD float64
	for _, rec := range results {
		amount, _ := rec.Amount.Float64()
		if rec.Currency == "CNY" {
			totalUSD += amount / 7.2
		} else {
			totalUSD += amount
		}
	}
	return totalUSD, nil
}

func (r *billRepository) GetTopResources(month string, limit int) ([]map[string]interface{}, error) {
	var results []struct {
		InstanceID   string          `gorm:"column:instance_id"`
		ResourceName string          `gorm:"column:resource_name"`
		ServiceCode  string          `gorm:"column:service_code"`
		Vendor       string          `gorm:"column:vendor"`
		Currency     string          `gorm:"column:currency"`
		Cost         decimal.Decimal `gorm:"column:cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ? AND instance_id IS NOT NULL AND instance_id != ''", month).
		Select("instance_id, MAX(resource_name) as resource_name, MAX(service_code) as service_code, MAX(vendor) as vendor, MAX(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')) as currency, SUM(consume_amount) as cost").
		Group("instance_id").
		Order("cost DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	resources := make([]map[string]interface{}, 0, len(results))
	for _, rec := range results {
		cost, _ := rec.Cost.Float64()
		resources = append(resources, map[string]interface{}{
			"resource_id":   rec.InstanceID,
			"resource_name": rec.ResourceName,
			"service_code":  rec.ServiceCode,
			"vendor":        rec.Vendor,
			"currency":      rec.Currency,
			"cost":          cost,
		})
	}
	return resources, nil
}

func (r *billRepository) GetCostByVendor(month string) (map[string]VendorCost, error) {
	var results []struct {
		Vendor   string          `gorm:"column:vendor"`
		Currency string          `gorm:"column:currency"`
		Cost     decimal.Decimal `gorm:"column:cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("vendor, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD') as currency, SUM(consume_amount) as cost").
		Group("vendor").Group("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	costByVendor := make(map[string]VendorCost)
	for _, rec := range results {
		cost, _ := rec.Cost.Float64()
		costByVendor[rec.Vendor] = VendorCost{Cost: cost, Currency: rec.Currency}
	}
	return costByVendor, nil
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
		Select("cycle as date, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD') as currency, SUM(consume_amount) as cost").
		Group("cycle").Group("COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra, '$.currency')), 'USD')").
		Order("date").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	dateMap := make(map[string]float64)
	for _, rec := range results {
		cost, _ := rec.Cost.Float64()
		if rec.Currency == "CNY" {
			cost /= 7.2
		}
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

func (r *billRepository) GetVMRecords(vendor, month string) ([]model.BillRecord, error) {
	var records []model.BillRecord
	vmKeywords := []string{"ec2", "ecs", "compute", "instance", "vm", "virtualmachine"}
	query := r.db.Model(&model.BillRecord{}).
		Where("vendor = ? AND cycle = ?", vendor, month)
	likeClauses := make([]string, 0, len(vmKeywords))
	likeArgs := make([]interface{}, 0, len(vmKeywords))
	for _, kw := range vmKeywords {
		likeClauses = append(likeClauses, "LOWER(resource_type) LIKE ? OR LOWER(service_code) LIKE ?")
		likeArgs = append(likeArgs, "%"+kw+"%", "%"+kw+"%")
	}
	combined := strings.Join(likeClauses, " OR ")
	if combined != "" {
		query = query.Where("("+combined+")", likeArgs...)
	}
	err := query.Find(&records).Error
	return records, err
}

func (r *billRepository) GetVMRecordsByGroup(vendor, month, groupBy string) (map[string]float64, error) {
	var results []struct {
		Group string
		Cost  float64
	}
	groupExpr := "resource_type"
	switch groupBy {
	case "service_code":
		groupExpr = "service_code"
	case "region":
		groupExpr = "region"
	}
	vmFilter := "(LOWER(resource_type) LIKE '%ec2%' OR LOWER(resource_type) LIKE '%ecs%' OR LOWER(resource_type) LIKE '%compute%' OR LOWER(resource_type) LIKE '%instance%' OR LOWER(service_code) LIKE '%ec2%' OR LOWER(service_code) LIKE '%ecs%')"
	sql := fmt.Sprintf("SELECT COALESCE(NULLIF(TRIM(%s), ''), 'unknown') AS `group`, SUM(consume_amount) AS cost FROM bill_records WHERE vendor = ? AND cycle = ? AND %s GROUP BY `group` ORDER BY cost DESC", groupExpr, vmFilter)
	err := r.db.Raw(sql, vendor, month).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]float64, len(results))
	for _, rec := range results {
		result[rec.Group] = rec.Cost
	}
	return result, nil
}

type VendorCost struct {
	Cost     float64
	Currency string
}

type IdleResource struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	Cost         float64 `json:"cost"`
	Vendor       string  `json:"vendor"`
	Region       string  `json:"region"`
	Currency     string  `json:"currency"`
	DaysInactive int     `json:"days_inactive"`
}

func (r *billRepository) GetIdleResources() ([]IdleResource, error) {
	var results []struct {
		ResourceID   string  `gorm:"column:resource_id"`
		ResourceName string  `gorm:"column:resource_name"`
		Cost         float64 `gorm:"column:cost"`
		Vendor       string  `gorm:"column:vendor"`
		Region       string  `gorm:"column:region"`
		Currency     string  `gorm:"column:currency"`
	}
	thresholdCycle := time.Now().AddDate(0, -1, 0).Format("2006-01")
	err := r.db.Raw(`
		SELECT br.resource_id, br.resource_name, 0.0 AS cost,
		       COALESCE(bca.cloud_type, '') AS vendor, COALESCE(br.region, '') AS region,
		       CASE WHEN bca.cloud_type = 'aws' THEN 'USD' ELSE 'CNY' END AS currency
		FROM bill_resources br
		INNER JOIN bill_cloud_accounts bca ON bca.id = br.cloud_account_id AND bca.status = 'active'
		INNER JOIN (
			SELECT instance_id, MAX(cycle) AS max_cycle
			FROM bill_records
			GROUP BY instance_id
		) agg ON agg.instance_id = br.resource_id
		WHERE agg.max_cycle < ?
		ORDER BY agg.max_cycle ASC
		LIMIT 100
	`, thresholdCycle).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	resources := make([]IdleResource, len(results))
	for i, row := range results {
		resources[i] = IdleResource{ResourceID: row.ResourceID, ResourceName: row.ResourceName, Cost: row.Cost, Vendor: row.Vendor, Region: row.Region, Currency: row.Currency}
	}
	return resources, nil
}

func (r *billRepository) GetLargeResources() ([]IdleResource, error) {
	var results []struct {
		ResourceID   string  `gorm:"column:resource_id"`
		ResourceName string  `gorm:"column:resource_name"`
		Cost         float64 `gorm:"column:cost"`
		Vendor       string  `gorm:"column:vendor"`
		Region       string  `gorm:"column:region"`
		Currency     string  `gorm:"column:currency"`
	}
	thresholdCycle := time.Now().AddDate(0, 0, -90).Format("2006-01")
	err := r.db.Raw(`
		SELECT b.instance_id as resource_id,
			   MAX(b.resource_name) as resource_name,
			   SUM(b.consume_amount) / COUNT(DISTINCT b.cycle) as cost,
			   MAX(b.vendor) as vendor,
			   MAX(b.region) as region,
			   MAX(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(b.extra, '$.currency')), 'USD')) as currency
		FROM bill_records b
		WHERE b.cycle >= ?
		  AND b.instance_id IS NOT NULL AND b.instance_id != ''
		GROUP BY b.instance_id
		HAVING SUM(b.consume_amount) / COUNT(DISTINCT b.cycle) > 500
		ORDER BY cost DESC
		LIMIT 50
	`, thresholdCycle).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	resources := make([]IdleResource, len(results))
	for i, row := range results {
		resources[i] = IdleResource{ResourceID: row.ResourceID, ResourceName: row.ResourceName, Cost: row.Cost, Vendor: row.Vendor, Region: row.Region, Currency: row.Currency}
	}
	return resources, nil
}
