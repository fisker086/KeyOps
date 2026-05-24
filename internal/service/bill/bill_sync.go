package bill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/shopspring/decimal"
)

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
