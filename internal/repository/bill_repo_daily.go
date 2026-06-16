package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func (r *billRepository) ReplaceDailyCosts(cycle string, costs []model.BillDailyCost) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("date LIKE ?", cycle+"-%").Delete(&model.BillDailyCost{}).Error; err != nil {
			return err
		}
		if len(costs) == 0 {
			return nil
		}
		return batchInsertDailyCosts(tx, costs)
	})
}

func (r *billRepository) GetDailyCostsFromRecords(cycle string) ([]model.BillDailyCost, error) {
	var rows []struct {
		Date          string          `gorm:"column:date"`
		Vendor        string          `gorm:"column:vendor"`
		Cost          decimal.Decimal `gorm:"column:cost"`
		EffectiveCost decimal.Decimal `gorm:"column:effective_cost"`
		ListCost      decimal.Decimal `gorm:"column:list_cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Select("usage_date AS date, vendor, SUM(consume_amount) AS cost, SUM(effective_cost) AS effective_cost, SUM(list_cost) AS list_cost").
		Where("cycle = ?", cycle).
		Group("usage_date, vendor").
		Having("usage_date != ''").
		Order("usage_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get daily costs from records: %w", err)
	}
	result := make([]model.BillDailyCost, 0, len(rows))
	for _, row := range rows {
		cost, _ := row.Cost.Float64()
		effectiveCost, _ := row.EffectiveCost.Float64()
		listCost, _ := row.ListCost.Float64()
		if listCost < cost {
			listCost = cost
		}
		result = append(result, model.BillDailyCost{
			Date:          row.Date,
			Vendor:        row.Vendor,
			Cost:          cost,
			EffectiveCost: effectiveCost,
			ListCost:      listCost,
		})
	}
	return result, nil
}

func batchInsertDailyCosts(tx *gorm.DB, costs []model.BillDailyCost) error {
	const batchSize = 500
	columns := "`date`,`vendor`,`cost`,`effective_cost`,`list_cost`,`created_at`,`updated_at`"
	now := time.Now()

	valuePlaceholders := make([]string, 0, batchSize)
	args := make([]interface{}, 0, batchSize*7)

	for i, c := range costs {
		valuePlaceholders = append(valuePlaceholders, "(?,?,?,?,?,?,?)")
		args = append(args, c.Date, c.Vendor, c.Cost, c.EffectiveCost, c.ListCost, now, now)

		if len(valuePlaceholders) == batchSize || i == len(costs)-1 {
			sql := "INSERT INTO `bill_daily_cost` (" + columns + ") VALUES " + strings.Join(valuePlaceholders, ",")
			if err := tx.Exec(sql, args...).Error; err != nil {
				return err
			}
			valuePlaceholders = valuePlaceholders[:0]
			args = args[:0]
		}
	}
	return nil
}

func (r *billRepository) GetDailyCostRange(startDate, endDate time.Time) ([]model.BillDailyCost, error) {
	if startDate.After(endDate) {
		startDate, endDate = endDate, startDate
	}
	start := startDate.Format("2006-01-02")
	end := endDate.Format("2006-01-02")

	var rows []struct {
		Date          string          `gorm:"column:date"`
		Vendor        string          `gorm:"column:vendor"`
		Cost          decimal.Decimal `gorm:"column:cost"`
		EffectiveCost decimal.Decimal `gorm:"column:effective_cost"`
		ListCost      decimal.Decimal `gorm:"column:list_cost"`
	}
	err := r.db.Model(&model.BillDailyCost{}).
		Select("date, vendor, SUM(cost) AS cost, SUM(effective_cost) AS effective_cost, SUM(list_cost) AS list_cost").
		Where("date >= ? AND date <= ?", start, end).
		Group("date, vendor").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get daily cost range: %w", err)
	}

	result := make([]model.BillDailyCost, 0, len(rows))
	for _, row := range rows {
		cost, _ := row.Cost.Float64()
		effectiveCost, _ := row.EffectiveCost.Float64()
		listCost, _ := row.ListCost.Float64()
		if listCost < cost {
			listCost = cost
		}
		result = append(result, model.BillDailyCost{
			Date:          row.Date,
			Vendor:        row.Vendor,
			Cost:          cost,
			EffectiveCost: effectiveCost,
			ListCost:      listCost,
		})
	}
	return result, nil
}
