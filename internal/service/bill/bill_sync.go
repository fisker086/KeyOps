package bill

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/cloud"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/shopspring/decimal"
)

func (s *BillService) SyncCloudBilling(cloudAccountID uint, billingDate time.Time) error {
	billingDateStr := billingDate.Format("2006-01")
	logger.Infof("[BillSync] starting sync: cloudAccountID=%d, billingDate=%s", cloudAccountID, billingDateStr)

	cloudAccount, err := s.cloudRepo.GetByID(cloudAccountID)
	if err != nil {
		return fmt.Errorf("failed to get cloud account: %w", err)
	}

	logger.Infof("[BillSync] cloudAccount: id=%d, cloudType=%s, bucketName=%s, reportName=%s, prefix=%s, region=%s",
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
	defer func() {
		if cleaner, ok := adapter.(cloud.TempFileCleaner); ok {
			cleaner.CleanupTempFiles()
		}
	}()

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

	logger.Infof("[BillSync] credentials validated, setting status to syncing...")
	s.updateCloudAccountStatus(cloudAccountID, "syncing", "")

	ctx, cancel := context.WithCancel(context.Background())
	s.syncCancels[syncKey(cloudAccountID, billingDateStr)] = cancel
	defer func() {
		s.mu.Lock()
		delete(s.syncCancels, syncKey(cloudAccountID, billingDateStr))
		s.mu.Unlock()
		cancel()
	}()
	dataCh, errCh := adapter.StreamRawBillingData(ctx, billingDate)

	var syncErr error
	defer func() {
		if syncErr != nil {
			s.sendSyncNotification(cloudAccount, billingDateStr, syncErr)
		} else {
			s.sendSyncNotification(cloudAccount, billingDateStr, nil)
		}
	}()

	records := make([]model.BillRecord, 0, 4096)
	resourceMap := make(map[string]model.BillResource)
	batchCount := 0

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	logger.Infof("[BillSync] waiting for data batches from adapter")
loop:
	for {
		select {
		case batch, ok := <-dataCh:
			if !ok {
				break loop
			}
			batchCount++

			var batchEffective, batchMarketplace decimal.Decimal
			var batchSkipped int
			batchSampleLogged := false
			parsedCount := 0
			for _, row := range batch {
				record, resource := s.normalizeRawBillingRow(cloudAccount, row, billingDate)
				if record == nil {
					batchSkipped++
					continue
				}
				record.UsageDate = resolveUsageDate(row)
				record.ListCost = record.ConsumeAmount
				records = append(records, *record)
				if resource != nil && resource.ResourceID != "" {
					resourceMap[billResourceKey(resource)] = *resource
				}
				batchEffective = batchEffective.Add(record.EffectiveCost)
				batchMarketplace = batchMarketplace.Add(record.MarketplaceCost)
				parsedCount++

				if !batchSampleLogged && len(records) <= 3 {
					logger.Infof("[BillSync] sample row: instanceID=%s service=%s region=%s cost=%v effective=%v list=%v marketplace=%v lineType=%s",
						record.InstanceID, record.ServiceCode, record.Region,
						record.ConsumeAmount, record.EffectiveCost, record.ListCost, record.MarketplaceCost,
						firstString(row, LineItemTypeKeys...))
					batchSampleLogged = true
				}
			}

			if batchCount%10 == 0 || batchCount == 1 {
				logger.Infof("[BillSync] progress: batch=%d, rows=%d parsed=%d skipped=%d, cumulative=%d, effective=%.4f marketplace=%.4f",
					batchCount, len(batch), parsedCount, batchSkipped, len(records),
					toFloat64(batchEffective), toFloat64(batchMarketplace))
			}

		case err, ok := <-errCh:
			if ok && err != nil {
				s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
				syncErr = fmt.Errorf("stream failed: %w", err)
				return syncErr
			}

		case <-ctx.Done():
			syncErr = fmt.Errorf("sync cancelled: %w", ctx.Err())
			return syncErr

		case <-heartbeat.C:
			logger.Infof("[BillSync] still alive: waiting for data batches, cumulative=%d records", len(records))
		}
	}

	resources := make([]model.BillResource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, resource)
	}
	logger.Infof("[BillSync] writing %d bill records and %d resources to MySQL for account %d", len(records), len(resources), cloudAccountID)
	if err := s.repo.ReplaceBillingRecordsForAccount(cloudAccountID, billingDateStr, records); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("normalize billing records failed: %w", err)
		return syncErr
	}
	logger.Infof("[BillSync] wrote %d bill records to MySQL successfully", len(records))
	if err := s.repo.UpsertBillResources(resources); err != nil {
		s.updateCloudAccountStatus(cloudAccountID, "error", err.Error())
		syncErr = fmt.Errorf("upsert bill resources failed: %w", err)
		return syncErr
	}
	logger.Infof("[BillSync] upserted %d resources to MySQL successfully", len(resources))

	if err := s.rebuildDailyCosts(billingDateStr); err != nil {
		logger.Warnf("[BillSync] rebuild daily costs warning: %v", err)
	}

	s.updateCloudAccountStatus(cloudAccountID, "active", "")
	if err := s.cloudRepo.UpdateLastImport(cloudAccountID); err != nil {
		syncErr = err
		return err
	}
	logger.Infof("[BillSync] synced %d bill records, %d resources for account %d", len(records), len(resources), cloudAccountID)

	if err := s.repo.RebuildDashboardAggregates(billingDateStr); err != nil {
		logger.Warnf("[BillSync] rebuild dashboard aggregates warning: %v", err)
	}
	if err := s.repo.RebuildSummary(cloudAccount.CloudType, billingDateStr); err != nil {
		logger.Warnf("[BillSync] rebuild summary warning: %v", err)
	}

	InvalidateDashboardCache()
	return nil
}

func (s *BillService) SyncCloudBillingAsync(cloudAccountID uint, billingDate time.Time) {
	_, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.syncCancels[syncKey(cloudAccountID, billingDate.Format("2006-01"))] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("[BillSync] async sync panicked: account=%d, panic=%v", cloudAccountID, r)
				s.updateCloudAccountStatus(cloudAccountID, "error", fmt.Sprintf("sync panic: %v", r))
			}
		}()
		defer cancel()
		logger.Infof("[BillSync] starting async sync: account=%d, billingDate=%s", cloudAccountID, billingDate.Format("2006-01"))
		if err := s.SyncCloudBilling(cloudAccountID, billingDate); err != nil {
			if s.isCancelledError(err) {
				logger.Infof("[BillSync] sync cancelled: account=%d, billingDate=%s", cloudAccountID, billingDate.Format("2006-01"))
				s.updateCloudAccountStatus(cloudAccountID, "active", "")
			} else {
				logger.Errorf("[BillSync] async sync failed: account=%d, err=%v", cloudAccountID, err)
			}
		}
	}()
}

func (s *BillService) CancelSync(cloudAccountID uint, billingDate time.Time) error {
	s.mu.Lock()
	cancel, ok := s.syncCancels[syncKey(cloudAccountID, billingDate.Format("2006-01"))]
	s.mu.Unlock()

	if ok {
		cancel()
		return nil
	}

	s.updateCloudAccountStatus(cloudAccountID, "active", "")
	return nil
}

func (s *BillService) isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "context canceled" || err.Error() == "sync cancelled"
}

func (s *BillService) SyncCloudBillingRange(cloudAccountID uint, startDate, endDate time.Time) {
	go func() {
		current := startDate
		logger.Infof("[BillSync] starting range sync: account=%d, from=%s, to=%s", cloudAccountID, startDate.Format("2006-01"), endDate.Format("2006-01"))
		for !current.After(endDate) {
			logger.Infof("[BillSync] async range sync: account=%d, month=%s", cloudAccountID, current.Format("2006-01"))
			if err := s.SyncCloudBilling(cloudAccountID, current); err != nil {
				if s.isCancelledError(err) {
					logger.Infof("[BillSync] range sync cancelled: account=%d, month=%s", cloudAccountID, current.Format("2006-01"))
				} else {
					logger.Errorf("[BillSync] async range sync failed: account=%d, month=%s, err=%v", cloudAccountID, current.Format("2006-01"), err)
				}
				return
			}
			current = current.AddDate(0, 1, 0)
		}
	}()
}

// excludedLineItemTypes 同步时跳过的行类型（Tax 计入清单对账价以对齐 YCloud Excel AWS 服务费用小计）
var excludedLineItemTypes = map[string]bool{}

var usageDateFieldKeys = []string{
	"Line_item_usage_start_date",
	"lineItem/UsageStartDate",
}

var billingPeriodFieldKeys = []string{
	"Identity_time_interval",
	"identity/TimeInterval",
	"Bill_billing_period_start_date",
	"bill/BillingPeriodStartDate",
}

// rowInBillingMonth 行必须落在目标账期月（用量日或账单周期），无日期行丢弃
func rowInBillingMonth(row map[string]interface{}, billingDate time.Time) bool {
	return cloud.CURRowInMonth(row, billingDate.Format("2006-01"))
}

func computeEffectiveCost(row map[string]interface{}, unblendedCost decimal.Decimal) decimal.Decimal {
	for _, key := range []string{"Reservation_effective_cost", "reservation/EffectiveCost"} {
		if v := firstDecimal(row, key); v.GreaterThan(decimal.Zero) {
			return v
		}
	}
	for _, key := range []string{
		"Savings_plan_savings_plan_effective_cost",
		"savings_plan_savings_plan_effective_cost",
		"savingsPlan/SavingsPlanEffectiveCost",
	} {
		if v := firstDecimal(row, key); v.GreaterThan(decimal.Zero) {
			return v
		}
	}
	return unblendedCost
}

func (s *BillService) normalizeRawBillingRow(account *model.CloudAccount, row map[string]interface{}, billingDate time.Time) (*model.BillRecord, *model.BillResource) {
	lineItemType := firstString(row, LineItemTypeKeys...)
	if excludedLineItemTypes[lineItemType] {
		return nil, nil
	}
	if !rowInBillingMonth(row, billingDate) {
		return nil, nil
	}

	usageAccountID := firstNonEmpty(firstString(row, AccountIDKeys...), account.AccountID)

	billingEntity := firstString(row, BillingEntityKeys...)

	// 清单对账价（consume_amount）：每行在账单上的标价
	cost := firstDecimal(row, UnblendedCostFieldKeys...)

	// 摊销分摊价（effective_cost）：RI/SP 折扣后的实际成本
	effectiveCost := computeEffectiveCost(row, cost)
	if effectiveCost.IsZero() && !cost.IsZero() {
		effectiveCost = cost
	}
	if effectiveCost.IsZero() {
		effectiveCost = firstDecimal(row, AmortizedCostFieldKeys...)
	}

	// 全部为零且非抵扣类行 → 跳过
	if cost.IsZero() && effectiveCost.IsZero() && lineItemType != "Credit" && lineItemType != "Refund" {
		return nil, nil
	}

	instanceID := firstString(row, ResourceIDKeys...)
	serviceCode := firstString(row, ProductCodeKeys...)
	serviceType := firstString(row, ServiceNameKeys...)

	isMarketplace := isYCloudMarketplaceCost(usageAccountID, serviceType, billingEntity)

	var excludedServiceCost decimal.Decimal
	if isExcludedFromServiceTotal(account.AccountID, usageAccountID, serviceType, serviceCode) {
		excludedServiceCost = cost
		if effectiveCost.IsZero() && !cost.IsZero() {
			effectiveCost = cost
		}
		cost = decimal.Zero
		effectiveCost = decimal.Zero
	}
	productName := firstString(row, "ProductName", "product_name")
	productDetail := firstString(row, "ProductDetail", "product_detail")
	rawRegion := firstString(row, RegionKeys...)
	usageType := firstString(row, UsageTypeKeys...)
	operation := firstString(row, OperationKeys...)
	currency := firstString(row, CurrencyKeys...)

	resourceType := firstNonEmpty(
		firstString(row, ProductFamilyKeys...),
		serviceCode,
	)
	region := firstNonEmpty(rawRegion, account.Region)
	tags := extractUserTags(row)
	tagsJSON := encodeStringMap(tags)

	displayName := firstString(row, ResourceNameKeys...)

	extra := map[string]interface{}{
		"line_item_type":  lineItemType,
		"usage_type":      usageType,
		"operation":       operation,
		"currency":        currency,
		"product_name":    productName,
		"product_detail":  productDetail,
		"billing_entity":              billingEntity,
		"is_marketplace":              isMarketplace,
		"excluded_from_service_total": excludedServiceCost.GreaterThan(decimal.Zero),
	}

	specDesc := firstString(row, UsageTypeKeys...)
	if specDesc == "" {
		specDesc = firstString(row, "InstanceSpec", "instance_spec")
	}

	resourceName := firstNonEmpty(displayName, productDetail, productName, instanceID)

	// Marketplace 费用：从 consume_amount 剥离，单独存 marketplace_cost
	var marketplaceCost decimal.Decimal
	if isMarketplace {
		marketplaceCost = cost
		cost = decimal.Zero
		effectiveCost = decimal.Zero
	}

	record := &model.BillRecord{
		Vendor:          account.CloudType,
		Cycle:           billingDate.Format("2006-01"),
		InstanceID:      instanceID,
		ResourceName:    resourceName,
		SpecDesc:        specDesc,
		ConsumeAmount:         cost,
		EffectiveCost:         effectiveCost,
		MarketplaceCost:       marketplaceCost,
		ExcludedServiceCost:   excludedServiceCost,
		ResourceType:          resourceType,
		ResourceCode:          resourceType,
		ServiceType:           serviceType,
		ServiceCode:           serviceCode,
		Region:                region,
		AccountID:             usageAccountID,
		CloudAccountID:  account.ID,
		Tags:            tagsJSON,
		Extra:           encodeMap(extra),
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
		InstanceType:   firstString(row, InstanceTypeKeys...),
		Region:         region,
		Zone:           firstString(row, AZKeys...),
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
		if value, ok := row[key]; ok {
			switch v := value.(type) {
			case float64:
				return decimal.NewFromFloat(v)
			case float32:
				return decimal.NewFromFloat32(v)
			case int64:
				return decimal.NewFromInt(v)
			case int:
				return decimal.NewFromInt(int64(v))
			}
		}
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

func (s *BillService) updateCloudAccountStatus(id uint, status, errorMsg string) {
	logger.Infof("[BillSync] updating account status: id=%d, status=%s, error=%s", id, status, errorMsg)
	acc, err := s.cloudRepo.GetByID(id)
	if err != nil {
		logger.Errorf("[BillSync] failed to get account for status update: id=%d, err=%v", id, err)
		return
	}
	acc.Status = status
	if errorMsg != "" {
		acc.LastImportError = errorMsg
	} else {
		acc.LastImportError = ""
	}
	if err := s.cloudRepo.Update(acc); err != nil {
		logger.Errorf("[BillSync] failed to update account status: id=%d, err=%v", id, err)
	}
}

func (s *BillService) sendSyncNotification(account *model.CloudAccount, billingDate string, syncErr error) {
	if (account.NotifyEnabled == nil || !*account.NotifyEnabled) || account.NotifyChannelID == 0 {
		return
	}
	channel, err := s.alertChannelRepo.FindByID(account.NotifyChannelID)
	if err != nil || channel == nil {
		logger.Errorf("[BillSync] failed to load alert channel %d for account %d: %v", account.NotifyChannelID, account.ID, err)
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
		logger.Warnf("[BillSync] webhook URL is empty for channel %d", channel.ID)
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
		logger.Warnf("[BillSync] unsupported channel type %s for channel %d", channel.ChannelType, channel.ID)
		return
	}
	if err := n.SendAlert(title, content); err != nil {
		logger.Errorf("[BillSync] failed to send sync notification for account %d: %v", account.ID, err)
	}
}

// bill_daily_cost 表内金额为各 vendor 原生币种（AWS 为 USD），由展示层换算。
func (s *BillService) rebuildDailyCosts(cycle string) error {
	costs, err := s.repo.GetDailyCostsFromRecords(cycle)
	if err != nil {
		return fmt.Errorf("get daily costs from records: %w", err)
	}
	if len(costs) == 0 {
		logger.Infof("[BillSync] rebuildDailyCosts: no daily costs found for cycle=%s", cycle)
		return nil
	}
	var totalCost, totalEffective, totalList float64
	dateMap := make(map[string]bool)
	for _, c := range costs {
		totalCost += c.Cost
		totalEffective += c.EffectiveCost
		totalList += c.ListCost
		dateMap[c.Date] = true
	}
	logger.Infof("[BillSync] rebuildDailyCosts: cycle=%s dates=%d entries=%d cost=%.4f effective=%.4f list=%.4f",
		cycle, len(dateMap), len(costs), totalCost, totalEffective, totalList)
	return s.repo.ReplaceDailyCosts(cycle, costs)
}

// RebuildBillAggregates 从 bill_records 重建日费用与 Dashboard 预聚合（无需重新拉取 CUR）。
// 修复货币口径或 bill_daily_cost 脏数据后调用；cycle 格式 2026-05。
func (s *BillService) RebuildBillAggregates(cycle string) error {
	if err := s.rebuildDailyCosts(cycle); err != nil {
		return err
	}
	if err := s.repo.RebuildDashboardAggregates(cycle); err != nil {
		return fmt.Errorf("rebuild dashboard aggregates: %w", err)
	}
	InvalidateDashboardCache()
	logger.Infof("[BillSync] RebuildBillAggregates done cycle=%s", cycle)
	return nil
}

func toFloat64(d decimal.Decimal) float64 {
	v, _ := d.Float64()
	return v
}

// resolveUsageDate 从原始行中提取使用日期（YYYY-MM-DD），兼容 AWS/Aliyun/Tencent
func resolveUsageDate(row map[string]interface{}) string {
	if day := cloud.CURDayFromRow(row, usageDateFieldKeys...); day != "" {
		return day
	}
	if day := cloud.CURDayFromRow(row, billingPeriodFieldKeys...); day != "" {
		return day
	}
	return cloud.CURDayFromRow(row, "BillingDate")
}
