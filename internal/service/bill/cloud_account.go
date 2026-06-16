package bill

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
	"github.com/robfig/cron/v3"
)

const cloudAccountDetailsCacheTTL = 2 * time.Minute

type cloudAccountAggCache struct {
	yearCosts  map[uint]float64
	monthCosts map[uint]float64
	resources  map[uint]int
	expiresAt  time.Time
}

var (
	cloudAccountDetailsCacheMu sync.Mutex
	cloudAccountAggCacheStore  = make(map[string]cloudAccountAggCache)
)

// CloudAccountDetails 云账户费用详情
type CloudAccountDetails struct {
	Cost          float64 `json:"cost"`
	Forecast      float64 `json:"forecast,omitempty"`
	Resources     int     `json:"resources"`
	LastMonthCost float64 `json:"last_month_cost"`
}

func validateSyncCron(expr string) error {
	if expr == "" {
		return nil
	}
	normalized := normalizeBillSyncCron(expr)
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(normalized)
	if err != nil {
		return fmt.Errorf("无效的 cron 表达式 %q: %w", expr, err)
	}
	return nil
}

// AddCloudAccount 添加云账户
func (s *BillService) AddCloudAccount(account *model.CloudAccount) error {
	if account.CloudType == cloud.CloudTypeAWS {
		if strings.TrimSpace(account.BucketName) == "" {
			return fmt.Errorf("AWS billing requires bucket_name for CUR S3 import")
		}
	}

	if err := validateSyncCron(account.SyncCron); err != nil {
		return err
	}

	// 验证凭证
	config := map[string]interface{}{
		"access_key_id":     account.AccessKeyID,
		"secret_access_key": account.SecretAccessKey,
		"region":            account.Region,
	}

	adapter, err := cloud.NewCloudAdapter(account.CloudType, config)
	if err != nil {
		return err
	}

	info, err := adapter.ValidateCredentials()
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	if accountID, ok := info["account_id"].(string); ok {
		account.AccountID = accountID
	}

	account.Status = "active"
	if err := s.cloudRepo.Create(account); err != nil {
		return err
	}
	s.reloadSyncScheduler()
	return nil
}

// ListCloudAccounts 列出云账户
func (s *BillService) ListCloudAccounts(cloudType string) ([]model.CloudAccount, error) {
	return s.cloudRepo.List(cloudType)
}

// DeleteCloudAccount 删除云账户
func (s *BillService) DeleteCloudAccount(id uint) error {
	if err := s.cloudRepo.Delete(id); err != nil {
		return err
	}
	s.reloadSyncScheduler()
	return nil
}

// GetCloudAccount 获取云账户
func (s *BillService) GetCloudAccount(id uint) (*model.CloudAccount, error) {
	return s.cloudRepo.GetByID(id)
}

// UpdateCloudAccount 更新云账户（密钥为空则保留原值）
func (s *BillService) UpdateCloudAccount(id uint, patch *model.CloudAccount) error {
	existing, err := s.cloudRepo.GetByID(id)
	if err != nil {
		return err
	}
	if patch.Name != "" {
		existing.Name = patch.Name
	}
	if patch.CloudType != "" {
		existing.CloudType = patch.CloudType
	}
	if patch.AccessKeyID != "" {
		existing.AccessKeyID = patch.AccessKeyID
	}
	if patch.SecretAccessKey != "" {
		existing.SecretAccessKey = patch.SecretAccessKey
	}
	if patch.Region != "" {
		existing.Region = patch.Region
	}
	if patch.BucketName != "" {
		existing.BucketName = patch.BucketName
	}
	if patch.BucketPrefix != "" {
		existing.BucketPrefix = patch.BucketPrefix
	}
	if patch.ReportName != "" {
		existing.ReportName = patch.ReportName
	}
	if patch.AccountID != "" {
		existing.AccountID = patch.AccountID
	}
	if patch.Status != "" {
		existing.Status = patch.Status
	}
	// sync_cron 允许清空（设为空字符串）
	if patch.SyncCron != existing.SyncCron {
		if err := validateSyncCron(patch.SyncCron); err != nil {
			return err
		}
		existing.SyncCron = patch.SyncCron
	}
	if existing.CloudType == cloud.CloudTypeAWS {
		if strings.TrimSpace(existing.BucketName) == "" {
			return fmt.Errorf("AWS billing requires bucket_name for CUR S3 import")
		}
	}
	if patch.NotifyEnabled != nil {
		existing.NotifyEnabled = patch.NotifyEnabled
	}
	if patch.NotifyChannelID != 0 {
		existing.NotifyChannelID = patch.NotifyChannelID
	}
	return s.cloudRepo.Update(existing)
}

func cloudAccountAggCacheKey(year, month string) string {
	return year + ":" + month + ":" + time.Now().Truncate(cloudAccountDetailsCacheTTL).Format(time.RFC3339)
}

func getCachedCloudAccountAgg(year, month string) (cloudAccountAggCache, bool) {
	key := cloudAccountAggCacheKey(year, month)
	cloudAccountDetailsCacheMu.Lock()
	defer cloudAccountDetailsCacheMu.Unlock()
	entry, ok := cloudAccountAggCacheStore[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return cloudAccountAggCache{}, false
	}
	return entry, true
}

func setCachedCloudAccountAgg(year, month string, yearCosts map[uint]float64, monthCosts map[uint]float64, resources map[uint]int) {
	key := cloudAccountAggCacheKey(year, month)
	cloudAccountDetailsCacheMu.Lock()
	defer cloudAccountDetailsCacheMu.Unlock()
	cloudAccountAggCacheStore[key] = cloudAccountAggCache{
		yearCosts:  yearCosts,
		monthCosts: monthCosts,
		resources:  resources,
		expiresAt:  time.Now().Add(cloudAccountDetailsCacheTTL),
	}
}

// ListCloudAccountsWithDetails 列出云账户（含费用详情）
func (s *BillService) ListCloudAccountsWithDetails(cloudType string) ([]map[string]interface{}, error) {
	accounts, err := s.cloudRepo.List(cloudType)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	thisYear := now.Format("2006")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	accountIDs := make([]uint, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}

	var yearCosts, monthCosts map[uint]float64
	var resources map[uint]int
	if cached, ok := getCachedCloudAccountAgg(thisYear, lastMonth); ok {
		yearCosts, monthCosts, resources = cached.yearCosts, cached.monthCosts, cached.resources
	} else {
		yearCosts, _ = s.repo.GetCostByCloudAccountsYear(accountIDs, thisYear)
		monthCosts, _ = s.repo.GetCostByCloudAccountsMonth(accountIDs, lastMonth)
		resources, _ = s.repo.GetResourceCountByCloudAccounts(accountIDs)
		setCachedCloudAccountAgg(thisYear, lastMonth, yearCosts, monthCosts, resources)
	}

	result := make([]map[string]interface{}, 0, len(accounts))
	for _, account := range accounts {
		item := map[string]interface{}{
			"id":                account.ID,
			"name":              account.Name,
			"cloud_type":        account.CloudType,
			"access_key_id":     account.AccessKeyID,
			"account_id":        account.AccountID,
			"region":            account.Region,
			"bucket_name":       account.BucketName,
			"bucket_prefix":     account.BucketPrefix,
			"report_name":       account.ReportName,
			"sync_cron":         account.SyncCron,
			"notify_enabled":    account.NotifyEnabled,
			"notify_channel_id": account.NotifyChannelID,
			"status":            account.Status,
			"last_import_at":    account.LastImportAt,
			"last_import_error": account.LastImportError,
			"created_at":        account.CreatedAt,
			"updated_at":        account.UpdatedAt,
			"details": CloudAccountDetails{
				Cost:          yearCosts[account.ID],
				Forecast:      0,
				Resources:     resources[account.ID],
				LastMonthCost: monthCosts[account.ID],
			},
		}
		result = append(result, item)
	}

	return result, nil
}

// GetCloudAccountWithDetails 获取单个云账户详情（含费用）
func (s *BillService) GetCloudAccountWithDetails(id uint) (map[string]interface{}, error) {
	account, err := s.cloudRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	thisYear := now.Format("2006")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	cost, _ := s.repo.GetCostByCloudAccountYear(id, thisYear)
	lastMonthCost, _ := s.repo.GetCostByCloudAccount(id, lastMonth)
	resources, _ := s.repo.GetResourceCountByCloudAccount(id)

	item := map[string]interface{}{
		"id":                account.ID,
		"name":              account.Name,
		"cloud_type":        account.CloudType,
		"access_key_id":     account.AccessKeyID,
		"account_id":        account.AccountID,
		"region":            account.Region,
		"bucket_name":       account.BucketName,
		"bucket_prefix":     account.BucketPrefix,
		"report_name":       account.ReportName,
		"sync_cron":         account.SyncCron,
		"notify_enabled":    account.NotifyEnabled,
		"notify_channel_id": account.NotifyChannelID,
		"status":            account.Status,
		"last_import_at":    account.LastImportAt,
		"last_import_error": account.LastImportError,
		"created_at":        account.CreatedAt,
		"updated_at":        account.UpdatedAt,
		"details": CloudAccountDetails{
			Cost:          cost,
			Forecast:      0,
			Resources:     resources,
			LastMonthCost: lastMonthCost,
		},
	}

	return item, nil
}

// ValidateCloudAccount 验证云账户凭证
func (s *BillService) ValidateCloudAccount(cloudType string, config map[string]interface{}) (map[string]interface{}, error) {
	adapter, err := cloud.NewCloudAdapter(cloudType, config)
	if err != nil {
		return nil, err
	}

	info, err := adapter.ValidateCredentials()
	if err != nil {
		return nil, err
	}

	return info, nil
}

// GetResource 云资源列表（bill_resources）
func (s *BillService) GetResource(vendor string, page, pageSize int) (interface{}, error) {
	total, resources, err := s.repo.ListBillResources(vendor, page, pageSize)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total":     total,
		"resources": resources,
	}, nil
}
