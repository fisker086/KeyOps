package bill

import (
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
)

// CloudAccountDetails 云账户费用详情
type CloudAccountDetails struct {
	Cost          float64 `json:"cost"`
	Forecast      float64 `json:"forecast,omitempty"`
	Resources     int     `json:"resources"`
	LastMonthCost float64 `json:"last_month_cost"`
}

// AddCloudAccount 添加云账户
func (s *BillService) AddCloudAccount(account *model.CloudAccount) error {
	if account.CloudType == cloud.CloudTypeAWS {
		if strings.TrimSpace(account.BucketName) == "" {
			return fmt.Errorf("AWS billing requires bucket_name for CUR S3 import")
		}
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

// ListCloudAccountsWithDetails 列出云账户（含费用详情）
func (s *BillService) ListCloudAccountsWithDetails(cloudType string) ([]map[string]interface{}, error) {
	accounts, err := s.cloudRepo.List(cloudType)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(accounts))
	now := time.Now()
	thisYear := now.Format("2006")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

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
			"details":           nil,
		}

		cost, _ := s.repo.GetCostByCloudAccountYear(account.ID, thisYear)
		lastMonthCost, _ := s.repo.GetCostByCloudAccount(account.ID, lastMonth)
		resources, _ := s.repo.GetResourceCountByCloudAccount(account.ID)

		details := CloudAccountDetails{
			Cost:          cost,
			Forecast:      0,
			Resources:     resources,
			LastMonthCost: lastMonthCost,
		}

		item["details"] = details
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
