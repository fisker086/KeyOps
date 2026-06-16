package bill

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/shopspring/decimal"
)

type BillService struct {
	repo             repository.BillRepository
	cloudRepo        repository.CloudAccountRepository
	alertChannelRepo repository.AlertChannelRepository
	notifyURL        string
	syncScheduler    *SyncScheduler
	usdToCNYRate     float64

	mu          sync.Mutex
	syncCancels map[string]context.CancelFunc
}

func syncKey(accountID uint, cycle string) string {
	return fmt.Sprintf("%d_%s", accountID, cycle)
}

func NewBillService(repo repository.BillRepository, cloudRepo repository.CloudAccountRepository, alertChannelRepo repository.AlertChannelRepository, usdToCNYRate float64) *BillService {
	return &BillService{repo: repo, cloudRepo: cloudRepo, alertChannelRepo: alertChannelRepo, notifyURL: "http://localhost:8080/api/notify/plain", usdToCNYRate: config.EffectiveUSDToCNYRate(usdToCNYRate), syncCancels: make(map[string]context.CancelFunc)}
}

func (s *BillService) SetSyncScheduler(sch *SyncScheduler) {
	s.syncScheduler = sch
}

func (s *BillService) SetNotifyURL(url string) {
	if url != "" {
		s.notifyURL = url
	}
}

func (s *BillService) reloadSyncScheduler() {
	if s.syncScheduler != nil {
		s.syncScheduler.Reload()
	}
}

func (s *BillService) GetBillingSummaryByCloud(startDate, endDate time.Time) (map[string]decimal.Decimal, error) {
	return s.repo.GetSummaryByCloud(startDate, endDate)
}

func (s *BillService) GetCloudPricing(cloudType string, filters map[string]string) ([]model.BillPricing, error) {
	config := map[string]interface{}{
		"access_key_id":     filters["access_key_id"],
		"secret_access_key": filters["secret_access_key"],
		"region":            filters["region"],
	}

	adapter, err := cloud.NewCloudAdapter(cloudType, config)
	if err != nil {
		return nil, err
	}

	rawPrices, err := adapter.GetPricing()
	if err != nil {
		return nil, err
	}

	pricingList := make([]model.BillPricing, 0, len(rawPrices))
	for _, raw := range rawPrices {
		price := model.BillPricing{
			CloudType:    getString(raw, "cloud_type"),
			ServiceCode:  getString(raw, "service_code"),
			InstanceType: getString(raw, "instance_type"),
			Region:       getString(raw, "region"),
			PricePerUnit: decimal.NewFromFloat(getFloat(raw, "price_per_unit")),
			Currency:     getString(raw, "currency"),
			Unit:         getString(raw, "unit"),
			SKU:          getString(raw, "sku"),
		}
		if price.CloudType == "" {
			price.CloudType = cloudType
		}
		pricingList = append(pricingList, price)
	}

	return pricingList, nil
}
