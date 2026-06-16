package repository

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func toFloat64(d decimal.Decimal) float64 {
	v, _ := d.Float64()
	return v
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
		Select("vendor, SUM(effective_cost) as total").
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
	if len(records) > 0 {
		var totalCost, totalEffective, totalList, totalMarketplace, totalExcluded decimal.Decimal
		for _, rec := range records {
			totalCost = totalCost.Add(rec.ConsumeAmount)
			totalEffective = totalEffective.Add(rec.EffectiveCost)
			totalList = totalList.Add(rec.ListCost)
			totalMarketplace = totalMarketplace.Add(rec.MarketplaceCost)
			totalExcluded = totalExcluded.Add(rec.ExcludedServiceCost)
		}
		log.Printf("[BillSync] ReplaceBillingRecords: account=%d cycle=%s records=%d cost=%.4f effective=%.4f list=%.4f marketplace=%.4f excluded=%.4f",
			cloudAccountID, cycle, len(records),
			toFloat64(totalCost), toFloat64(totalEffective), toFloat64(totalList), toFloat64(totalMarketplace), toFloat64(totalExcluded))
	} else {
		log.Printf("[BillSync] ReplaceBillingRecords: account=%d cycle=%s records=0 (deleting only)", cloudAccountID, cycle)
	}

	// 大批量写入勿包在单事务里（~200 万行会导致超时/回滚后数据为空）
	if err := r.db.Where("cloud_account_id = ? AND cycle = ?", cloudAccountID, cycle).
		Delete(&model.BillRecord{}).Error; err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if r.db.Dialector.Name() == "postgres" {
		return r.db.CreateInBatches(records, 2000).Error
	}

	return batchInsertBillRecords(r.db, records, cloudAccountID, cycle)
}

func batchInsertBillRecords(tx *gorm.DB, records []model.BillRecord, cloudAccountID uint, cycle string) error {
	const batchSize = 2000
	columns := "`vendor`,`cycle`,`instance_id`,`resource_name`,`spec_desc`,`consume_amount`,`effective_cost`,`list_cost`,`marketplace_cost`,`excluded_service_cost`,`usage_date`,`resource_type`,`resource_code`,`service_type`,`service_code`,`region`,`account_id`,`cloud_account_id`,`tags`,`extra`,`created_at`,`updated_at`"
	now := time.Now()
	totalBatches := (len(records) + batchSize - 1) / batchSize

	valuePlaceholders := make([]string, 0, batchSize)
	args := make([]interface{}, 0, batchSize*22)

	for i, r := range records {
		valuePlaceholders = append(valuePlaceholders, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")

		createdAt := r.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := r.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}

		args = append(args, r.Vendor, r.Cycle, r.InstanceID, r.ResourceName, r.SpecDesc, r.ConsumeAmount, r.EffectiveCost, r.ListCost, r.MarketplaceCost, r.ExcludedServiceCost, r.UsageDate, r.ResourceType, r.ResourceCode, r.ServiceType, r.ServiceCode, r.Region, r.AccountID, r.CloudAccountID, r.Tags, r.Extra, createdAt, updatedAt)

		if len(valuePlaceholders) == batchSize || i == len(records)-1 {
			batchNo := i/batchSize + 1
			sql := "INSERT INTO `bill_records` (" + columns + ") VALUES " + strings.Join(valuePlaceholders, ",")
			if err := tx.Exec(sql, args...).Error; err != nil {
				return fmt.Errorf("insert batch %d/%d: %w", batchNo, totalBatches, err)
			}
			if batchNo == 1 || batchNo == totalBatches || batchNo%100 == 0 {
				log.Printf("[BillSync] insert progress: account=%d cycle=%s batch=%d/%d rows=%d",
					cloudAccountID, cycle, batchNo, totalBatches, i+1)
			}
			valuePlaceholders = valuePlaceholders[:0]
			args = args[:0]
		}
	}
	log.Printf("[BillSync] insert complete: account=%d cycle=%s rows=%d", cloudAccountID, cycle, len(records))
	return nil
}

func (r *billRepository) GetAuxiliaryCostsByDateRange(startDate, endDate time.Time) (float64, float64, error) {
	cycles := billingCyclesBetween(startDate, endDate)
	if len(cycles) == 0 {
		return 0, 0, nil
	}
	var row struct {
		Excluded    decimal.Decimal `gorm:"column:excluded"`
		Marketplace decimal.Decimal `gorm:"column:marketplace"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Select("COALESCE(SUM(excluded_service_cost), 0) AS excluded, COALESCE(SUM(marketplace_cost), 0) AS marketplace").
		Where("cycle IN ?", cycles).
		Scan(&row).Error
	if err != nil {
		return 0, 0, fmt.Errorf("get auxiliary costs by date range: %w", err)
	}
	excluded, _ := row.Excluded.Float64()
	marketplace, _ := row.Marketplace.Float64()
	return excluded, marketplace, nil
}

func (r *billRepository) GetAuxiliaryCostsByCycle(cycle string) (float64, float64, error) {
	var row struct {
		Excluded    decimal.Decimal `gorm:"column:excluded"`
		Marketplace decimal.Decimal `gorm:"column:marketplace"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Select("COALESCE(SUM(excluded_service_cost), 0) AS excluded, COALESCE(SUM(marketplace_cost), 0) AS marketplace").
		Where("cycle = ?", cycle).
		Scan(&row).Error
	if err != nil {
		return 0, 0, fmt.Errorf("get auxiliary costs by cycle: %w", err)
	}
	excluded, _ := row.Excluded.Float64()
	marketplace, _ := row.Marketplace.Float64()
	return excluded, marketplace, nil
}

func (r *billRepository) GetMonthlyGrossTotalByCycle(cycle string) (float64, error) {
	var row struct {
		Gross decimal.Decimal `gorm:"column:gross"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Select("COALESCE(SUM(consume_amount), 0) + COALESCE(SUM(marketplace_cost), 0) AS gross").
		Where("cycle = ? AND vendor = ?", cycle, "aws").
		Scan(&row).Error
	if err != nil {
		return 0, fmt.Errorf("get monthly gross total: %w", err)
	}
	gross, _ := row.Gross.Float64()
	return gross, nil
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
