package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	psource "github.com/xitongsys/parquet-go-source/local"
	pq "github.com/xitongsys/parquet-go/reader"
)

var (
	parquetRegex = regexp.MustCompile(`\.snappy\.parquet(\.gz)?$`)
)

// AWSAdapter AWS云适配器，实现CloudAdapter接口
type AWSAdapter struct {
	accessKeyID     string
	secretAccessKey string
	region          string
	bucketName      string
	bucketPrefix    string
	reportName      string
	session         *session.Session
}

// NewAWSAdapter 创建AWS适配器
func NewAWSAdapter(config map[string]interface{}) *AWSAdapter {
	accessKeyID, _ := config["access_key_id"].(string)
	secretAccessKey, _ := config["secret_access_key"].(string)
	region, _ := config["region"].(string)
	if region == "" {
		region = "us-east-1"
	}
	bucketName, _ := config["bucket_name"].(string)
	bucketPrefix, _ := config["bucket_prefix"].(string)
	if bucketPrefix == "" {
		bucketPrefix = "reports"
	}
	reportName, _ := config["report_name"].(string)
	if reportName == "" {
		reportName = "optscale-report"
	}

	return &AWSAdapter{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		bucketName:      bucketName,
		bucketPrefix:    bucketPrefix,
		reportName:      reportName,
	}
}

// -----------------------------------------------------------------------------------
// CloudAdapter接口实现
// -----------------------------------------------------------------------------------

// ValidateCredentials 验证AWS凭证
func (a *AWSAdapter) ValidateCredentials() (map[string]interface{}, error) {
	sess, err := a.getSession()
	if err != nil {
		return nil, err
	}

	stsSvc := sts.New(sess)
	result, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("credential validation failed: %w", err)
	}

	return map[string]interface{}{
		"account_id":   *result.Account,
		"account_type": "aws",
	}, nil
}

// GetRawBillingData 全量获取（保留兼容）
func (a *AWSAdapter) GetRawBillingData(billingDate time.Time) ([]map[string]interface{}, error) {
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

// StreamRawBillingData AWS 流式解析 Parquet 文件，每 1000 行发一批，避免 OOM
func (a *AWSAdapter) StreamRawBillingData(ctx context.Context, billingDate time.Time) (<-chan []map[string]interface{}, <-chan error) {
	dataCh := make(chan []map[string]interface{}, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)

		log.Printf("[AWSSync] starting sync for billingDate=%s", billingDate.Format("2006-01"))

		manifest, err := a.getManifest(billingDate)
		if err != nil {
			log.Printf("[AWSSync] getManifest failed: %v", err)
			errCh <- err
			return
		}

		log.Printf("[AWSSync] manifest fetched, reportKeys=%d", len(manifest.ReportKeys))

		for _, reportKey := range manifest.ReportKeys {
			log.Printf("[AWSSync] processing parquet reportKey=%s", reportKey)
			if err := a.parseParquetReportStream(ctx, reportKey, billingDate, dataCh); err != nil {
				errCh <- err
				return
			}
		}
		log.Printf("[AWSSync] sync completed successfully for billingDate=%s", billingDate.Format("2006-01"))
	}()

	return dataCh, errCh
}

// -----------------------------------------------------------------------------------
// S3基础操作
// -----------------------------------------------------------------------------------

// getSession 获取AWS会话（单例）
func (a *AWSAdapter) getSession() (*session.Session, error) {
	if a.session != nil {
		return a.session, nil
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(a.region),
		Credentials: credentials.NewStaticCredentials(a.accessKeyID, a.secretAccessKey, ""),
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			Timeout:   30 * time.Minute,
		},
		MaxRetries: aws.Int(3),
		S3ForcePathStyle: aws.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("create AWS session failed: %w", err)
	}
	a.session = sess
	return sess, nil
}

// getBucketRegion 获取bucket所在region（S3 bucket可能在不同于配置的region）
func (a *AWSAdapter) getBucketRegion() (string, error) {
	sess, err := a.getSession()
	if err != nil {
		return "", err
	}
	s3Svc := s3.New(sess)
	out, err := s3Svc.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(a.bucketName),
	})
	if err != nil {
		return a.region, err
	}
	location := out.LocationConstraint
	if location == nil || *location == "" {
		return "us-east-1", nil
	}
	return *location, nil
}

// downloadReportBytes 从S3下载报告文件（支持gz压缩）
func (a *AWSAdapter) downloadReportBytes(key string) ([]byte, error) {
	bucketRegion, err := a.getBucketRegion()
	if err != nil {
		bucketRegion = a.region
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(bucketRegion),
		Credentials: credentials.NewStaticCredentials(a.accessKeyID, a.secretAccessKey, ""),
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			Timeout:   30 * time.Minute,
		},
		MaxRetries: aws.Int(3),
	})
	if err != nil {
		return nil, fmt.Errorf("create session for bucket region failed: %w", err)
	}
	s3c := s3.New(sess)
	out, err := s3c.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(a.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	raw, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(strings.ToLower(key), ".gz") {
		gzr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		return io.ReadAll(gzr)
	}
	return raw, nil
}

// -----------------------------------------------------------------------------------
// S3 CUR Manifest操作
// -----------------------------------------------------------------------------------

type Manifest struct {
	ReportKeys []string `json:"reportKeys"`
}

// getManifest 列出 CUR 2.0 Parquet 文件
func (a *AWSAdapter) getManifest(billingDate time.Time) (*Manifest, error) {
	return a.listParquetFiles(billingDate)
}

// listParquetFiles 列出 CUR 2.0 格式的 Parquet 文件
func (a *AWSAdapter) listParquetFiles(billingDate time.Time) (*Manifest, error) {
	sess, err := a.getSession()
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}
	s3Svc := s3.New(sess)

	prefix := a.bucketPrefix
	if prefix == "" {
		prefix = "reports"
	}

	// CUR 2.0 Hive 分区格式: <prefix>/year=2026/month=5/
	year := billingDate.Format("2006")
	month := billingDate.Format("1")
	listPrefix := fmt.Sprintf("%s/year=%s/month=%s/", prefix, year, month)

	log.Printf("[AWSSync] listing parquet files with prefix: %s", listPrefix)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(a.bucketName),
		Prefix: aws.String(listPrefix),
	}

	var reportKeys []string
	err = s3Svc.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			if parquetRegex.MatchString(key) {
				reportKeys = append(reportKeys, key)
			}
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("list s3 objects failed: %w", err)
	}

	if len(reportKeys) == 0 {
		// 尝试上月
		prevDate := billingDate.AddDate(0, -1, 0)
		prevYear := prevDate.Format("2006")
		prevMonth := prevDate.Format("1")
		listPrefix = fmt.Sprintf("%s/year=%s/month=%s/", prefix, prevYear, prevMonth)
		log.Printf("[AWSSync] no parquet files found for %s/%s, trying previous month: %s", year, month, listPrefix)

		input.Prefix = aws.String(listPrefix)
		err = s3Svc.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				key := *obj.Key
				if parquetRegex.MatchString(key) {
					reportKeys = append(reportKeys, key)
				}
			}
			return true
		})

		if err != nil {
			return nil, fmt.Errorf("list s3 objects failed: %w", err)
		}
	}

	if len(reportKeys) == 0 {
		return nil, fmt.Errorf("no parquet files found for %s or previous month", billingDate.Format("2006-01"))
	}

	log.Printf("[AWSSync] selecting %d parquet files for sync", len(reportKeys))
	return &Manifest{ReportKeys: reportKeys}, nil
}

// parseParquetReportStream 解析 AWS CUR v2 parquet 文件，逐批发送到 dataCh
func (a *AWSAdapter) parseParquetReportStream(ctx context.Context, reportKey string, billingDate time.Time, dataCh chan<- []map[string]interface{}) error {
	raw, err := a.downloadReportBytes(reportKey)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "cur-*.snappy.parquet")
	if err != nil {
		return fmt.Errorf("create temp file for parquet failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(raw); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write parquet temp file failed: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close parquet temp file failed: %w", err)
	}

	fr, err := psource.NewLocalFileReader(tmpPath)
	if err != nil {
		return fmt.Errorf("open parquet file failed: %w", err)
	}
	defer fr.Close()

	pr, err := pq.NewParquetReader(fr, nil, 1000)
	if err != nil {
		return fmt.Errorf("create parquet reader failed: %w", err)
	}
	defer pr.ReadStop()

	targetMonth := billingDate.Format("2006-01")
	batch := make([]map[string]interface{}, 0, 1000)
	numRows := int(pr.GetNumRows())

	log.Printf("[AWSSync] parquet file has %d rows, targetMonth=%s", numRows, targetMonth)

	filtered := 0
	printedKeys := false
	for i := 0; i < numRows; i += 1000 {
		rows, err := pr.ReadByNumber(1000)
		if err != nil {
			return fmt.Errorf("read parquet rows failed: %w", err)
		}
		for _, row := range rows {
			typed := parquetRowToMap(row)

			// 打印第一条记录的字段名
			if !printedKeys {
				keys := make([]string, 0, len(typed))
				for k := range typed {
					keys = append(keys, k)
				}
				log.Printf("[AWSSync] parquet row keys (first 20): %v", keys[:min(20, len(keys))])
				printedKeys = true
			}

			ts := firstColValue(typed, "Line_item_usage_start_date", "lineItem/UsageStartDate", "UsageStartDate")
			if strings.TrimSpace(ts) != "" {
				dt, err := time.Parse("2006-01-02T15:04:05Z", ts)
				if err != nil {
					dt, err = time.Parse("2006-01-02T15:04:05", ts)
				}
				if err != nil {
					dt, err = time.Parse("2006-01-02", ts)
				}
				if err == nil && dt.Format("2006-01") != targetMonth {
					filtered++
					continue
				}
			}

			batch = append(batch, typed)
			if len(batch) >= 1000 {
				select {
				case dataCh <- batch:
					batch = make([]map[string]interface{}, 0, 1000)
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	if len(batch) > 0 {
		select {
		case dataCh <- batch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	log.Printf("[AWSSync] parquet processing done: total=%d, filtered=%d, sent=%d", numRows, filtered, numRows-filtered)
	return nil
}

// parquetRowToMap 将 parquet 行（struct 或 map）转为 map[string]interface{}
func parquetRowToMap(row interface{}) map[string]interface{} {
	if rowMap, ok := row.(map[string]interface{}); ok {
		return rowMap
	}

	// 使用反射将 struct 转为 map
	result := make(map[string]interface{})
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return result
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := v.Field(i).Interface()
		result[field.Name] = normalizeParquetValue(val)
	}
	return result
}

// normalizeParquetValue 将 parquet 返回值转为标准类型
func normalizeParquetValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// firstColValue 从 map 中取第一个存在的键值
func firstColValue(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// GetPricing implements CloudAdapter.GetPricing for AWS.
func (a *AWSAdapter) GetPricing() ([]map[string]interface{}, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(a.accessKeyID, a.secretAccessKey, ""),
	})
	if err != nil {
		return nil, fmt.Errorf("create AWS session failed: %w", err)
	}

	svc := pricing.New(sess)
	var allPrices []map[string]interface{}

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []*pricing.Filter{
			{Type: aws.String("TERM_MATCH"), Field: aws.String("servicecode"), Value: aws.String("AmazonEC2")},
		},
		MaxResults: aws.Int64(100),
	}

	apiErr := svc.GetProductsPages(input, func(page *pricing.GetProductsOutput, lastPage bool) bool {
		for _, priceItem := range page.PriceList {
			item := map[string]interface{}(priceItem)
			product, _ := item["product"].(map[string]interface{})
			terms, _ := item["terms"].(map[string]interface{})
			if product == nil || terms == nil {
				continue
			}
			attributes, _ := product["attributes"].(map[string]interface{})
			if attributes == nil {
				continue
			}

			instanceType, _ := attributes["instanceType"].(string)
			region, _ := attributes["regionCode"].(string)
			serviceCode, _ := attributes["servicecode"].(string)
			sku, _ := product["sku"].(string)

			// Extract OnDemand price
			onDemand, _ := terms["OnDemand"].(map[string]interface{})
			if onDemand != nil {
				for _, term := range onDemand {
					termMap, _ := term.(map[string]interface{})
					if termMap == nil {
						continue
					}
					priceDimensions, _ := termMap["priceDimensions"].(map[string]interface{})
					if priceDimensions == nil {
						continue
					}
					for _, dimension := range priceDimensions {
						dimMap, _ := dimension.(map[string]interface{})
						if dimMap == nil {
							continue
						}
						pricePerUnit, _ := dimMap["pricePerUnit"].(map[string]interface{})
						unit, _ := dimMap["unit"].(string)
						if pricePerUnit == nil {
							continue
						}
						for currency, priceStr := range pricePerUnit {
							price, _ := strconv.ParseFloat(fmt.Sprintf("%v", priceStr), 64)
							priceEntry := map[string]interface{}{
								"cloud_type":    "aws",
								"service_code":  serviceCode,
								"instance_type": instanceType,
								"region":        region,
								"price_per_unit": price,
								"currency":      currency,
								"unit":          unit,
								"sku":           sku,
							}
							allPrices = append(allPrices, priceEntry)
						}
					}
				}
			}
		}
		return !lastPage
	})

	return allPrices, apiErr
}
