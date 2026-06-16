package bill

import (
	"strings"

	"github.com/fisker086/keyops/internal/repository"
)

// ParseDimensions 将逗号分隔的维度字符串解析为 Dimension 列表
//  DTO 层的 parse_dimension()
func ParseDimensions(s string) []repository.Dimension {
	parts := strings.Split(s, ",")
	dims := make([]repository.Dimension, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "vendor", "cloud_type":
			dims = append(dims, repository.DimVendor)
		case "service", "svc":
			dims = append(dims, repository.DimService)
		case "service_code":
			dims = append(dims, repository.DimServiceCode)
		case "region":
			dims = append(dims, repository.DimRegion)
		case "account":
			dims = append(dims, repository.DimAccount)
		case "resource_type":
			dims = append(dims, repository.DimResourceType)
		case "instance_type":
			dims = append(dims, repository.DimInstanceType)
		}
	}
	return dims
}

// Query 通用查询 —  的 /api/query 端点
func (s *BillService) Query(dimensions []repository.Dimension, vendor, serviceCode, dateFrom, dateTo, sortBy, keyword string, descending bool, limit, offset int) (*repository.QueryResult, error) {
	q := repository.Query{
		Dimensions:  dimensions,
		Vendor:      vendor,
		ServiceCode: serviceCode,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		SortBy:      sortBy,
		Descending:  descending,
		Limit:       limit,
		Offset:      offset,
		Keyword:     keyword,
	}
	return s.repo.QueryRows(q)
}
