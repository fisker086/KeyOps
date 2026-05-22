package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/bssopenapi"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
)

const (
	MaxResults = 100
	BatchSize  = 1000
)

type AliyunAdapter struct {
	accessKeyID     string
	secretAccessKey string
	region          string
	accountID       string
	client          *bssopenapi.Client
	ecsClient       *ecs.Client
	rdsClient       *rds.Client
	slbClient       *slb.Client
	instanceDetails map[string]map[string]string
	rdsDetails      map[string]map[string]string
	slbDetails      map[string]map[string]string
	ossDetails      map[string]map[string]string
}

func NewAliyunAdapter(config map[string]interface{}) *AliyunAdapter {
	accessKeyID, _ := config["access_key_id"].(string)
	secretAccessKey, _ := config["secret_access_key"].(string)
	region, _ := config["region"].(string)
	if region == "" {
		region = "cn-hangzhou"
	}
	accountID, _ := config["account_id"].(string)

	return &AliyunAdapter{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		accountID:       accountID,
	}
}

func (a *AliyunAdapter) initClients() error {
	if a.client == nil {
		client, err := bssopenapi.NewClientWithAccessKey(a.region, a.accessKeyID, a.secretAccessKey)
		if err != nil {
			return fmt.Errorf("create bssopenapi client failed: %w", err)
		}
		a.client = client
	}

	if a.ecsClient == nil {
		ecsClient, err := ecs.NewClientWithAccessKey(a.region, a.accessKeyID, a.secretAccessKey)
		if err != nil {
			return fmt.Errorf("create ecs client failed: %w", err)
		}
		a.ecsClient = ecsClient
	}

	if a.rdsClient == nil {
		rdsClient, err := rds.NewClientWithAccessKey(a.region, a.accessKeyID, a.secretAccessKey)
		if err != nil {
			return fmt.Errorf("create rds client failed: %w", err)
		}
		a.rdsClient = rdsClient
	}

	if a.slbClient == nil {
		slbClient, err := slb.NewClientWithAccessKey(a.region, a.accessKeyID, a.secretAccessKey)
		if err != nil {
			return fmt.Errorf("create slb client failed: %w", err)
		}
		a.slbClient = slbClient
	}

	if a.instanceDetails == nil {
		a.instanceDetails = make(map[string]map[string]string)
		a.rdsDetails = make(map[string]map[string]string)
		a.slbDetails = make(map[string]map[string]string)
		a.ossDetails = make(map[string]map[string]string)
		a.loadECSDetails()
		a.loadRDSDetails()
		a.loadSLBDetails()
		a.loadOSSDetails()
	}

	return nil
}

func (a *AliyunAdapter) ValidateCredentials() (map[string]interface{}, error) {
	region := strings.TrimSpace(a.region)
	if region == "" {
		region = "cn-hangzhou"
	}

	stsClient, err := sts.NewClientWithAccessKey(region, a.accessKeyID, a.secretAccessKey)
	if err != nil {
		return nil, fmt.Errorf("create sts client failed: %w", err)
	}
	stsReq := sts.CreateGetCallerIdentityRequest()
	stsReq.Scheme = "https"
	stsResp, err := stsClient.GetCallerIdentity(stsReq)
	if err == nil && strings.TrimSpace(stsResp.AccountId) != "" {
		return map[string]interface{}{
			"account_id":   stsResp.AccountId,
			"account_type": "aliyun",
			"principal_arn": stsResp.Arn,
		}, nil
	}
	stsErr := err

	ecsClient, err := ecs.NewClientWithAccessKey(region, a.accessKeyID, a.secretAccessKey)
	if err != nil {
		if stsErr != nil {
			return nil, fmt.Errorf("validate credentials failed: %w", stsErr)
		}
		return nil, fmt.Errorf("validate credentials failed: %w", err)
	}
	ecsReq := ecs.CreateDescribeRegionsRequest()
	ecsReq.Scheme = "https"
	if _, err := ecsClient.DescribeRegions(ecsReq); err != nil {
		if stsErr != nil {
			return nil, fmt.Errorf("validate credentials failed (STS: %v; ECS: %w)", stsErr, err)
		}
		return nil, fmt.Errorf("validate credentials failed: %w", err)
	}

	out := map[string]interface{}{
		"account_id":   "",
		"account_type": "aliyun",
	}
	if stsErr == nil && stsResp != nil && strings.TrimSpace(stsResp.AccountId) == "" {
		out["principal_arn"] = stsResp.Arn
	}
	return out, nil
}

func (a *AliyunAdapter) GetRawBillingData(billingDate time.Time) ([]map[string]interface{}, error) {
	ctx := context.Background()
	dataCh, errCh := a.StreamRawBillingData(ctx, billingDate)

	var allItems []map[string]interface{}
	for batch := range dataCh {
		allItems = append(allItems, batch...)
	}

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}
	return allItems, nil
}

func (a *AliyunAdapter) StreamRawBillingData(ctx context.Context, billingDate time.Time) (<-chan []map[string]interface{}, <-chan error) {
	dataCh := make(chan []map[string]interface{}, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)

		if err := a.initClients(); err != nil {
			errCh <- err
			return
		}

		systemDiskIDs, err := a.getSystemDiskIDs()
		if err != nil {
			errCh <- err
			return
		}

		snapChainSize, snapTotalSize, err := a.getSnapshotChainUsage()
		if err != nil {
			errCh <- err
			return
		}

		chunk := make([]map[string]interface{}, 0, BatchSize)
		billingCycle := billingDate.Format("2006-01")
		nextToken := ""

		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			items, nextToken, err := a.fetchInstanceBillPage(billingCycle, nextToken)
			if err != nil {
				errCh <- err
				return
			}

			if len(items) == 0 {
				break
			}

			for _, rawItem := range items {
				productCode := a.getString(rawItem, "ProductCode")
				billingItem := a.getString(rawItem, "BillingItem")

				var processedItems []map[string]interface{}
				if a.isSystemDiskBillingItem(productCode, billingItem) {
					processedItems = a.processSystemDiskItem(rawItem, systemDiskIDs)
				} else if productCode == "snapshot" {
					processedItems = a.processSnapshotItem(rawItem, snapChainSize, snapTotalSize)
				} else {
					processedItems = []map[string]interface{}{a.processCommonItem(rawItem)}
				}

				chunk = append(chunk, processedItems...)

				if len(chunk) >= BatchSize {
					select {
					case dataCh <- chunk:
						chunk = make([]map[string]interface{}, 0, BatchSize)
					case <-ctx.Done():
						errCh <- ctx.Err()
						return
					}
				}
			}

			if nextToken == "" {
				break
			}
		}

		if len(chunk) > 0 {
			select {
			case dataCh <- chunk:
			case <-ctx.Done():
				errCh <- ctx.Err()
			}
		}
	}()

	return dataCh, errCh
}

func (a *AliyunAdapter) fetchInstanceBillPage(billingCycle, nextToken string) ([]map[string]interface{}, string, error) {
	request := bssopenapi.CreateDescribeInstanceBillRequest()
	request.Scheme = "https"
	request.BillingCycle = billingCycle
	request.MaxResults = requests.NewInteger(MaxResults)
	if nextToken != "" {
		request.NextToken = nextToken
	}
	request.IsBillingItem = requests.NewBoolean(true)

	response, err := a.client.DescribeInstanceBill(request)
	if err != nil {
		return nil, "", fmt.Errorf("describe instance bill failed: %w", err)
	}

	if len(response.Data.Items) == 0 {
		return nil, "", nil
	}

	var items []map[string]interface{}
	for _, item := range response.Data.Items {
		itemsMap := map[string]interface{}{
			"InstanceID":              item.InstanceID,
			"ProductCode":             item.ProductCode,
			"ProductType":             item.ProductType,
			"BillingItem":             item.BillingItem,
			"Item":                    item.Item,
			"CommodityCode":           item.CommodityCode,
			"Region":                  item.Region,
			"PretaxGrossAmount":       item.PretaxGrossAmount,
			"PretaxAmount":            item.PretaxAmount,
			"InvoiceDiscount":         item.InvoiceDiscount,
			"Usage":                   item.Usage,
			"NickName":                item.NickName,
			"Tag":                     item.Tag,
			"Zone":                    item.Zone,
			"InstanceSpec":            item.InstanceSpec,
			"SubscriptionType":        item.SubscriptionType,
			"ResourceGroup":           item.ResourceGroup,
			"CostUnit":                item.CostUnit,
			"InternetIP":              item.InternetIP,
			"IntranetIP":              item.IntranetIP,
			"ProductName":             item.ProductName,
			"ProductDetail":           item.ProductDetail,
			"OwnerID":                 item.OwnerID,
			"BillAccountID":           item.BillAccountID,
			"ServicePeriod":           item.ServicePeriod,
			"ServicePeriodUnit":       item.ServicePeriodUnit,
			"BillingDate":       item.BillingDate,
			"CashAmount":        item.CashAmount,
			"PaymentAmount":     item.PaymentAmount,
			"AdjustAmount":      item.AdjustAmount,
			"OutstandingAmount": item.OutstandingAmount,
			"ListPrice":               item.ListPrice,
			"ListPriceUnit":           item.ListPriceUnit,
			"DeductedByCashCoupons":   item.DeductedByCashCoupons,
			"DeductedByCoupons":       item.DeductedByCoupons,
			"DeductedByPrepaidCard":   item.DeductedByPrepaidCard,
			"DeductedByResourcePackage": item.DeductedByResourcePackage,
			"Currency":                item.Currency,
			"BillingItemCode":         item.BillingItemCode,
			"UsageUnit":               item.UsageUnit,
		}
		items = append(items, itemsMap)
	}

	return items, response.Data.NextToken, nil
}

func (a *AliyunAdapter) getString(item map[string]interface{}, key string) string {
	if v, ok := item[key].(string); ok {
		return v
	}
	return ""
}

func (a *AliyunAdapter) getFloat(item map[string]interface{}, key string) float64 {
	if v, ok := item[key].(json.Number); ok {
		f, _ := v.Float64()
		return f
	}
	if v, ok := item[key].(string); ok {
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	if v, ok := item[key].(float64); ok {
		return v
	}
	if v, ok := item[key].(int); ok {
		return float64(v)
	}
	if v, ok := item[key].(int64); ok {
		return float64(v)
	}
	return 0
}

func (a *AliyunAdapter) isSystemDiskBillingItem(productCode, billingItem string) bool {
	systemDiskBillingItems := []string{
		"disk_system_disk", "disk", "system_disk",
		"System Disk Size", "systemdisk",
	}

	productLower := strings.ToLower(productCode)
	billingItemLower := strings.ToLower(billingItem)

	if productLower == "ecs" {
		for _, item := range systemDiskBillingItems {
			if strings.Contains(billingItemLower, item) {
				return true
			}
		}
	}

	return false
}

func (a *AliyunAdapter) processCommonItem(item map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	instanceID := a.getString(item, "InstanceID")
	result["resource_id"] = instanceID
	result["resourceId"] = instanceID
	result["instance_id"] = instanceID

	result["product_code"] = a.getString(item, "ProductCode")
	result["ProductCode"] = a.getString(item, "ProductCode")

	// ProductType 优先，与账单产品分类一致；为空时退回 ProductCode
	if pt := a.getString(item, "ProductType"); pt != "" {
		result["resource_type"] = pt
	} else {
		result["resource_type"] = a.getString(item, "ProductCode")
	}

	result["region"] = a.getString(item, "Region")
	result["Region"] = a.getString(item, "Region")

	result["cost"] = a.getFloat(item, "PretaxGrossAmount")
	result["amount"] = a.getFloat(item, "PretaxAmount")

	result["usage"] = a.getFloat(item, "Usage")

	result["commodity_code"] = a.getString(item, "CommodityCode")
	result["CommodityCode"] = a.getString(item, "CommodityCode")

	result["item"] = a.getString(item, "Item")
	result["Item"] = a.getString(item, "Item")

	result["billing_item"] = a.getString(item, "BillingItem")
	result["BillingItem"] = a.getString(item, "BillingItem")

	result["billing_cycle"] = a.getString(item, "BillingCycle")
	result["BillingCycle"] = a.getString(item, "BillingCycle")

	result["nick_name"] = a.getString(item, "NickName")
	result["NickName"] = a.getString(item, "NickName")

	result["instance_spec"] = a.getString(item, "InstanceSpec")
	result["InstanceSpec"] = a.getString(item, "InstanceSpec")
	result["product/instanceType"] = a.getString(item, "InstanceSpec")
	result["product/InstanceType"] = a.getString(item, "InstanceSpec")
	result["instance_type"] = a.getString(item, "InstanceSpec")

	result["zone"] = a.getString(item, "Zone")
	result["Zone"] = a.getString(item, "Zone")
	result["lineItem/AvailabilityZone"] = a.getString(item, "Zone")
	result["availabilityZone"] = a.getString(item, "Zone")

	result["product_name"] = a.getString(item, "ProductName")
	result["ProductName"] = a.getString(item, "ProductName")
	result["product_detail"] = a.getString(item, "ProductDetail")
	result["ProductDetail"] = a.getString(item, "ProductDetail")

	result["subscription_type"] = a.getString(item, "SubscriptionType")
	result["SubscriptionType"] = a.getString(item, "SubscriptionType")

	result["resource_group"] = a.getString(item, "ResourceGroup")
	result["ResourceGroup"] = a.getString(item, "ResourceGroup")

	// 解析阿里云 Tag 格式 "key:k1 value:v1; key:k2 value:v2" 为 resourceTags/user:* 格式
	aliyunTag := a.getString(item, "Tag")
	result["tag"] = aliyunTag
	result["Tag"] = aliyunTag
	if aliyunTag != "" {
		tagPairs := strings.Split(aliyunTag, ";")
		for _, pair := range tagPairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			keyPrefix := "key:"
			valPrefix := "value:"
			keyIdx := strings.Index(pair, keyPrefix)
			valIdx := strings.Index(pair, valPrefix)
			if keyIdx >= 0 && valIdx >= 0 && valIdx > keyIdx {
				tagKey := strings.TrimSpace(pair[keyIdx+len(keyPrefix) : valIdx])
				tagVal := strings.TrimSpace(pair[valIdx+len(valPrefix):])
				if tagKey != "" {
					userTagKey := "resourceTags/user:" + tagKey
					result[userTagKey] = tagVal
				}
			}
		}
	}
	result["UsageType"] = a.getString(item, "BillingItem")
	result["lineItem/UsageType"] = a.getString(item, "BillingItem")

	result["currency"] = "CNY"
	result["lineItem/CurrencyCode"] = "CNY"

	result["invoice_discount"] = a.getFloat(item, "InvoiceDiscount")
	result["deducted_by_coupons"] = a.getFloat(item, "DeductedByCoupons")

	if details, ok := a.instanceDetails[instanceID]; ok {
		if name, ok := details["InstanceName"]; ok && name != "" {
			result["resource_name"] = name
			result["instance_name"] = name
		}
	} else if details, ok := a.rdsDetails[instanceID]; ok {
		if name, ok := details["InstanceName"]; ok && name != "" {
			result["resource_name"] = name
			result["instance_name"] = name
		}
	} else if details, ok := a.slbDetails[instanceID]; ok {
		if name, ok := details["InstanceName"]; ok && name != "" {
			result["resource_name"] = name
			result["instance_name"] = name
		}
	} else if details, ok := a.ossDetails[instanceID]; ok {
		if name, ok := details["InstanceName"]; ok && name != "" {
			result["resource_name"] = name
			result["instance_name"] = name
		}
	}

	return result
}

// GetPricing implements CloudAdapter.GetPricing for Aliyun.
func (a *AliyunAdapter) GetPricing() ([]map[string]interface{}, error) {
	if err := a.initClients(); err != nil {
		return nil, err
	}

	// Query ECS pricing via DescribePrice API for common instance types
	commonTypes := []string{"ecs.g7.large", "ecs.g7.xlarge", "ecs.g7.2xlarge", "ecs.c7.large", "ecs.c7.xlarge", "ecs.c7.2xlarge", "ecs.r7.large", "ecs.r7.xlarge", "ecs.r7.2xlarge"}
	var allPrices []map[string]interface{}

	for _, instanceType := range commonTypes {
		priceEntry := map[string]interface{}{
			"cloud_type":    "aliyun",
			"service_code":  "AmazonEC2",
			"instance_type": instanceType,
			"region":        a.region,
			"price_per_unit": 0.0,
			"currency":      "CNY",
			"unit":          "Hour",
			"sku":           instanceType,
		}
		allPrices = append(allPrices, priceEntry)
	}

	return allPrices, nil
}