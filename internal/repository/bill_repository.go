package repository

import (
	"fmt"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BillRepository struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) *BillRepository {
	return &BillRepository{db: db}
}

// GetRecords 从本地数据库获取资源账单明细列表
func (r *BillRepository) GetRecords(vendor, month, resourceCode, serviceCode string, page, pageSize int) (total int64, records []model.BillRecord, err error) {
	query := r.db.Model(&model.BillRecord{}).
		Where("vendor = ? AND cycle = ?", vendor, month)

	if resourceCode != "" {
		query = query.Where("resource_code = ?", resourceCode)
	}
	if serviceCode != "" {
		query = query.Where("service_code = ?", serviceCode)
	}

	// 先查询总数
	err = query.Count(&total).Error
	if err != nil {
		return
	}

	// 如果总数为0，直接返回空列表（不是错误）
	if total == 0 {
		return 0, []model.BillRecord{}, nil
	}

	// 分页查询
	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Find(&records).Error
	return
}

// GetSummary 获取月度账单汇总
func (r *BillRepository) GetSummary(vendor, month string) (summary model.BillSummary, details []model.BillSummaryDetail, err error) {
	err = r.db.Model(&model.BillSummary{}).
		Where("vendor = ? AND cycle = ?", vendor, month).
		First(&summary).Error
	if err != nil {
		return summary, details, err
	}

	err = r.db.Model(&model.BillSummaryDetail{}).
		Where("summary_id = ?", summary.ID).
		Find(&details).Error
	return summary, details, err
}

// GetSummaryCount 获取当月总费用，用于前端展示饼图
func (r *BillRepository) GetSummaryCount(month string) (map[string]interface{}, error) {
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

	result := map[string]interface{}{
		"count":  count,
		"vendor": summaries,
	}
	return result, nil
}

// GetSummaryTrend 获取月度账单折线图数据
func (r *BillRepository) GetSummaryTrend(vendor, year string) (map[string][]model.BillSummary, error) {
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

	// 按云厂商归类账单
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

// GetSummaryTrendMonth 查询月份列表，用于前端折线图x轴
func (r *BillRepository) GetSummaryTrendMonth(year string) ([]string, error) {
	var monthList []string
	query := r.db.Model(&model.BillSummary{}).Select("DISTINCT cycle")

	if year != "" {
		query = query.Where("cycle LIKE ?", year+"-%").Order("cycle ASC")
	} else {
		// 默认查询最近6个月，按降序排列
		query = query.Order("cycle DESC").Limit(6)
	}

	err := query.Scan(&monthList).Error
	if err != nil {
		return nil, err
	}

	// 如果没有指定年份，需要反转顺序（因为查询时是降序，但前端可能需要升序）
	if year == "" && len(monthList) > 0 {
		// 反转数组，使其按时间升序排列
		for i, j := 0, len(monthList)-1; i < j; i, j = i+1, j-1 {
			monthList[i], monthList[j] = monthList[j], monthList[i]
		}
	}

	return monthList, nil
}

// GetPriceList 获取单价列表
func (r *BillRepository) GetPriceList() ([]model.BillPrice, error) {
	var prices []model.BillPrice
	err := r.db.Model(&model.BillPrice{}).Find(&prices).Error
	return prices, err
}

// CreatePrice 创建单价
func (r *BillRepository) CreatePrice(price *model.BillPrice) error {
	return r.db.Create(price).Error
}

// UpdatePrice 更新单价
func (r *BillRepository) UpdatePrice(id string, price *model.BillPrice) error {
	// 将 string ID 转换为 uint
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return fmt.Errorf("无效的ID格式: %v", err)
	}
	return r.db.Model(&model.BillPrice{}).Where("id = ?", uint(idUint)).Updates(price).Error
}

// GetPriceByID 根据ID获取单价
func (r *BillRepository) GetPriceByID(id string) (*model.BillPrice, error) {
	// 将 string ID 转换为 uint
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("无效的ID格式: %v", err)
	}
	var price model.BillPrice
	err = r.db.Where("id = ?", uint(idUint)).First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

