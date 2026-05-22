package tencent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/billing/v20180709"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
)

const (
	BatchSize   = 1000
	MaxPageSize = 1000
)

type TencentAdapter struct {
	accessKeyID     string
	secretAccessKey string
	region          string
	accountID       string
	client          *v20180709.Client
}

func NewTencentAdapter(config map[string]interface{}) *TencentAdapter {
	accessKeyID, _ := config["access_key_id"].(string)
	secretAccessKey, _ := config["secret_access_key"].(string)
	region, _ := config["region"].(string)
	if region == "" {
		region = "ap-guangzhou"
	}
	accountID, _ := config["account_id"].(string)

	return &TencentAdapter{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		accountID:       accountID,
	}
}

func (a *TencentAdapter) initClient() error {
	if a.client == nil {
		credential := common.NewCredential(a.accessKeyID, a.secretAccessKey)
		cpf := profile.NewClientProfile()
		client, err := v20180709.NewClient(credential, a.region, cpf)
		if err != nil {
			return fmt.Errorf("create tencent billing client failed: %w", err)
		}
		a.client = client
	}
	return nil
}

func (a *TencentAdapter) ValidateCredentials() (map[string]interface{}, error) {
	credential := common.NewCredential(a.accessKeyID, a.secretAccessKey)
	cpf := profile.NewClientProfile()
	stsClient, err := v20180813.NewClient(credential, a.region, cpf)
	if err != nil {
		return nil, fmt.Errorf("create sts client failed: %w", err)
	}

	req := v20180813.NewGetCallerIdentityRequest()
	resp, err := stsClient.GetCallerIdentity(req)
	if err != nil {
		return nil, fmt.Errorf("credential validation failed: %w", err)
	}

	accountID := ""
	arn := ""
	if resp != nil && resp.Response != nil {
		if resp.Response.AccountId != nil {
			accountID = *resp.Response.AccountId
		}
		if resp.Response.Arn != nil {
			arn = *resp.Response.Arn
		}
	}

	return map[string]interface{}{
		"account_id":    accountID,
		"account_type":  "tencent",
		"principal_arn": arn,
	}, nil
}

func (a *TencentAdapter) GetRawBillingData(billingDate time.Time) ([]map[string]interface{}, error) {
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

func (a *TencentAdapter) StreamRawBillingData(ctx context.Context, billingDate time.Time) (<-chan []map[string]interface{}, <-chan error) {
	dataCh := make(chan []map[string]interface{}, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)

		if err := a.initClient(); err != nil {
			errCh <- err
			return
		}

		month := billingDate.Format("2006-01")
		if err := a.streamResourceSummary(ctx, month, dataCh, errCh); err != nil {
			errCh <- err
		}
	}()

	return dataCh, errCh
}

func (a *TencentAdapter) streamResourceSummary(ctx context.Context, month string, dataCh chan<- []map[string]interface{}, errCh chan<- error) error {
	limit := uint64(MaxPageSize)
	offset := uint64(0)
	needRecordNum := int64(1)

	req := v20180709.NewDescribeBillResourceSummaryRequest()
	req.Month = &month
	req.Limit = &limit
	req.NeedRecordNum = &needRecordNum
	req.Offset = &offset

	for {
		resp, err := a.client.DescribeBillResourceSummary(req)
		if err != nil {
			return fmt.Errorf("DescribeBillResourceSummary failed: %w", err)
		}
		if resp == nil || resp.Response == nil || len(resp.Response.ResourceSummarySet) == 0 {
			return nil
		}

		batch := make([]map[string]interface{}, 0, len(resp.Response.ResourceSummarySet))
		for _, rs := range resp.Response.ResourceSummarySet {
			if rs == nil {
				continue
			}

			resourceID := strVal(rs.ResourceId)
			resourceName := strVal(rs.ResourceName)
			totalCost := f64Val(rs.RealTotalCost)
			regionName := strVal(rs.RegionName)
			region := regionName
			if region == "" && rs.RegionId != nil {
				region = fmt.Sprintf("ap-region-%d", *rs.RegionId)
			}
			serviceCode := strVal(rs.BusinessCode)
			productName := strVal(rs.BusinessCodeName)
			productDetail := strVal(rs.ProductCodeName)

			item := map[string]interface{}{
				// Primary keys for normalizeRawBillingRow
				"resource_id":   resourceID,
				"resourceId":    resourceID,
				"instance_id":   resourceID,
				"cost":          totalCost,
				"amount":        totalCost,
				"currency":      "CNY",
				"region":        region,
				"Region":        region,
				"product_code":  serviceCode,
				"ProductCode":   serviceCode,
				"resource_type": productDetail,
				"resource_name": resourceName,
				"instance_name": resourceName,
				"service_name":  productName,

				// For display name resolution
				"ProductName":   productName,
				"product_name":  productName,
				"ProductDetail": productDetail,
				"product_detail": productDetail,

				// For extra metadata
				"ActionTypeName": strVal(rs.ActionTypeName),
				"PayModeName":    strVal(rs.PayModeName),
				"ProjectName":    strVal(rs.ProjectName),
				"OwnerUin":       strVal(rs.OwnerUin),
				"PayerUin":       strVal(rs.PayerUin),
				"OrderId":        strVal(rs.OrderId),

				// billing cycle
				"billing_cycle": month,
				"BillingCycle":  month,
			}

			// Tags
			if len(rs.Tags) > 0 {
				tagPairs := make([]string, 0, len(rs.Tags))
				for _, tag := range rs.Tags {
					if tag == nil {
						continue
					}
					k := strVal(tag.TagKey)
					v := strVal(tag.TagValue)
					if k != "" {
						item["resourceTags/user:"+k] = v
						item["Resource_tags_user_"+k] = v
					}
					if k != "" || v != "" {
						tagPairs = append(tagPairs, k+":"+v)
					}
				}
				if len(tagPairs) > 0 {
					item["tag"] = strings.Join(tagPairs, ",")
					item["Tag"] = strings.Join(tagPairs, ",")
				}
			}

			// Payment breakdown
			if cash := f64Val(rs.CashPayAmount); cash != 0 {
				item["CashPayAmount"] = cash
			}
			if incentive := f64Val(rs.IncentivePayAmount); incentive != 0 {
				item["IncentivePayAmount"] = incentive
			}
			if voucher := f64Val(rs.VoucherPayAmount); voucher != 0 {
				item["VoucherPayAmount"] = voucher
			}
			if transfer := f64Val(rs.TransferPayAmount); transfer != 0 {
				item["TransferPayAmount"] = transfer
			}

			batch = append(batch, item)
		}

		select {
		case dataCh <- batch:
		case <-ctx.Done():
			return ctx.Err()
		}

		total := *req.Offset + uint64(len(batch))
		if resp.Response.Total != nil && uint64(*resp.Response.Total) > total {
			total = uint64(*resp.Response.Total)
		}
		offset += uint64(len(batch))
		if offset >= total {
			return nil
		}
		req.Offset = &offset
	}
}

func (a *TencentAdapter) GetPricing() ([]map[string]interface{}, error) {
	commonTypes := []string{"S5.LARGE8", "S5.SMALL2", "S5.MEDIUM4", "C5.LARGE8", "M5.LARGE16"}
	var allPrices []map[string]interface{}
	for _, instanceType := range commonTypes {
		allPrices = append(allPrices, map[string]interface{}{
			"cloud_type":     "tencent",
			"service_code":   "cvm",
			"instance_type":  instanceType,
			"region":         a.region,
			"price_per_unit": 0.0,
			"currency":       "CNY",
			"unit":           "Hour",
			"sku":            instanceType,
		})
	}
	return allPrices, nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func f64Val(s *string) float64 {
	if s == nil || *s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(*s, 64)
	if err != nil {
		return 0
	}
	return f
}


