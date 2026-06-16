package repository

import (
	"fmt"
	"log"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AggTypeVendor   = "vendor"
	AggTypeService  = "service"
	AggTypeRegion   = "region"
	AggTypeInstance = "instance"
)

func (r *billRepository) HasDashboardAggregates(cycle string) (bool, error) {
	var n int64
	err := r.db.Model(&model.BillDashboardAggregate{}).Where("cycle = ?", cycle).Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *billRepository) ListDashboardAggregates(cycle, aggType string, limit int) ([]model.BillDashboardAggregate, error) {
	var rows []model.BillDashboardAggregate
	q := r.db.Where("cycle = ? AND agg_type = ?", cycle, aggType).Order("cost_usd DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&rows).Error
	return rows, err
}

func recordCostToUSD(vendor string, amount decimal.Decimal, rate float64) float64 {
	v, _ := amount.Float64()
	if v == 0 {
		return 0
	}
	if vendor == "aws" {
		return v
	}
	if rate <= 0 {
		rate = config.DefaultUSDToCNYRate
	}
	return v / rate
}

func (r *billRepository) RebuildDashboardAggregates(cycle string) error {
	var totalCost, totalEffective float64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cycle = ?", cycle).Delete(&model.BillDashboardAggregate{}).Error; err != nil {
			return err
		}

		type row struct {
			Key           string          `gorm:"column:agg_key"`
			SubKey        string          `gorm:"column:sub_key"`
			Vendor        string          `gorm:"column:vendor"`
			Currency      string          `gorm:"column:currency"`
			Cost          decimal.Decimal `gorm:"column:cost"`
			EffectiveCost decimal.Decimal `gorm:"column:effective_cost"`
			ResourceName  string          `gorm:"column:resource_name"`
		}

		insertRows := func(aggType string, rows []row) error {
			if len(rows) == 0 {
				return nil
			}
			batch := make([]model.BillDashboardAggregate, 0, len(rows))
			for _, rec := range rows {
				c, _ := rec.Cost.Float64()
				ec, _ := rec.EffectiveCost.Float64()
				totalCost += c
				totalEffective += ec
				batch = append(batch, model.BillDashboardAggregate{
					Cycle:            cycle,
					AggType:          aggType,
					AggKey:           rec.Key,
					SubKey:           rec.SubKey,
					Vendor:           rec.Vendor,
					Currency:         rec.Currency,
					CostUSD:          recordCostToUSD(rec.Vendor, rec.Cost, r.effectiveUSDToCNYRate()),
					EffectiveCostUSD: recordCostToUSD(rec.Vendor, rec.EffectiveCost, r.effectiveUSDToCNYRate()),
					ResourceName:     rec.ResourceName,
				})
			}
			return tx.CreateInBatches(batch, 500).Error
		}

		var vendorRows []row
		if err := tx.Model(&model.BillRecord{}).
			Where("cycle = ?", cycle).
			Select(`vendor AS agg_key, '' AS sub_key, vendor,
				CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END AS currency,
				SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, '' AS resource_name`).
			Group("vendor").
			Scan(&vendorRows).Error; err != nil {
			return err
		}
		if err := insertRows(AggTypeVendor, vendorRows); err != nil {
			return err
		}

		var serviceRows []row
		if err := tx.Model(&model.BillRecord{}).
			Where("cycle = ?", cycle).
			Select(`COALESCE(NULLIF(TRIM(service_code), ''), NULLIF(TRIM(resource_type), ''), 'unknown') AS agg_key,
				'' AS sub_key, MAX(vendor) AS vendor,
				MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
				SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, '' AS resource_name`).
			Group("COALESCE(NULLIF(TRIM(service_code), ''), NULLIF(TRIM(resource_type), ''), 'unknown')").
			Scan(&serviceRows).Error; err != nil {
			return err
		}
		if err := insertRows(AggTypeService, serviceRows); err != nil {
			return err
		}

		var regionRows []row
		if err := tx.Model(&model.BillRecord{}).
			Where("cycle = ?", cycle).
			Select(`COALESCE(NULLIF(TRIM(region), ''), 'unknown') AS agg_key,
				'' AS sub_key, MAX(vendor) AS vendor,
				MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
				SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, '' AS resource_name`).
			Group("COALESCE(NULLIF(TRIM(region), ''), 'unknown')").
			Scan(&regionRows).Error; err != nil {
			return err
		}
		if err := insertRows(AggTypeRegion, regionRows); err != nil {
			return err
		}

		var instanceRows []row
		subSQL := tx.Model(&model.BillRecord{}).
			Select(`instance_id AS agg_key, MAX(service_code) AS sub_key, MAX(vendor) AS vendor,
				MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
				SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, MAX(resource_name) AS resource_name`).
			Where("cycle = ? AND instance_id IS NOT NULL AND instance_id <> ''", cycle).
			Group("instance_id").
			Order("cost DESC").
			Limit(300)
		if err := tx.Table("(?) AS t", subSQL).Scan(&instanceRows).Error; err != nil {
			// 部分方言不支持子查询表，回退为全量实例聚合后截断
			if err := tx.Model(&model.BillRecord{}).
				Select(`instance_id AS agg_key, MAX(service_code) AS sub_key, MAX(vendor) AS vendor,
					MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
					SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, MAX(resource_name) AS resource_name`).
				Where("cycle = ? AND instance_id IS NOT NULL AND instance_id <> ''", cycle).
				Group("instance_id").
				Order("cost DESC").
				Limit(300).
				Scan(&instanceRows).Error; err != nil {
				return fmt.Errorf("instance aggregates: %w", err)
			}
		}
		return insertRows(AggTypeInstance, instanceRows)
	})
	if err == nil {
		log.Printf("[BillSync] RebuildDashboardAggregates: cycle=%s cost=%.4f effective=%.4f", cycle, totalCost, totalEffective)
	}
	return err
}
