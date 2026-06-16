package repository

import (
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BreakdownResult struct {
	Breakdown       map[string]map[string]float64 `json:"breakdown"`
	Totals          map[string]float64            `json:"totals"`
	Granularity     string                        `json:"granularity"`
	GroupBy         string                        `json:"group_by"`
	CostType        string                        `json:"cost_type,omitempty"`
	ExcludedCost    float64                       `json:"excluded_cost,omitempty"`
	MarketplaceCost float64                       `json:"marketplace_cost,omitempty"`
}

type BillRepository interface {
	GetSummaryByCloud(startDate, endDate time.Time) (map[string]decimal.Decimal, error)
	CreateRecord(record *model.BillRecord) error
	ReplaceBillingRecordsForAccount(cloudAccountID uint, cycle string, records []model.BillRecord) error
	UpsertBillResources(resources []model.BillResource) error
	RebuildSummary(vendor, cycle string) error
	ListBillResources(vendor string, page, pageSize int) (int64, []model.BillResource, error)
	GetRecordsByCloudAccount(cloudAccountID uint, month string) ([]model.BillRecord, error)
	GetCostByService(month string) (map[string]float64, error)
	GetCostByRegion(month string) (map[string]float64, error)
	GetCostByCloudAccount(cloudAccountID uint, month string) (float64, error)
	GetCostByCloudAccountYear(cloudAccountID uint, year string) (float64, error)
	GetResourceCountByCloudAccount(cloudAccountID uint) (int, error)
	GetCostByCloudAccountsYear(accountIDs []uint, year string) (map[uint]float64, error)
	GetCostByCloudAccountsMonth(accountIDs []uint, month string) (map[uint]float64, error)
	GetResourceCountByCloudAccounts(accountIDs []uint) (map[uint]int, error)
	GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword, costType string) (*BreakdownResult, error)
	GetTotalCostByMonth(month string) (float64, error)
	GetAuxiliaryCostsByCycle(cycle string) (excludedService, marketplace float64, err error)
	GetAuxiliaryCostsByDateRange(startDate, endDate time.Time) (excludedService, marketplace float64, err error)
	GetMonthlyGrossTotalByCycle(cycle string) (float64, error)
	GetTopResources(month string, limit int) ([]map[string]interface{}, error)
	RebuildDashboardAggregates(cycle string) error
	HasDashboardAggregates(cycle string) (bool, error)
	ListDashboardAggregates(cycle, aggType string, limit int) ([]model.BillDashboardAggregate, error)
	GetCostByVendor(month string) (map[string]VendorCost, error)
	GetDailyCostTrend(startDate, endDate time.Time) ([]map[string]interface{}, error)
	GetVMRecords(vendor, month string) ([]model.BillRecord, error)
	GetVMRecordsByGroup(vendor, month, groupBy string) (map[string]float64, error)
	GetIdleResources() ([]IdleResource, error)
	GetLargeResources() ([]IdleResource, error)
	GetTopServiceShare(cycle string) (string, float64, error)
	GetUntaggedResources(cycle string, limit int) ([]IdleResource, error)
	GetNewRegions(cycle string) ([]IdleResource, error)
	GetDailyDiscountRate(cycle string) (float64, error)
	GetConsolidationCandidates(cycle string) ([]IdleResource, error)

	// ReplaceDailyCosts 替换指定周期的日费用聚合（全删全插）
	ReplaceDailyCosts(cycle string, costs []model.BillDailyCost) error
	// GetDailyCostsFromRecords 从 bill_records 聚合日费用
	GetDailyCostsFromRecords(cycle string) ([]model.BillDailyCost, error)
	// GetDailyCostRange 查询日期范围内的日费用（按日期、vendor 汇总）
	GetDailyCostRange(startDate, endDate time.Time) ([]model.BillDailyCost, error)

	// QueryRows 通用聚合查询， Engine.query()
	QueryRows(q Query) (*QueryResult, error)
}

type billRepository struct {
	db           *gorm.DB
	usdToCNYRate float64
}

func NewBillRepository(db *gorm.DB, usdToCNYRate float64) BillRepository {
	return &billRepository{db: db, usdToCNYRate: config.EffectiveUSDToCNYRate(usdToCNYRate)}
}

func (r *billRepository) effectiveUSDToCNYRate() float64 {
	return config.EffectiveUSDToCNYRate(r.usdToCNYRate)
}

func (r *billRepository) dateBucketExpr(column, granularity string) string {
	// cycle 列本身就是 'YYYY-MM' 字符串账期，直接作为月度桶。
	// 否则 MySQL 的 DATE_FORMAT('2026-06', ...) 会把它当 datetime 解析失败返回 NULL，
	// 导致所有月份塌进空 key、X 轴丢失月份。
	if column == "cycle" {
		return column
	}
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
