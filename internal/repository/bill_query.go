package repository

import (
	"fmt"
	"math"
	"strings"
)

// Dimension 表示可聚合的账单维度， 的 Dimension 枚举
type Dimension string

const (
	DimVendor       Dimension = "vendor"
	DimService      Dimension = "service"
	DimServiceCode  Dimension = "service_code"
	DimRegion       Dimension = "region"
	DimAccount      Dimension = "account"
	DimResourceType Dimension = "resource_type"
	DimInstanceType Dimension = "instance_type"
	DimTag          Dimension = "tag"
)

// QueryFilter 过滤条件
type QueryFilter struct {
	Field string // 字段名：vendor, region, service_code, account_id
	Value string
}

// Query 通用账单查询， 的 Query struct
type Query struct {
	Dimensions  []Dimension   // GROUP BY 维度
	Filters     []QueryFilter // WHERE 条件
	Vendor      string        // 快速过滤 vendor
	ServiceCode string        // 快速过滤 service_code
	DateFrom    string        // YYYY-MM
	DateTo      string        // YYYY-MM
	SortBy      string        // cost 或 name
	Descending  bool
	Limit       int
	Offset      int
	Keyword     string // 模糊搜索 resource_name / instance_id
}

// AggregatedRow 聚合结果行， 的 AggregatedRow
type AggregatedRow struct {
	Date  string  `json:"date,omitempty"`
	Group string  `json:"group"`
	Cost  float64 `json:"cost"`
}

// QueryResult 聚合查询结果
type QueryResult struct {
	Rows  []AggregatedRow `json:"rows"`
	Total float64         `json:"total"`
}

func (d Dimension) String() string { return string(d) }

// needsJoin 检查维度是否需要 JOIN 其他表
func (d Dimension) needsJoin() bool {
	return false
}

func (d Dimension) groupExpr() (selectAlias, groupByExpr string) {
	switch d {
	case DimVendor:
		e := "COALESCE(vendor, 'unknown')"
		return e + " AS grp", e
	case DimServiceCode:
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(service_code), ''), 'unknown'))"
		return e + " AS grp", e
	case DimRegion:
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(region), ''), 'unknown'))"
		return e + " AS grp", e
	case DimResourceType:
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(resource_type), ''), 'unknown'))"
		return e + " AS grp", e
	case DimInstanceType:
		e := "COALESCE(NULLIF(TRIM(instance_type), ''), 'unknown')"
		return e + " AS grp", e
	case DimAccount:
		e := "CONCAT(COALESCE(NULLIF(TRIM(bill_records.vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(bill_records.account_id), ''), 'unknown'))"
		return e + " AS grp", e
	case DimService:
		e := "CONCAT(COALESCE(NULLIF(TRIM(vendor), ''), 'unknown'), '|', COALESCE(NULLIF(TRIM(service_type), ''), NULLIF(TRIM(service_code), ''), 'unknown'))"
		return e + " AS grp", e
	default:
		return "COALESCE(vendor, 'unknown') AS grp", "COALESCE(vendor, 'unknown')"
	}
}

// QueryRows 通用聚合查询引擎 —  Engine.query() 的 filter→group→aggregate→sort→limit 模式。
// 返回原始 consume_amount 合计（未做币种换算），由 service 层统一处理 currency normalization。
func (r *billRepository) QueryRows(q Query) (*QueryResult, error) {
	if r.db == nil {
		return &QueryResult{}, nil
	}

	needsJoin := false
	for _, dim := range q.Dimensions {
		if dim.needsJoin() {
			needsJoin = true
			break
		}
	}

	baseQuery := r.db.Table("bill_records")
	if needsJoin {
		baseQuery = baseQuery.Joins("LEFT JOIN bill_cloud_accounts ca ON ca.id = bill_records.cloud_account_id")
	}

	for _, f := range q.Filters {
		if f.Value != "" {
			baseQuery = baseQuery.Where(f.Field+" = ?", f.Value)
		}
	}
	if q.Vendor != "" {
		baseQuery = baseQuery.Where("vendor = ?", q.Vendor)
	}
	if q.ServiceCode != "" {
		baseQuery = baseQuery.Where("service_code = ?", q.ServiceCode)
	}
	if q.DateFrom != "" {
		baseQuery = baseQuery.Where("cycle >= ?", q.DateFrom)
	}
	if q.DateTo != "" {
		baseQuery = baseQuery.Where("cycle <= ?", q.DateTo)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		baseQuery = baseQuery.Where("(resource_name LIKE ? OR instance_id LIKE ?)", kw, kw)
	}

	selects := "COALESCE(SUM(consume_amount), 0) AS cost"
	groupBys := make([]string, 0)

	for _, dim := range q.Dimensions {
		alias, expr := dim.groupExpr()
		selects = alias + ", " + selects
		groupBys = append(groupBys, expr)
	}

	if len(groupBys) == 0 {
		groupBys = append(groupBys, "1")
	}

	orderClause := "cost DESC"
	if q.SortBy == "name" && len(q.Dimensions) > 0 {
		_, firstExpr := q.Dimensions[0].groupExpr()
		orderClause = firstExpr
		if !q.Descending {
			orderClause += " ASC"
		} else {
			orderClause += " DESC"
		}
	} else if q.SortBy != "name" {
		if !q.Descending {
			orderClause = "cost ASC"
		}
	}

	query := baseQuery.Select(selects).Group(strings.Join(groupBys, ", ")).Order(orderClause)
	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	if q.Offset > 0 {
		query = query.Offset(q.Offset)
	}

	var results []struct {
		Grp  string
		Cost float64
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("QueryRows: %w", err)
	}

	total := 0.0
	rows := make([]AggregatedRow, 0, len(results))
	for _, row := range results {
		cost := math.Round(row.Cost*100) / 100
		total += row.Cost
		rows = append(rows, AggregatedRow{
			Group: row.Grp,
			Cost:  cost,
		})
	}

	return &QueryResult{Rows: rows, Total: total}, nil
}
