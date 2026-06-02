package bill

// GetRecords 获取账单明细列表
func (s *BillService) GetRecords(vendor, month, resourceCode, serviceCode string, page, pageSize int, queryRemote, withAmount bool) (interface{}, error) {
	// TODO: 如果 queryRemote 为 true，需要调用云厂商API
	// 目前先实现本地数据库查询

	total, records, err := s.repo.GetRecords(vendor, month, resourceCode, serviceCode, page, pageSize)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"total":   total,
		"records": records,
	}

	// 如果需要计算费用
	if withAmount && pageSize == 0 {
		var totalAmount float64
		for _, record := range records {
			amount, _ := record.ConsumeAmount.Float64()
			totalAmount += amount
		}
		result["amount"] = totalAmount
	}

	return result, nil
}

// GetSummary 获取月度账单汇总
func (s *BillService) GetSummary(vendor, month string, queryRemote bool) (interface{}, error) {
	// TODO: 如果 queryRemote 为 true，需要调用云厂商API
	// 目前先实现本地数据库查询

	summary, details, err := s.repo.GetSummary(vendor, month)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"summary": summary,
		"details": details,
	}

	return result, nil
}

// GetStatistics 获取费用统计
func (s *BillService) GetStatistics(month string) (interface{}, error) {
	return s.repo.GetSummaryCount(month)
}

// GetTrend 获取费用趋势
func (s *BillService) GetTrend(vendor, year string) (interface{}, error) {
	return s.repo.GetSummaryTrend(vendor, year)
}

// GetTrendMonth 获取费用趋势月份列表
func (s *BillService) GetTrendMonth(year string) (interface{}, error) {
	return s.repo.GetSummaryTrendMonth(year)
}

// GetVM 获取虚拟机分摊账单
func (s *BillService) GetVM(vendor, month, splitType string, withDetail bool) (interface{}, error) {
	if splitType == "" {
		splitType = "resource_type"
	}
	byGroup, err := s.repo.GetVMRecordsByGroup(vendor, month, splitType)
	if err != nil {
		return nil, err
	}

	total := 0.0
	for _, cost := range byGroup {
		total += cost
	}

	items := make([]map[string]interface{}, 0, len(byGroup))
	for group, cost := range byGroup {
		item := map[string]interface{}{
			"group": group,
			"cost":  cost,
			"ratio": 0.0,
		}
		if total > 0 {
			item["ratio"] = cost / total * 100
		}
		items = append(items, item)
	}

	result := map[string]interface{}{
		"vendor":     vendor,
		"month":      month,
		"split_type": splitType,
		"total_cost": total,
		"items":      items,
	}

	if withDetail {
		records, err := s.repo.GetVMRecords(vendor, month)
		if err == nil {
			details := make([]map[string]interface{}, 0, len(records))
			for _, r := range records {
				amount, _ := r.ConsumeAmount.Float64()
				details = append(details, map[string]interface{}{
					"instance_id":   r.InstanceID,
					"resource_name": r.ResourceName,
					"resource_type": r.ResourceType,
					"service_code":  r.ServiceCode,
					"cost":          amount,
					"region":        r.Region,
				})
			}
			result["details"] = details
		}
	}

	return result, nil
}
