package auth

import (
	"fmt"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

type BillService struct {
	repo *repository.BillRepository
}

func NewBillService(repo *repository.BillRepository) *BillService {
	return &BillService{repo: repo}
}

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
	// TODO: 实现虚拟机分摊逻辑
	// 目前返回空数据
	return map[string]interface{}{
		"vendor":     vendor,
		"month":      month,
		"split_type": splitType,
		"data":       map[string]interface{}{},
	}, nil
}

// GetPriceList 获取单价列表
func (s *BillService) GetPriceList() (interface{}, error) {
	return s.repo.GetPriceList()
}

// CreatePrice 创建单价
func (s *BillService) CreatePrice(price *model.BillPrice) (interface{}, error) {
	if err := s.repo.CreatePrice(price); err != nil {
		return nil, err
	}
	return price, nil
}

// UpdatePrice 更新单价
func (s *BillService) UpdatePrice(id string, price *model.BillPrice) (interface{}, error) {
	// 检查是否存在
	existing, err := s.repo.GetPriceByID(id)
	if err != nil {
		return nil, fmt.Errorf("单价不存在: %v", err)
	}

	// 更新字段
	if price.Vendor != "" {
		existing.Vendor = price.Vendor
	}
	if price.ResourceType != "" {
		existing.ResourceType = price.ResourceType
	}
	if price.Scale != "" {
		existing.Scale = price.Scale
	}
	if price.Cluster != "" {
		existing.Cluster = price.Cluster
	}
	if !price.Price.IsZero() {
		existing.Price = price.Price
	}
	if price.Description != "" {
		existing.Description = price.Description
	}

	if err := s.repo.UpdatePrice(id, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// GetResource 获取我的资源列表
func (s *BillService) GetResource() (interface{}, error) {
	// TODO: 实现资源列表查询逻辑
	return []interface{}{}, nil
}

