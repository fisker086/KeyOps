package bill

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

func (s *BillService) MCPGetCostByVendor(month string) (map[string]repository.VendorCost, error) {
	return s.repo.GetCostByVendor(month)
}

func (s *BillService) MCPGetCostByService(month string) (map[string]float64, error) {
	return s.repo.GetCostByService(month)
}

func (s *BillService) MCPGetCostByRegion(month string) (map[string]float64, error) {
	return s.repo.GetCostByRegion(month)
}

func (s *BillService) MCPListBillResources(vendor string, page, pageSize int) (int64, []model.BillResource, error) {
	return s.repo.ListBillResources(vendor, page, pageSize)
}

func (s *BillService) MCPGetDailyCostTrend(startDate, endDate time.Time) ([]map[string]interface{}, error) {
	return s.repo.GetDailyCostTrend(startDate, endDate)
}
