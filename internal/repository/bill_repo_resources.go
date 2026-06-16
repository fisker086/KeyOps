package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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

func (r *billRepository) GetTotalCostByMonth(month string) (float64, error) {
	var results []struct {
		Currency string          `gorm:"column:currency"`
		Amount   decimal.Decimal `gorm:"column:amount"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END as currency, SUM(consume_amount) as amount").
		Group("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END").
		Scan(&results).Error
	if err != nil {
		return 0, err
	}

	var totalUSD float64
	for _, rec := range results {
		amount, _ := rec.Amount.Float64()
		if rec.Currency == "USD" {
			totalUSD += amount
		} else {
			totalUSD += amount / r.effectiveUSDToCNYRate()
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
	// FORCE INDEX 使按月+实例聚合走 idx_cycle_instance_id，避免全表扫描
	err := r.db.Raw(`
SELECT instance_id,
       MAX(resource_name) AS resource_name,
       MAX(service_code) AS service_code,
       MAX(vendor) AS vendor,
       MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
       SUM(consume_amount) AS cost
FROM bill_records FORCE INDEX (idx_cycle_instance_id)
WHERE cycle = ? AND instance_id IS NOT NULL AND instance_id <> ''
GROUP BY instance_id
ORDER BY cost DESC
LIMIT ?`, month, limit).Scan(&results).Error
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

type VendorCost struct {
	Cost          float64
	EffectiveCost float64
	Currency      string
}

func (r *billRepository) GetCostByVendor(month string) (map[string]VendorCost, error) {
	var results []struct {
		Vendor        string          `gorm:"column:vendor"`
		Currency      string          `gorm:"column:currency"`
		Cost          decimal.Decimal `gorm:"column:cost"`
		EffectiveCost decimal.Decimal `gorm:"column:effective_cost"`
	}
	err := r.db.Model(&model.BillRecord{}).
		Where("cycle = ?", month).
		Select("vendor, CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END as currency, SUM(consume_amount) as cost, SUM(effective_cost) as effective_cost").
		Group("vendor").Group("CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	costByVendor := make(map[string]VendorCost)
	for _, rec := range results {
		cost, _ := rec.Cost.Float64()
		effectiveCost, _ := rec.EffectiveCost.Float64()
		costByVendor[rec.Vendor] = VendorCost{Cost: cost, EffectiveCost: effectiveCost, Currency: rec.Currency}
	}
	return costByVendor, nil
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
	sql := "SELECT COALESCE(NULLIF(TRIM(" + groupExpr + "), ''), 'unknown') AS `group`, SUM(consume_amount) AS cost FROM bill_records WHERE vendor = ? AND cycle = ? AND " + vmFilter + " GROUP BY `group` ORDER BY cost DESC"
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
	// 用同步时维护的 last_seen 判断闲置，避免对 bill_records 做逐行 NOT EXISTS（曾达 40s+）
	idleBefore := time.Now().AddDate(0, -1, 0)
	err := r.db.Raw(`
		SELECT br.resource_id, br.resource_name, 0.0 AS cost,
		       COALESCE(bca.cloud_type, '') AS vendor, COALESCE(br.region, '') AS region,
		       CASE WHEN bca.cloud_type = 'aws' THEN 'USD' ELSE 'CNY' END AS currency
		FROM bill_resources br
		INNER JOIN bill_cloud_accounts bca ON bca.id = br.cloud_account_id AND bca.status = 'active'
		WHERE br.last_seen IS NULL OR br.last_seen < ?
		ORDER BY br.last_seen ASC
		LIMIT 100
	`, idleBefore).Scan(&results).Error
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
	cycle := time.Now().Format("2006-01")
	err := r.db.Model(&model.BillDashboardAggregate{}).
		Select("agg_key AS resource_id, resource_name, cost_usd AS cost, vendor, '' AS region, 'USD' AS currency").
		Where("cycle = ? AND agg_type = 'instance' AND cost_usd > 500", cycle).
		Order("cost_usd DESC").
		Limit(50).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	resources := make([]IdleResource, len(results))
	for i, row := range results {
		resources[i] = IdleResource{ResourceID: row.ResourceID, ResourceName: row.ResourceName, Cost: row.Cost, Vendor: row.Vendor, Region: row.Region, Currency: row.Currency}
	}
	return resources, nil
}

func (r *billRepository) GetTopServiceShare(cycle string) (string, float64, error) {
	var res struct {
		Service string          `gorm:"column:service"`
		Share   decimal.Decimal `gorm:"column:share"`
	}
	// 优先读预聚合表，避免 bill_records 双次全表扫描
	err := r.db.Raw(`
		SELECT agg_key AS service,
		       cost_usd / NULLIF((
		           SELECT SUM(cost_usd) FROM bill_dashboard_aggregates
		           WHERE cycle = ? AND agg_type = 'service'
		       ), 0) AS share
		FROM bill_dashboard_aggregates
		WHERE cycle = ? AND agg_type = 'service'
		ORDER BY cost_usd DESC
		LIMIT 1
	`, cycle, cycle).Scan(&res).Error
	if err == nil && res.Service != "" {
		share, _ := res.Share.Float64()
		return res.Service, share, nil
	}
	err = r.db.Raw(`
		SELECT COALESCE(NULLIF(TRIM(service_code), ''), 'unknown') AS service,
		       SUM(consume_amount) / (SELECT SUM(consume_amount) FROM bill_records WHERE cycle = ?) AS share
		FROM bill_records WHERE cycle = ?
		GROUP BY service ORDER BY share DESC LIMIT 1
	`, cycle, cycle).Scan(&res).Error
	if err != nil || res.Service == "" {
		return "", 0, err
	}
	share, _ := res.Share.Float64()
	return res.Service, share, nil
}

func (r *billRepository) GetUntaggedResources(cycle string, limit int) ([]IdleResource, error) {
	var results []IdleResource
	// 从预聚合表驱动（当月仅数百行），再 LEFT JOIN 资源清单查标签
	err := r.db.Raw(`
		SELECT agg.agg_key AS resource_id,
		       COALESCE(NULLIF(agg.resource_name, ''), '') AS resource_name,
		       agg.cost_usd AS cost,
		       COALESCE(NULLIF(agg.vendor, ''), bca.cloud_type, '') AS vendor,
		       COALESCE(br.region, '') AS region,
		       'USD' AS currency,
		       0 AS days_inactive
		FROM bill_dashboard_aggregates agg
		LEFT JOIN bill_resources br ON br.resource_id = agg.agg_key
		LEFT JOIN bill_cloud_accounts bca ON bca.id = br.cloud_account_id
		WHERE agg.cycle = ? AND agg.agg_type = 'instance'
		  AND (br.id IS NULL OR br.tags IS NULL OR br.tags = '')
		ORDER BY agg.cost_usd DESC
		LIMIT ?
	`, cycle, limit).Scan(&results).Error
	return results, err
}

func (r *billRepository) GetNewRegions(cycle string) ([]IdleResource, error) {
	lastMonth := previousCycle(cycle)
	var results []IdleResource
	err := r.db.Raw(`
		SELECT CONCAT(curr.vendor, '/', curr.agg_key) AS resource_id,
		       curr.vendor AS resource_name,
		       curr.cost_usd AS cost,
		       curr.vendor,
		       curr.agg_key AS region,
		       CASE WHEN curr.vendor = 'aws' THEN 'USD' ELSE 'CNY' END AS currency,
		       0 AS days_inactive
		FROM bill_dashboard_aggregates curr
		WHERE curr.cycle = ? AND curr.agg_type = 'region'
		  AND curr.agg_key IS NOT NULL AND curr.agg_key != '' AND curr.agg_key != 'unknown'
		  AND NOT EXISTS (
		      SELECT 1 FROM bill_dashboard_aggregates prev
		      WHERE prev.cycle = ? AND prev.agg_type = 'region'
		        AND prev.vendor = curr.vendor AND prev.agg_key = curr.agg_key
		      LIMIT 1
		  )
		ORDER BY curr.cost_usd DESC
		LIMIT 10
	`, cycle, lastMonth).Scan(&results).Error
	if err == nil {
		// 预聚合已存在时直接返回（空列表 = 无新区域，勿回退扫 bill_records）
		if has, hasErr := r.HasDashboardAggregates(cycle); hasErr == nil && has {
			return results, nil
		}
	}
	// 预聚合未就绪时回退
	err = r.db.Raw(`
		SELECT CONCAT(vendor, '/', region) AS resource_id,
		       MAX(vendor) AS resource_name,
		       SUM(consume_amount) AS cost,
		       MAX(vendor) AS vendor,
		       MAX(region) AS region,
		       MAX(CASE WHEN vendor = 'aws' THEN 'USD' ELSE 'CNY' END) AS currency,
		       0 AS days_inactive
		FROM bill_records curr
		WHERE curr.cycle = ? AND curr.region IS NOT NULL AND curr.region != ''
		  AND NOT EXISTS (
		      SELECT 1 FROM bill_records prev
		      WHERE prev.cycle = ? AND prev.vendor = curr.vendor AND prev.region = curr.region
		      LIMIT 1
		  )
		GROUP BY curr.vendor, curr.region
		ORDER BY cost DESC
		LIMIT 10
	`, cycle, lastMonth).Scan(&results).Error
	return results, err
}

func (r *billRepository) GetDailyDiscountRate(cycle string) (float64, error) {
	var res struct {
		TotalCost     decimal.Decimal `gorm:"column:total_cost"`
		TotalListCost decimal.Decimal `gorm:"column:total_list_cost"`
	}
	err := r.db.Model(&model.BillDailyCost{}).
		Select("SUM(cost) AS total_cost, SUM(list_cost) AS total_list_cost").
		Where("date LIKE ?", cycle+"-%").
		Scan(&res).Error
	if err != nil || res.TotalListCost.IsZero() {
		return 0, err
	}
	totalCost, _ := res.TotalCost.Float64()
	totalList, _ := res.TotalListCost.Float64()
	return 1 - totalCost/totalList, nil
}

func (r *billRepository) GetConsolidationCandidates(cycle string) ([]IdleResource, error) {
	// column not available in bill_records
	return nil, nil
}

func previousCycle(cycle string) string {
	t, err := time.Parse("2006-01", cycle)
	if err != nil {
		return cycle
	}
	return t.AddDate(0, -1, 0).Format("2006-01")
}

func billResourceKey(resource *model.BillResource) string {
	return resource.Vendor + ":" + resource.ResourceID
}
