package bill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BillService struct {
	repo             *repository.BillRepository
	cloudRepo        *repository.CloudAccountRepository
	alertChannelRepo *repository.AlertChannelRepository
	mongoColl        *mongo.Collection // MongoDB raw_expenses 集合
	notifyURL        string
	syncScheduler    *SyncScheduler
}

func NewBillService(repo *repository.BillRepository, cloudRepo *repository.CloudAccountRepository, alertChannelRepo *repository.AlertChannelRepository, mongoColl *mongo.Collection) *BillService {
	return &BillService{repo: repo, cloudRepo: cloudRepo, alertChannelRepo: alertChannelRepo, mongoColl: mongoColl, notifyURL: "http://localhost:8080/api/notify/plain"}
}

func (s *BillService) SetSyncScheduler(sch *SyncScheduler) {
	s.syncScheduler = sch
}

func (s *BillService) SetNotifyURL(url string) {
	if url != "" {
		s.notifyURL = url
	}
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
	if price.Spec != "" {
		existing.Spec = price.Spec
	}
	if price.Description != "" {
		existing.Description = price.Description
	}
	if !price.UnitPrice.IsZero() {
		existing.UnitPrice = price.UnitPrice
	}
	if price.Currency != "" {
		existing.Currency = price.Currency
	}
	if price.Unit != "" {
		existing.Unit = price.Unit
	}
	if price.Region != "" {
		existing.Region = price.Region
	}
	if price.EffectiveDate != "" {
		existing.EffectiveDate = price.EffectiveDate
	}

	if err := s.repo.UpdatePrice(id, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeletePrice 删除单价
func (s *BillService) DeletePrice(id string) error {
	if _, err := s.repo.GetPriceByID(id); err != nil {
		return fmt.Errorf("单价不存在: %v", err)
	}
	return s.repo.DeletePrice(id)
}

// SyncCloudBilling 同步云账单（流式处理，避免OOM）
func (s *BillService) SyncCloudBilling(cloudAccountID uint, billingDate time.Time) error {
	billingDateStr := billingDate.Format("2006-01")
	log.Printf("[BillSync] starting sync: cloudAccountID=%d, billingDate=%s", cloudAccountID, billingDateStr)

	if s.mongoColl == nil {
		return fmt.Errorf("mongodb not initialized, cloud bill sync is unavailable")
	}

	cloudAccount, err := s.cloudRepo.GetByID(cloudAccountID)
	if err != nil {
		return fmt.Errorf("failed to get cloud account: %w", err)
	}

	log.Printf("[BillSync] cloudAccount: id=%d, cloudType=%s, bucketName=%s, reportName=%s, prefix=%s, region=%s",
		cloudAccount.ID, cloudAccount.CloudType, cloudAccount.BucketName, cloudAccount.ReportName, cloudAccount.BucketPrefix, cloudAccount.Region)

	config := map[string]interface{}{
		"access_key_id":     cloudAccount.AccessKeyID,
		"secret_access_key": cloudAccount.SecretAccessKey,
		"region":            cloudAccount.Region,
		"bucket_name":       cloudAccount.BucketName,
		"bucket_prefix":     cloudAccount.BucketPrefix,
		"report_name":       cloudAccount.ReportName,
	}

	adapter, err := cloud.NewCloudAdapter(cloudAccount.CloudType, config)
	if err != nil {
		return fmt.Errorf("failed to create cloud adapter: %w", err)
	}

	if _, err = adapter.ValidateCredentials(); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		s.sendSyncNotification(cloudAccount, billingDateStr, err)
		return fmt.Errorf("credential validation failed: %w", err)
	}

	if cloudAccount.CloudType == cloud.CloudTypeAWS {
		if cloudAccount.BucketName == "" {
			err := fmt.Errorf("AWS billing requires bucket_name for CUR S3 import")
			s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
			s.sendSyncNotification(cloudAccount, billingDateStr, err)
			return err
		}
	}

	log.Printf("[BillSync] credentials validated, setting status to syncing...")
	s.updateCloudAccountStatus(cloudAccountID, "syncing", "")

	// 流式获取数据（边读边写，避免OOM）
	ctx := context.Background()
	dataCh, errCh := adapter.StreamRawBillingData(ctx, billingDate)

	// 先删旧数据
	reportIdentity := fmt.Sprintf("%d_%s", cloudAccountID, billingDateStr)
	now := time.Now().UTC()
	mongoScope := map[string]interface{}{
		model.MetaCloudAccountID: cloudAccountID,
		model.MetaReportIdentity: reportIdentity,
	}
	_, _ = s.mongoColl.DeleteMany(ctx, mongoScope)

	// 若后续流式写入或 MySQL 落库失败，避免 Mongo 里残留半批数据（与 MySQL 旧数据并存造成误解）
	mysqlBillCommitted := false
	var syncErr error
	defer func() {
		if mysqlBillCommitted || s.mongoColl == nil {
			return
		}
		_, _ = s.mongoColl.DeleteMany(context.Background(), mongoScope)
	}()
	defer func() {
		if syncErr != nil {
			s.sendSyncNotification(cloudAccount, billingDateStr, syncErr)
		} else {
			s.sendSyncNotification(cloudAccount, billingDateStr, nil)
		}
	}()

	// 边读边写MongoDB，同时标准化为 MySQL 查询模型
	var total int
	records := make([]model.BillRecord, 0, 4096)
	resourceMap := make(map[string]model.BillResource)
	for batch := range dataCh {
		docs := make([]interface{}, 0, len(batch))
		for _, row := range batch {
			row[model.MetaCloudAccountID] = cloudAccountID
			row[model.MetaReportIdentity] = reportIdentity
			row[model.MetaCreatedAt] = now
			docs = append(docs, row)

			record, resource := s.normalizeRawBillingRow(cloudAccount, row, billingDate)
			if record != nil {
				records = append(records, *record)
			}
			if resource != nil && resource.ResourceID != "" {
				resourceMap[billResourceKey(resource)] = *resource
			}
		}

		if len(docs) > 0 {
			if _, err := s.mongoColl.InsertMany(ctx, docs); err != nil {
				s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
				syncErr = fmt.Errorf("insert to MongoDB failed: %w", err)
				return syncErr
			}
			total += len(docs)
		}

		// 检查错误
		select {
		case err := <-errCh:
			if err != nil {
				s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
				syncErr = fmt.Errorf("stream failed: %w", err)
				return syncErr
			}
		default:
		}
	}
	if err := <-errCh; err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("stream failed: %w", err)
		return syncErr
	}

	resources := make([]model.BillResource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, resource)
	}
	if err := s.repo.ReplaceBillingRecordsForAccount(cloudAccountID, billingDateStr, records); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("normalize billing records failed: %w", err)
		return syncErr
	}
	mysqlBillCommitted = true
	if err := s.repo.UpsertBillResources(resources); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("upsert bill resources failed: %w", err)
		return syncErr
	}
	if err := s.repo.RebuildSummary(cloudAccount.CloudType, billingDateStr); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("rebuild bill summary failed: %w", err)
		return syncErr
	}

	s.updateCloudAccountStatus(cloudAccountID, "active", "")
	if err := s.cloudRepo.UpdateLastImport(cloudAccountID); err != nil {
		syncErr = err
		return err
	}
	log.Printf("[BillSync] synced %d raw records to MongoDB, %d bill records, %d resources for account %d", total, len(records), len(resources), cloudAccountID)
	return nil
}

// SyncCloudBillingAsync 异步同步账单，启动 goroutine 后立即返回
func (s *BillService) SyncCloudBillingAsync(cloudAccountID uint, billingDate time.Time) {
	go func() {
		log.Printf("[BillSync] starting async sync: account=%d, billingDate=%s", cloudAccountID, billingDate.Format("2006-01"))
		if err := s.SyncCloudBilling(cloudAccountID, billingDate); err != nil {
			log.Printf("[BillSync] async sync failed: account=%d, err=%v", cloudAccountID, err)
		}
	}()
}

// SyncCloudBillingRange 异步同步多个月份
func (s *BillService) SyncCloudBillingRange(cloudAccountID uint, startDate, endDate time.Time) {
	go func() {
		current := startDate
		for !current.After(endDate) {
			log.Printf("[BillSync] async range sync: account=%d, month=%s", cloudAccountID, current.Format("2006-01"))
			if err := s.SyncCloudBilling(cloudAccountID, current); err != nil {
				log.Printf("[BillSync] async range sync failed: account=%d, month=%s, err=%v", cloudAccountID, current.Format("2006-01"), err)
				return
			}
			current = current.AddDate(0, 1, 0)
		}
	}()
}

func (s *BillService) normalizeRawBillingRow(account *model.CloudAccount, row map[string]interface{}, billingDate time.Time) (*model.BillRecord, *model.BillResource) {
	cost := firstDecimal(row,
		"Line_item_net_unblended_cost",
		"lineItem/NetUnblendedCost",
		"Line_item_unblended_cost",
		"lineItem/UnblendedCost",
		"Line_item_blended_cost",
		"lineItem/BlendedCost",
		"Line_item_net_amortized_cost",
		"lineItem/NetAmortizedCost",
		"Line_item_amortized_cost",
		"lineItem/AmortizedCost",
		"cost",
		"amount",
	)

	// SavingsPlanCoveredUsage: 用量 × SP 折扣率 = 实际应付
	lineItemType := firstString(row, "Line_item_line_item_type", "lineItem/LineItemType", "lineItemType")
	if lineItemType == "SavingsPlanCoveredUsage" {
		spRateStr := firstString(row, "savings_plan_savings_plan_rate", "savingsPlan/SavingsPlanRate")
		if spRateStr != "" {
			if spRate, err := decimal.NewFromString(spRateStr); err == nil && spRate.GreaterThan(decimal.Zero) {
				usage := firstDecimal(row,
					"Line_item_usage_amount", "lineItem/UsageAmount", "usage_amount",
				)
				cost = usage.Mul(spRate)
			}
		}
	}

	instanceID := firstString(row,
		"Line_item_resource_id",
		"lineItem/ResourceId",
		"resourceId",
		"ResourceId",
		"resource_id",
	)
	serviceCode := firstString(row,
		"Line_item_product_code",
		"lineItem/ProductCode",
		"product/ProductCode",
		"ProductCode",
		"product_code",
	)
	serviceType := firstString(row,
		"ProductName",
		"product_name",
		"Product_servicename",
		"product/servicename",
		"product/servicecode",
		"product/ServiceName",
		"service_name",
	)
	resourceType := firstNonEmpty(
		firstString(row,
			"ProductDetail",
			"product_detail",
			"Product_product_family",
			"product/productFamily",
			"product/ProductFamily",
			"resource_type",
		),
		serviceCode,
	)
	region := firstNonEmpty(
		firstString(row, "Product_region", "product/region", "product/Region", "Line_item_availability_zone", "lineItem/AvailabilityZone", "availabilityZone", "region"),
		account.Region,
	)
	tags := extractUserTags(row)
	tagsJSON := encodeStringMap(tags)

	displayName := firstString(row,
		"Resource_tags_user_name",
		"resourceTags/user:Name",
		"resourceTags/user:name",
		"resource_name",
		"NickName",
		"nick_name",
		"InstanceName",
		"instanceName",
		"HostName",
		"hostname",
		"ItemName",
		"item_name",
	)

	productName := firstString(row, "ProductName", "product_name")
	productDetail := firstString(row, "ProductDetail", "product_detail")

	extra := map[string]interface{}{
		"report_identity": reportIdentityValue(row),
		"line_item_type":  firstString(row, "Line_item_line_item_type", "lineItem/LineItemType", "LineItemType"),
		"usage_type":      firstString(row, "Line_item_usage_type", "lineItem/UsageType", "UsageType"),
		"operation":       firstString(row, "Line_item_operation", "lineItem/Operation", "Operation"),
		"currency":        firstString(row, "Line_item_currency_code", "lineItem/CurrencyCode", "currency"),
		"product_name":    productName,
		"product_detail":  productDetail,
	}

	specDesc := firstString(row,
		"Line_item_usage_type",
		"lineItem/UsageType",
		"UsageType",
		"InstanceSpec",
		"instance_spec",
	)

	resourceName := firstNonEmpty(displayName, productDetail, productName, instanceID)

	record := &model.BillRecord{
		Vendor:         account.CloudType,
		Cycle:          billingDate.Format("2006-01"),
		InstanceID:     instanceID,
		ResourceName:   resourceName,
		SpecDesc:       specDesc,
		ConsumeAmount:  cost,
		ResourceType:   resourceType,
		ResourceCode:   resourceType,
		ServiceType:    serviceType,
		ServiceCode:    serviceCode,
		Region:         region,
		AccountID:      firstNonEmpty(firstString(row, "Line_item_usage_account_id", "lineItem/UsageAccountId", "bill/PayerAccountId"), account.AccountID),
		CloudAccountID: account.ID,
		Tags:           tagsJSON,
		Extra:          encodeMap(extra),
	}

	if instanceID == "" {
		return record, nil
	}

	resource := &model.BillResource{
		Vendor:         account.CloudType,
		CloudAccountID: account.ID,
		AccountID:      record.AccountID,
		ResourceID:     instanceID,
		ResourceType:   resourceType,
		ResourceName:   firstNonEmpty(strings.TrimSpace(displayName), productDetail, productName),
		InstanceType:   firstString(row, "Product_instance_type", "product/instanceType", "product/InstanceType", "instance_type"),
		Region:         region,
		Zone:           firstString(row, "Line_item_availability_zone", "lineItem/AvailabilityZone", "availabilityZone"),
		Tags:           tagsJSON,
		Status:         "active",
		FirstSeen:      billingDate,
		LastSeen:       billingDate,
	}
	return record, resource
}

func billResourceKey(resource *model.BillResource) string {
	return fmt.Sprintf("%d:%s", resource.CloudAccountID, resource.ResourceID)
}

func firstString(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return strings.TrimSpace(v.String())
				}
			default:
				text := strings.TrimSpace(fmt.Sprintf("%v", v))
				if text != "" && text != "<nil>" {
					return text
				}
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstDecimal(row map[string]interface{}, keys ...string) decimal.Decimal {
	for _, key := range keys {
		raw := firstString(row, key)
		if raw == "" {
			continue
		}
		if d, err := decimal.NewFromString(raw); err == nil {
			return d
		}
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return decimal.NewFromFloat(f)
		}
	}
	return decimal.Zero
}

func extractUserTags(row map[string]interface{}) map[string]string {
	tags := make(map[string]string)
	for key, value := range row {
		tagKey := ""
		if strings.HasPrefix(key, "resourceTags/user:") {
			tagKey = strings.TrimPrefix(key, "resourceTags/user:")
		} else if strings.HasPrefix(key, "Resource_tags_user_") {
			tagKey = strings.TrimPrefix(key, "Resource_tags_user_")
		}
		if tagKey == "" {
			continue
		}
		tagValue := firstString(map[string]interface{}{key: value}, key)
		if tagKey != "" && tagValue != "" {
			tags[tagKey] = tagValue
		}
	}
	return tags
}

func encodeStringMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(m))
	for _, key := range keys {
		if value := strings.TrimSpace(m[key]); value != "" {
			ordered[key] = value
		}
	}
	if len(ordered) == 0 {
		return ""
	}
	b, _ := json.Marshal(ordered)
	return string(b)
}

func encodeMap(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]interface{}, len(m))
	for _, key := range keys {
		if value, ok := m[key]; ok && value != "" {
			ordered[key] = value
		}
	}
	if len(ordered) == 0 {
		return ""
	}
	b, _ := json.Marshal(ordered)
	return string(b)
}

func reportIdentityValue(row map[string]interface{}) string {
	return firstString(row, model.MetaReportIdentity)
}

// GetCloudPricing 获取云资源定价
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

// updateCloudAccountStatus 更新云账户状态
func (s *BillService) updateCloudAccountStatus(id uint, status, errorMsg string) {
	log.Printf("[BillSync] updating account status: id=%d, status=%s, error=%s", id, status, errorMsg)
	acc, err := s.cloudRepo.GetByID(id)
	if err != nil {
		log.Printf("[BillSync] failed to get account for status update: id=%d, err=%v", id, err)
		return
	}
	acc.Status = status
	if errorMsg != "" {
		acc.LastImportError = errorMsg
	} else {
		acc.LastImportError = ""
	}
	if err := s.cloudRepo.Update(acc); err != nil {
		log.Printf("[BillSync] failed to update account status: id=%d, err=%v", id, err)
	}
}

// sendSyncNotification 发送账单同步完成通知
func (s *BillService) sendSyncNotification(account *model.CloudAccount, billingDate string, syncErr error) {
	if (account.NotifyEnabled == nil || !*account.NotifyEnabled) || account.NotifyChannelID == 0 {
		return
	}
	channel, err := s.alertChannelRepo.FindByID(account.NotifyChannelID)
	if err != nil || channel == nil {
		log.Printf("[BillSync] failed to load alert channel %d for account %d: %v", account.NotifyChannelID, account.ID, err)
		return
	}

	var channelConfig map[string]interface{}
	if err := json.Unmarshal([]byte(channel.ChannelSign), &channelConfig); err != nil {
		channelConfig = map[string]interface{}{
			"webhook": channel.ChannelSign,
		}
	}
	webhookURL, _ := channelConfig["webhook"].(string)
	secret, _ := channelConfig["secret"].(string)
	if webhookURL == "" {
		log.Printf("[BillSync] webhook URL is empty for channel %d", channel.ID)
		return
	}

	title := fmt.Sprintf("账单同步: %s", account.Name)
	var content string
	if syncErr != nil {
		title = "❌ " + title
		content = fmt.Sprintf(
			"**云账户**: %s\n**云类型**: %s\n**账单月份**: %s\n**状态**: 同步失败\n**错误**: %s",
			account.Name, account.CloudType, billingDate, syncErr.Error(),
		)
	} else {
		title = "✅ " + title
		content = fmt.Sprintf(
			"**云账户**: %s\n**云类型**: %s\n**账单月份**: %s\n**状态**: 同步成功",
			account.Name, account.CloudType, billingDate,
		)
	}

	var n notification.Notifier
	switch channel.ChannelType {
	case "feishu":
		n = notification.NewFeishuNotifier(webhookURL, secret)
	case "dingtalk":
		n = notification.NewDingTalkNotifier(webhookURL, secret)
	case "wechat":
		n = notification.NewWeChatNotifier(webhookURL)
	default:
		log.Printf("[BillSync] unsupported channel type %s for channel %d", channel.ChannelType, channel.ID)
		return
	}
	if err := n.SendAlert(title, content); err != nil {
		log.Printf("[BillSync] failed to send sync notification for account %d: %v", account.ID, err)
	}
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

func (s *BillService) reloadSyncScheduler() {
	if s.syncScheduler != nil {
		s.syncScheduler.Reload()
	}
}

// GetBillingSummaryByCloud 按云厂商汇总账单
func (s *BillService) GetBillingSummaryByCloud(startDate, endDate time.Time) (map[string]decimal.Decimal, error) {
	return s.repo.GetSummaryByCloud(startDate, endDate)
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

// GetBreakdownByTags 按标签分解费用
func (s *BillService) GetBreakdownByTags(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByTags(vendor, month)
}

// GetBreakdownByAccounts 按云账户分解费用
func (s *BillService) GetBreakdownByAccounts(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByAccounts(vendor, month)
}

// GetBreakdownByRegion 按区域分解费用
func (s *BillService) GetBreakdownByRegion(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByRegion(vendor, month)
}

// GetBreakdownByService 按服务分解费用
func (s *BillService) GetBreakdownByService(vendor, month string) (interface{}, error) {
	return s.repo.GetBreakdownByService(vendor, month)
}

// GetRegionExpenses 按区域聚合费用
func (s *BillService) GetRegionExpenses(startDate, endDate time.Time) (interface{}, error) {
	return s.repo.GetRegionExpenses(startDate, endDate)
}

// GetTrafficExpenses 按流量聚合费用（目前使用 region 聚合作为占位，后续可接 CUR 流量字段）
func (s *BillService) GetTrafficExpenses(startDate, endDate time.Time, resourceID string) (interface{}, error) {
	return s.repo.GetTrafficExpenses(startDate, endDate, resourceID)
}

// CloudAccountDetails 云账户费用详情
type CloudAccountDetails struct {
	Cost          float64 `json:"cost"`
	Forecast      float64 `json:"forecast,omitempty"`
	Resources     int     `json:"resources"`
	LastMonthCost float64 `json:"last_month_cost"`
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

// GetExpensesBreakdown 获取费用分解
func (s *BillService) GetExpensesBreakdown(startDate, endDate time.Time, granularity, groupBy, vendor, serviceCode, keyword string) (*repository.BreakdownResult, error) {
	if granularity == "daily" {
		return s.getMongoDailyBreakdown(startDate, endDate, groupBy, vendor)
	}
	return s.repo.GetExpensesBreakdown(startDate, endDate, granularity, groupBy, vendor, serviceCode, keyword)
}

// getMongoDailyBreakdown 从 MongoDB 按日聚合费用（用于日粒度图表）
func (s *BillService) getMongoDailyBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*repository.BreakdownResult, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized, daily breakdown unavailable")
	}

	ctx := context.Background()
	pipeline := mongo.Pipeline{}

	// match 阶段
	match := bson.D{{Key: "$match", Value: bson.M{}}}
	if vendor != "" {
		match[0].Value.(bson.M)["lineItem/ProductCode"] = vendor
	}
	dateFilter := bson.M{}
	if !startDate.IsZero() {
		dateFilter["$gte"] = startDate.Format("2006-01-02")
	}
	if !endDate.IsZero() {
		dateFilter["$lte"] = endDate.Format("2006-01-02")
	}
	if len(dateFilter) > 0 {
		match[0].Value.(bson.M)["lineItem/UsageStartDate"] = dateFilter
	}
	pipeline = append(pipeline, match)

	// group 阶段：按日期 + groupBy 字段聚合 cost
	groupField := "$lineItem/ProductCode"
	dateFormat := "%Y-%m-%d"
	switch groupBy {
	case "region":
		groupField = "$product/region"
	case "service_name", "service_code":
		groupField = "$lineItem/ProductCode"
	case "cloud_type":
		groupField = "$lineItem/ProductCode"
	}

	groupID := bson.D{
		{Key: "date", Value: bson.D{{Key: "$dateToString", Value: bson.D{
			{Key: "format", Value: dateFormat},
			{Key: "date", Value: "$lineItem/UsageStartDate"},
		}}}},
		{Key: "group", Value: groupField},
	}
	groupStage := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "totalCost", Value: bson.D{{Key: "$sum", Value: "$lineItem/BlendedCost"}}},
		}},
	}
	pipeline = append(pipeline, groupStage)

	// sort by date
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "_id.date", Value: 1}}},
	})

	cur, err := s.mongoColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	breakdown := make(map[string]map[string]float64)
	totals := make(map[string]float64)

	type mongoDoc struct {
		ID        struct {
			Date  string `bson:"date"`
			Group string `bson:"group"`
		} `bson:"_id"`
		TotalCost float64 `bson:"totalCost"`
	}

	for cur.Next(ctx) {
		var doc mongoDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		date := doc.ID.Date
		group := doc.ID.Group
		if group == "" {
			group = "unknown"
		}
		if breakdown[date] == nil {
			breakdown[date] = make(map[string]float64)
		}
		breakdown[date][group] += doc.TotalCost
		totals[group] += doc.TotalCost
	}

	return &repository.BreakdownResult{
		Breakdown:   breakdown,
		Totals:      totals,
		Granularity: "daily",
		GroupBy:     groupBy,
	}, nil
}

// GetResourceCountBreakdown 获取资源数量分解
func (s *BillService) GetResourceCountBreakdown(startDate, endDate time.Time, groupBy, vendor string) (*repository.BreakdownResult, error) {
	return s.repo.GetResourceCountBreakdown(startDate, endDate, groupBy, vendor)
}

type VendorCostEntry struct {
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`
}

type DashboardData struct {
	CurrentMonthCost float64                      `json:"current_month_cost"`
	LastMonthCost    float64                      `json:"last_month_cost"`
	ForecastCost     float64                      `json:"forecast_cost"`
	ChangePercent    float64                      `json:"change_percent"`
	BaseCurrency     string                       `json:"base_currency"`
	TopResources     []TopResource                `json:"top_resources"`
	CostByVendor     map[string]VendorCostEntry   `json:"cost_by_vendor"`
	CostByService    map[string]float64           `json:"cost_by_service"`
	CostByRegion     map[string]float64           `json:"cost_by_region"`
	CostTrend        []DailyCost                  `json:"cost_trend"`
}

type TopResource struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ServiceCode  string  `json:"service_code"`
	Cost         float64 `json:"cost"`
	Vendor       string  `json:"vendor"`
	Currency     string  `json:"currency"`
}

type DailyCost struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// GetDashboardData 获取 Dashboard 数据
// baseCurrency: "CNY" or "USD" — 聚合图表统一转为此币种显示
func (s *BillService) GetDashboardData(baseCurrency string) (*DashboardData, error) {
	now := time.Now()
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	type queryResult struct {
		currentCost   float64
		lastCost      float64
		topResources  []map[string]interface{}
		costByVendor  map[string]repository.VendorCost
		costByService map[string]float64
		costByRegion  map[string]float64
		costTrend     []map[string]interface{}
		errs          []error
	}

	var res queryResult
	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		res.currentCost, _ = s.repo.GetTotalCostByMonth(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.lastCost, _ = s.repo.GetTotalCostByMonth(lastMonth)
	}()
	go func() {
		defer wg.Done()
		res.topResources, _ = s.repo.GetTopResources(thisMonth, 10)
	}()
	go func() {
		defer wg.Done()
		res.costByVendor, _ = s.repo.GetCostByVendor(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costByService, _ = s.repo.GetCostByService(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costByRegion, _ = s.repo.GetCostByRegion(thisMonth)
	}()
	go func() {
		defer wg.Done()
		res.costTrend, _ = s.repo.GetDailyCostTrend(now.AddDate(0, -30, 0), now)
	}()

	wg.Wait()

	if baseCurrency == "" {
		baseCurrency = "CNY"
	}
	currencyFromUSD := func(v float64) float64 {
		if baseCurrency == "CNY" {
			return v * 7.2
		}
		return v
	}

	var changePercent float64
	if res.lastCost > 0 {
		changePercent = ((res.currentCost - res.lastCost) / res.lastCost) * 100
	}

	_, monthEnd := getMonthRange(now)
	daysInMonth := monthEnd.Day()
	forecastCost := res.currentCost
	if now.Day() > 0 {
		forecastCost = res.currentCost / float64(now.Day()) * float64(daysInMonth)
	}

	topResourcesResult := make([]TopResource, 0)
	for _, m := range res.topResources {
		tr := TopResource{
			ResourceID:   getString(m, "resource_id"),
			ResourceName: getString(m, "resource_name"),
			ServiceCode:  getString(m, "service_code"),
			Cost:         getFloat(m, "cost"),
			Vendor:       getString(m, "vendor"),
			Currency:     getString(m, "currency"),
		}
		topResourcesResult = append(topResourcesResult, tr)
	}

	costTrendResult := make([]DailyCost, 0)
	for _, m := range res.costTrend {
		dc := DailyCost{
			Date: getString(m, "date"),
			Cost: currencyFromUSD(getFloat(m, "cost")),
		}
		costTrendResult = append(costTrendResult, dc)
	}

	vendorCost := make(map[string]VendorCostEntry, len(res.costByVendor))
	for v, vc := range res.costByVendor {
		vendorCost[v] = VendorCostEntry{Cost: vc.Cost, Currency: vc.Currency}
	}

	// 转换聚合数据到目标基币
	costByService := make(map[string]float64, len(res.costByService))
	for k, v := range res.costByService {
		costByService[k] = currencyFromUSD(v)
	}
	costByRegion := make(map[string]float64, len(res.costByRegion))
	for k, v := range res.costByRegion {
		costByRegion[k] = currencyFromUSD(v)
	}

	return &DashboardData{
		CurrentMonthCost: currencyFromUSD(res.currentCost),
		LastMonthCost:    currencyFromUSD(res.lastCost),
		ForecastCost:     currencyFromUSD(forecastCost),
		ChangePercent:    changePercent,
		BaseCurrency:     baseCurrency,
		TopResources:     topResourcesResult,
		CostByVendor:     vendorCost,
		CostByService:    costByService,
		CostByRegion:     costByRegion,
		CostTrend:        costTrendResult,
	}, nil
}

type Recommendation struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Savings     float64          `json:"savings"`
	Resources   int              `json:"resources"`
	Priority    string           `json:"priority"`
	Items       []RecommendationItem `json:"items,omitempty"`
}

type RecommendationItem struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	Vendor       string  `json:"vendor"`
	Cost         float64 `json:"cost"`
	Region       string  `json:"region"`
	Currency     string  `json:"currency"`
}

const maxRecommendationItems = 20

func truncateRecommendationItems(items []RecommendationItem) []RecommendationItem {
	if len(items) <= maxRecommendationItems {
		return items
	}
	return items[:maxRecommendationItems]
}

// GetRecommendations 获取优化建议
func (s *BillService) GetRecommendations() ([]Recommendation, error) {
	recommendations := []Recommendation{}

	var idleResources []repository.IdleResource
	var largeResources []repository.IdleResource
	var idleErr, largeErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		idleResources, idleErr = s.repo.GetIdleResources()
	}()
	go func() {
		defer wg.Done()
		largeResources, largeErr = s.repo.GetLargeResources()
	}()
	wg.Wait()

	_ = idleErr
	_ = largeErr

	// 1. 闲置资源
	if len(idleResources) > 0 {
		idleCost := 0.0
		items := make([]RecommendationItem, 0, len(idleResources))
		for _, r := range idleResources {
			idleCost += r.Cost
			items = append(items, RecommendationItem{
				ResourceID:   r.ResourceID,
				ResourceName: r.ResourceName,
				Vendor:       r.Vendor,
				Cost:         r.Cost,
				Region:       r.Region,
				Currency:     r.Currency,
			})
		}
		recommendations = append(recommendations, Recommendation{
			ID:          "idle_resources",
			Type:        "idle",
			Title:       "闲置资源",
			Description: "检测到未使用的资源，可考虑释放以节省成本",
			Savings:     idleCost,
			Resources:   len(idleResources),
			Priority:    "high",
			Items:       truncateRecommendationItems(items),
		})
	}

	// 2. 大规格资源
	if len(largeResources) > 0 {
		largeCost := 0.0
		items := make([]RecommendationItem, 0, len(largeResources))
		for _, r := range largeResources {
			largeCost += r.Cost
			items = append(items, RecommendationItem{
				ResourceID:   r.ResourceID,
				ResourceName: r.ResourceName,
				Vendor:       r.Vendor,
				Cost:         r.Cost,
				Region:       r.Region,
				Currency:     r.Currency,
			})
		}
		recommendations = append(recommendations, Recommendation{
			ID:          "rightsizing",
			Type:        "rightsizing",
			Title:       "大规格资源",
			Description: "检测到大规格资源（月均费用 > 500），建议评估是否可降配",
			Savings:     largeCost * 0.3,
			Resources:   len(largeResources),
			Priority:    "medium",
			Items:       truncateRecommendationItems(items),
		})
	}

	return recommendations, nil
}

// ============ Budgets ============

func (s *BillService) ListBudgets() ([]model.Budget, error) {
	return s.repo.ListBudgets()
}

func (s *BillService) CreateBudget(budget *model.Budget) (*model.Budget, error) {
	budget.Status = "active"
	err := s.repo.CreateBudget(budget)
	return budget, err
}

func (s *BillService) UpdateBudget(id uint, budget *model.Budget) (*model.Budget, error) {
	existing, err := s.repo.GetBudgetByID(id)
	if err != nil {
		return nil, err
	}
	if budget.Name != "" {
		existing.Name = budget.Name
	}
	if budget.Amount > 0 {
		existing.Amount = budget.Amount
	}
	if budget.Period != "" {
		existing.Period = budget.Period
	}
	if budget.AlertThreshold > 0 {
		existing.AlertThreshold = budget.AlertThreshold
	}
	if budget.Status != "" {
		existing.Status = budget.Status
	}
	err = s.repo.UpdateBudget(existing)
	return existing, err
}

func (s *BillService) DeleteBudget(id uint) error {
	return s.repo.DeleteBudget(id)
}

// ============ Pools ============

func (s *BillService) ListPools() ([]map[string]interface{}, error) {
	pools, err := s.repo.ListPools()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(pools))
	for _, pool := range pools {
		currentCost, _ := s.repo.GetPoolCost(pool.Members)
		item := map[string]interface{}{
			"id":           pool.ID,
			"name":         pool.Name,
			"description":  pool.Description,
			"limit_amount": pool.LimitAmount,
			"owner":        pool.Owner,
			"status":       pool.Status,
			"current_cost": currentCost,
			"members":      pool.Members,
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *BillService) CreatePool(pool *model.Pool) (*model.Pool, error) {
	pool.Status = "active"
	if pool.Members == "" {
		pool.Members = "[]"
	}
	err := s.repo.CreatePool(pool)
	return pool, err
}

func (s *BillService) UpdatePool(id uint, pool *model.Pool) (*model.Pool, error) {
	existing, err := s.repo.GetPoolByID(id)
	if err != nil {
		return nil, err
	}
	if pool.Name != "" {
		existing.Name = pool.Name
	}
	if pool.Description != "" {
		existing.Description = pool.Description
	}
	if pool.LimitAmount > 0 {
		existing.LimitAmount = pool.LimitAmount
	}
	err = s.repo.UpdatePool(existing)
	return existing, err
}

func (s *BillService) DeletePool(id uint) error {
	return s.repo.DeletePool(id)
}

// ============ Policies ============

func (s *BillService) ListPolicies() ([]model.Policy, error) {
	return s.repo.ListPolicies()
}

func (s *BillService) CreatePolicy(policy *model.Policy) (*model.Policy, error) {
	policy.Enabled = true
	err := s.repo.CreatePolicy(policy)
	return policy, err
}

func (s *BillService) UpdatePolicy(id uint, policy *model.Policy) (*model.Policy, error) {
	existing, err := s.repo.GetPolicyByID(id)
	if err != nil {
		return nil, err
	}
	if policy.Name != "" {
		existing.Name = policy.Name
	}
	if policy.Type != "" {
		existing.Type = policy.Type
	}
	if policy.Action != "" {
		existing.Action = policy.Action
	}
	if policy.Conditions != "" {
		existing.Conditions = policy.Conditions
	}
	existing.Enabled = policy.Enabled
	err = s.repo.UpdatePolicy(existing)
	return existing, err
}

func (s *BillService) DeletePolicy(id uint) error {
	return s.repo.DeletePolicy(id)
}
func getMonthRange(t time.Time) (time.Time, time.Time) {
	year := t.Year()
	month := t.Month()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)
	return firstDay, lastDay
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(json.Number); ok {
		f, _ := v.Float64()
		return f
	}
	return 0
}

// CheckBudgetAlerts 检查预算告警（可被定时任务调用）
// 返回触发告警的预算列表，并通过系统通知渠道发送告警
func (s *BillService) CheckBudgetAlerts() ([]map[string]interface{}, error) {
	budgets, err := s.repo.ListBudgets()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	alerts := make([]map[string]interface{}, 0)

	for _, budget := range budgets {
		if budget.Status != "active" {
			continue
		}

		// 计算当前周期的实际花费
		currentCost, err := s.getCurrentPeriodCost(budget, now)
		if err != nil {
			continue
		}

		// 计算使用百分比
		var usagePercent float64
		if budget.Amount > 0 {
			usagePercent = (currentCost / budget.Amount) * 100
		}

		// 检查是否超过阈值
		if usagePercent >= budget.AlertThreshold {
			alert := map[string]interface{}{
				"budget_id":         budget.ID,
				"budget_name":       budget.Name,
				"budget_amount":     budget.Amount,
				"current_cost":      currentCost,
				"usage_percent":     usagePercent,
				"threshold":         budget.AlertThreshold,
				"owner":             budget.Owner,
				"alert_channel_ids": budget.AlertChannelIDs,
			}
			alerts = append(alerts, alert)

			// 通过系统通知渠道发送告警
			if budget.AlertChannelIDs != "" {
				s.sendBudgetAlert(budget, alert)
			}
		}
	}

	return alerts, nil
}

// sendBudgetAlert 通过系统 AlertChannel 发送预算告警
func (s *BillService) sendBudgetAlert(budget model.Budget, alert map[string]interface{}) {
	// 解析通知渠道 ID 列表
	var channelIDs []uint
	if budget.AlertChannelIDs != "" {
		if err := json.Unmarshal([]byte(budget.AlertChannelIDs), &channelIDs); err != nil {
			log.Printf("[BudgetAlert] Failed to parse alert_channel_ids for budget %d: %v", budget.ID, err)
			return
		}
	}
	if len(channelIDs) == 0 {
		return
	}

	// 构建通知内容
	title := fmt.Sprintf("💰 预算告警: %s", budget.Name)
	content := fmt.Sprintf(
		"**预算名称**: %s\n**当前费用**: %.2f 元\n**预算金额**: %.2f 元\n**使用率**: %.1f%% (阈值: %.0f%%)\n**负责人**: %s",
		budget.Name,
		alert["current_cost"],
		budget.Amount,
		alert["usage_percent"],
		budget.AlertThreshold,
		budget.Owner,
	)

	notifyURL := "http://localhost:8080/api/notify/plain"
	if s.notifyURL != "" {
		notifyURL = s.notifyURL
	}
	message := map[string]interface{}{
		"channel_ids": channelIDs,
		"title":       title,
		"content":     content,
	}
	data, _ := json.Marshal(message)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(notifyURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[BudgetAlert] Failed to send notification for budget %d: %v", budget.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[BudgetAlert] Notification sent for budget: %s", budget.Name)
	} else {
		log.Printf("[BudgetAlert] Notification failed (status %d) for budget: %s", resp.StatusCode, budget.Name)
	}
}

// getCurrentPeriodCost 获取当前周期的花费
func (s *BillService) getCurrentPeriodCost(budget model.Budget, now time.Time) (float64, error) {
	var startDate, endDate string

	switch budget.Period {
	case "monthly":
		start, end := getMonthRange(now)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month())-1)/3 + 1
		startMonth := (quarter-1)*3 + 1
		start := time.Date(now.Year(), time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, -1)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	case "yearly":
		startDate = now.Format("2006") + "-01-01"
		endDate = now.Format("2006") + "-12-31"
	default:
		startDate = budget.StartDate
		endDate = budget.EndDate
	}

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	cycles := billingCyclesBetween(start, end)
	if len(cycles) == 0 {
		return 0, nil
	}

	var total float64
	for _, cycle := range cycles {
		cost, _ := s.repo.GetTotalCostByMonth(cycle)
		total += cost
	}

	return total, nil
}

// billingCyclesBetween 将起止日期转为包含的账单月份 cycle（YYYY-MM）
func billingCyclesBetween(startDate, endDate time.Time) []string {
	if startDate.After(endDate) {
		startDate, endDate = endDate, startDate
	}
	start := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	var cycles []string
	for d := start; !d.After(end); d = d.AddDate(0, 1, 0) {
		cycles = append(cycles, d.Format("2006-01"))
	}
	return cycles
}

// -----------------------------------------------------------------------------------
// MongoDB 查询方法（AWS 账单从 MongoDB 查询，减轻 MySQL 压力）
// -----------------------------------------------------------------------------------

// QueryMongoRaw 从 MongoDB 查询 AWS CUR 原始数据
// filters: 可选过滤条件，如 cloud_account_id、lineItem/ProductCode 等
// startAfter/endBefore: 可选时间范围（lineItem/UsageStartDate）
func (s *BillService) QueryMongoRaw(filters map[string]interface{}, startAfter, endBefore *time.Time, limit int) ([]map[string]interface{}, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized")
	}

	ctx := context.Background()
	query := bson.M{}
	for k, v := range filters {
		query[k] = v
	}
	if startAfter != nil {
		query["lineItem/UsageStartDate"] = bson.M{"$gte": startAfter.Format(time.RFC3339)}
	}
	if endBefore != nil {
		if _, ok := query["lineItem/UsageStartDate"]; !ok {
			query["lineItem/UsageStartDate"] = bson.M{}
		}
		query["lineItem/UsageStartDate"].(bson.M)["$lt"] = endBefore.Format(time.RFC3339)
	}

	findOptions := options.Find()
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	findOptions.SetSort(bson.D{{Key: "lineItem/UsageStartDate", Value: -1}})

	cur, err := s.mongoColl.Find(ctx, query, findOptions)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []map[string]interface{}
	if err := cur.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// AggregateMongoByField 按字段聚合费用（类似 MySQL 的 GROUP BY）
// fieldPath: MongoDB 字段路径，如 "lineItem/ProductCode"、"product/region"
// filters: 额外过滤条件
func (s *BillService) AggregateMongoByField(fieldPath string, filters map[string]interface{}, startAfter, endBefore *time.Time) (map[string]float64, error) {
	if s.mongoColl == nil {
		return nil, fmt.Errorf("mongodb not initialized")
	}

	ctx := context.Background()
	pipeline := mongo.Pipeline{}

	// match 阶段
	match := bson.D{{Key: "$match", Value: bson.M{}}}
	for k, v := range filters {
		match[0].Value.(bson.M)[k] = v
	}
	if startAfter != nil || endBefore != nil {
		dateFilter := bson.M{}
		if startAfter != nil {
			dateFilter["$gte"] = startAfter.Format(time.RFC3339)
		}
		if endBefore != nil {
			dateFilter["$lt"] = endBefore.Format(time.RFC3339)
		}
		match[0].Value.(bson.M)["lineItem/UsageStartDate"] = dateFilter
	}
	pipeline = append(pipeline, match)

	// group 阶段
	group := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$" + fieldPath},
			{Key: "totalCost", Value: bson.D{{Key: "$sum", Value: "$lineItem/BlendedCost"}}},
		}},
	}
	pipeline = append(pipeline, group)

	cur, err := s.mongoColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	results := make(map[string]float64)
	for cur.Next(ctx) {
		var doc struct {
			ID        interface{} `bson:"_id"`
			TotalCost float64     `bson:"totalCost"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		key := fmt.Sprintf("%v", doc.ID)
		if key == "" {
			key = "unknown"
		}
		results[key] = doc.TotalCost
	}
	return results, nil
}
