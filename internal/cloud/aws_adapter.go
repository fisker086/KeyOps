package cloud

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/xitongsys/parquet-go-source/local"
	pq "github.com/xitongsys/parquet-go/reader"
)

const (
	// S3 下载串行（单文件占满带宽，避免多文件互相抢导致越下越慢）
	curDownloadConcurrency = 1
	// 本地 parquet 解析可并行（纯 CPU + 磁盘，不占 S3 带宽）
	curParseConcurrency = 2
	// 单文件 multipart 分片（对齐 AWS CLI 行为）
	s3DownloadPartSize    = 8 * 1024 * 1024
	s3DownloadConcurrency = 8
	// 单次下载尝试超时（卡死连接强制中断重试）
	s3DownloadAttemptTimeout = 15 * time.Minute
)

var (
	supportedRegions = map[string]bool{
		// ap-northeast-1: Asia Pacific (Tokyo)
		"ap-northeast-1": true,
		// ap-southeast-1: Asia Pacific (Singapore)
		"ap-southeast-1": true,
		// us-east-1: US East (N. Virginia)
		"us-east-1": true,
		// eu-west-1: EU (Ireland)
		"eu-west-1": true,
	}
)

// decompressGzip 将 gzip 压缩的 parquet 文件解压为临时文件
func (a *AWSAdapter) decompressGzip(gzPath string) (string, error) {
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return "", fmt.Errorf("open gzip file failed: %w", err)
	}
	defer gzFile.Close()

	gzr, err := gzip.NewReader(gzFile)
	if err != nil {
		return "", fmt.Errorf("create gzip reader failed: %w", err)
	}
	defer gzr.Close()

	out, err := os.CreateTemp("", "cur-*.snappy.parquet")
	if err != nil {
		return "", fmt.Errorf("create temp file failed: %w", err)
	}
	outPath := out.Name()

	if _, err := io.Copy(out, gzr); err != nil {
		out.Close()
		os.Remove(outPath)
		return "", fmt.Errorf("decompress gzip failed: %w", err)
	}
	out.Close()
	a.removeTempPath(gzPath)
	a.registerTempPath(outPath)
	return outPath, nil
}

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

	mu           sync.Mutex
	cfg          *aws.Config
	bucketRegion string
	s3Client     *s3.Client
	s3Downloader *manager.Downloader
	tempPaths    []string
}

var (
	awsHTTPTransportOnce sync.Once
	awsHTTPTransport     *http.Transport
)

func sharedAWSHTTPTransport() *http.Transport {
	awsHTTPTransportOnce.Do(func() {
		awsHTTPTransport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   25,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		}
	})
	return awsHTTPTransport
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
	cfg, err := a.getConfig()
	if err != nil {
		return nil, err
	}

	svc := sts.NewFromConfig(cfg)
	result, err := svc.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
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

// registerTempPath 记录待清理的本地 CUR 临时文件（MySQL 写入后由 CleanupTempFiles 删除）
func (a *AWSAdapter) registerTempPath(path string) {
	if path == "" {
		return
	}
	a.mu.Lock()
	a.tempPaths = append(a.tempPaths, path)
	a.mu.Unlock()
}

func (a *AWSAdapter) removeTempPath(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
	a.mu.Lock()
	filtered := a.tempPaths[:0]
	for _, p := range a.tempPaths {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	a.tempPaths = filtered
	a.mu.Unlock()
}

// CleanupTempFiles 删除本次同步下载的本地 CUR 临时文件
func (a *AWSAdapter) CleanupTempFiles() {
	a.mu.Lock()
	paths := append([]string(nil), a.tempPaths...)
	a.tempPaths = nil
	a.mu.Unlock()

	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("[AWSSync] remove temp file failed: %s err=%v", path, err)
		} else if err == nil {
			log.Printf("[AWSSync] removed temp file: %s", path)
		}
	}
}

// StreamRawBillingData AWS 流式解析 Parquet 文件，每 1000 行发一批，避免 OOM
func (a *AWSAdapter) StreamRawBillingData(ctx context.Context, billingDate time.Time) (<-chan []map[string]interface{}, <-chan error) {
	dataCh := make(chan []map[string]interface{}, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		defer close(dataCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		log.Printf("[AWSSync] starting sync for billingDate=%s", billingDate.Format("2006-01"))

		manifest, err := a.getManifest(billingDate)
		if err != nil {
			log.Printf("[AWSSync] getManifest failed: %v", err)
			errCh <- err
			return
		}

		log.Printf("[AWSSync] manifest fetched, reportKeys=%d", len(manifest.ReportKeys))

		// 阶段 1：串行下载（每文件独占带宽 + multipart 分片）
		type localCURFile struct {
			key  string
			path string
		}
		localFiles := make([]localCURFile, 0, len(manifest.ReportKeys))
		for i, reportKey := range manifest.ReportKeys {
			if ctx.Err() != nil {
				a.CleanupTempFiles()
				errCh <- ctx.Err()
				return
			}
			log.Printf("[AWSSync] downloading file %d/%d: %s", i+1, len(manifest.ReportKeys), reportKey)
			tmpPath, err := a.streamToTempFile(ctx, reportKey)
			if err != nil {
				a.CleanupTempFiles()
				errCh <- err
				return
			}
			localFiles = append(localFiles, localCURFile{key: reportKey, path: tmpPath})
		}
		log.Printf("[AWSSync] all %d parquet files downloaded, starting parallel parse", len(localFiles))

		// 阶段 2：并行解析本地文件（不占 S3 带宽）
		sem := make(chan struct{}, curParseConcurrency)
		var wg sync.WaitGroup
		for _, lf := range localFiles {
			sem <- struct{}{}
			wg.Add(1)
			go func(key, path string) {
				defer wg.Done()
				defer func() { <-sem }()
				log.Printf("[AWSSync] parsing parquet reportKey=%s", key)
				if err := a.parseParquetLocalFile(ctx, key, path, billingDate, dataCh); err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
				}
			}(lf.key, lf.path)
		}
		wg.Wait()
		log.Printf("[AWSSync] sync completed for billingDate=%s", billingDate.Format("2006-01"))
	}()

	return dataCh, errCh
}

// -----------------------------------------------------------------------------------
// S3基础操作
// -----------------------------------------------------------------------------------

// getConfig 获取AWS配置（单例），线程安全
func (a *AWSAdapter) getConfig() (aws.Config, error) {
	a.mu.Lock()
	if a.cfg != nil {
		defer a.mu.Unlock()
		return *a.cfg, nil
	}
	a.mu.Unlock()

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(a.region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(a.accessKeyID, a.secretAccessKey, ""),
		),
		config.WithHTTPClient(&http.Client{
			Transport: sharedAWSHTTPTransport(),
			Timeout:   30 * time.Minute,
		}),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("create AWS config failed: %w", err)
	}

	a.mu.Lock()
	a.cfg = &cfg
	a.mu.Unlock()
	return cfg, nil
}

// ensureBucketRegion 懒加载 bucketRegion，只调用一次 GetBucketLocation，线程安全
func (a *AWSAdapter) ensureBucketRegion() (string, error) {
	a.mu.Lock()
	if a.bucketRegion != "" {
		defer a.mu.Unlock()
		return a.bucketRegion, nil
	}
	a.mu.Unlock()

	cfg, err := a.getConfig()
	if err != nil {
		return "", err
	}
	s3Svc := s3.NewFromConfig(cfg)
	out, err := s3Svc.GetBucketLocation(context.Background(), &s3.GetBucketLocationInput{
		Bucket: aws.String(a.bucketName),
	})
	if err != nil {
		log.Printf("[AWSSync] GetBucketLocation failed, fallback to region=%s: %v", a.region, err)
		a.mu.Lock()
		a.bucketRegion = a.region
		a.mu.Unlock()
		return a.region, nil
	}
	location := string(out.LocationConstraint)
	if location == "" {
		a.mu.Lock()
		a.bucketRegion = "us-east-1"
		a.mu.Unlock()
		return "us-east-1", nil
	}
	a.mu.Lock()
	a.bucketRegion = location
	a.mu.Unlock()
	return location, nil
}

// ensureS3Client 懒加载并缓存 S3 client（复用连接池），线程安全
func (a *AWSAdapter) ensureS3Client() (*s3.Client, error) {
	a.mu.Lock()
	if a.s3Client != nil {
		defer a.mu.Unlock()
		return a.s3Client, nil
	}
	a.mu.Unlock()

	bucketRegion, err := a.ensureBucketRegion()
	if err != nil {
		bucketRegion = a.region
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(bucketRegion),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(a.accessKeyID, a.secretAccessKey, ""),
		),
		config.WithHTTPClient(&http.Client{
			Transport: sharedAWSHTTPTransport(),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create S3 config failed: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	a.mu.Lock()
	a.s3Client = client
	a.s3Downloader = nil
	a.mu.Unlock()
	return client, nil
}

func (a *AWSAdapter) ensureDownloader() (*manager.Downloader, error) {
	a.mu.Lock()
	if a.s3Downloader != nil {
		defer a.mu.Unlock()
		return a.s3Downloader, nil
	}
	a.mu.Unlock()

	client, err := a.ensureS3Client()
	if err != nil {
		return nil, err
	}
	dl := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = s3DownloadPartSize
		d.Concurrency = s3DownloadConcurrency
	})
	a.mu.Lock()
	a.s3Downloader = dl
	a.mu.Unlock()
	return dl, nil
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

// listObjectsWithPrefix 使用 v2 paginator 列出匹配 prefix 的 parquet 文件
func (a *AWSAdapter) listObjectsWithPrefix(s3Svc *s3.Client, bucket, listPrefix string, reportKeys *[]string) error {
	paginator := s3.NewListObjectsV2Paginator(s3Svc, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(listPrefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			key := *obj.Key
			if parquetRegex.MatchString(key) {
				*reportKeys = append(*reportKeys, key)
			}
		}
	}
	return nil
}

// listParquetFiles 列出 CUR 2.0 格式的 Parquet 文件
func (a *AWSAdapter) listParquetFiles(billingDate time.Time) (*Manifest, error) {
	s3Svc, err := a.ensureS3Client()
	if err != nil {
		return nil, fmt.Errorf("create s3 client failed: %w", err)
	}

	prefix := a.bucketPrefix
	if prefix == "" {
		prefix = "reports"
	}

	// CUR 2.0 Hive 分区格式: <prefix>/year=2026/month=5/
	year := billingDate.Format("2006")
	month := billingDate.Format("1")
	listPrefix := fmt.Sprintf("%s/year=%s/month=%s/", prefix, year, month)

	log.Printf("[AWSSync] listing parquet files with prefix: %s", listPrefix)

	var reportKeys []string
	err = a.listObjectsWithPrefix(s3Svc, a.bucketName, listPrefix, &reportKeys)
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

		err = a.listObjectsWithPrefix(s3Svc, a.bucketName, listPrefix, &reportKeys)
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

// streamToTempFile 从 S3 GetObject 流式下载到临时文件，内存友好，网络错误自动重试
func (a *AWSAdapter) streamToTempFile(ctx context.Context, key string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			log.Printf("[AWSSync] retry %d/3 for s3://%s/%s after: %v", attempt+1, a.bucketName, key, lastErr)
			select {
			case <-time.After(time.Duration(attempt*2) * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		tmpPath, err := a.streamOnce(ctx, key)
		if err == nil {
			return tmpPath, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("download failed after 3 attempts, last error: %w", lastErr)
}

func (a *AWSAdapter) streamOnce(ctx context.Context, key string) (string, error) {
	dl, err := a.ensureDownloader()
	if err != nil {
		return "", err
	}
	s3c, err := a.ensureS3Client()
	if err != nil {
		return "", err
	}
	log.Printf("[AWSSync] downloading s3://%s/%s (multipart partSize=%dMB concurrency=%d)",
		a.bucketName, key, s3DownloadPartSize/1024/1024, s3DownloadConcurrency)
	startDL := time.Now()

	var total int64
	if head, e := s3c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(a.bucketName),
		Key:    aws.String(key),
	}); e == nil && head.ContentLength != nil {
		total = *head.ContentLength
		log.Printf("[AWSSync] s3://%s/%s content-length=%d bytes", a.bucketName, key, total)
	}

	tmpFile, err := os.CreateTemp("", "cur-*.snappy.parquet")
	if err != nil {
		return "", fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()

	tracker := &progressWriterAt{
		file:    tmpFile,
		bucket:  a.bucketName,
		key:     key,
		total:   total,
		startDL: startDL,
		lastLog: startDL,
	}

	dlCtx, cancel := context.WithTimeout(ctx, s3DownloadAttemptTimeout)
	defer cancel()

	n, err := dl.Download(dlCtx, tracker, &s3.GetObjectInput{
		Bucket: aws.String(a.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("multipart download failed: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file failed: %w", err)
	}

	log.Printf("[AWSSync] downloaded %d bytes to %s from s3://%s/%s (elapsed=%s)", n, tmpPath, a.bucketName, key, time.Since(startDL))

	// .gz 文件下载后再解压
	if strings.HasSuffix(strings.ToLower(key), ".gz") {
		decompressed, err := a.decompressGzip(tmpPath)
		if err != nil {
			os.Remove(tmpPath)
			return "", err
		}
		return decompressed, nil
	}

	a.registerTempPath(tmpPath)
	return tmpPath, nil
}

// progressWriterAt 供 s3manager 并发分片写入，并节流打印进度
type progressWriterAt struct {
	file    *os.File
	bucket  string
	key     string
	total   int64
	mu      sync.Mutex
	done    int64
	startDL time.Time
	lastLog time.Time
}

func (w *progressWriterAt) WriteAt(p []byte, off int64) (int, error) {
	n, err := w.file.WriteAt(p, off)
	if n > 0 {
		w.mu.Lock()
		w.done += int64(n)
		now := time.Now()
		if now.Sub(w.lastLog) >= 10*time.Second {
			w.lastLog = now
			elapsed := now.Sub(w.startDL).Round(time.Second)
			if w.total > 0 {
				pct := float64(w.done) / float64(w.total) * 100
				if pct > 100 {
					pct = 100
				}
				log.Printf("[AWSSync] download progress: s3://%s/%s %d/%d MB (%.1f%%) (elapsed=%s)",
					w.bucket, w.key, w.done/1024/1024, w.total/1024/1024, pct, elapsed)
			} else {
				log.Printf("[AWSSync] download progress: s3://%s/%s %d MB (elapsed=%s)",
					w.bucket, w.key, w.done/1024/1024, elapsed)
			}
		}
		w.mu.Unlock()
	}
	return n, err
}

// parseParquetLocalFile 解析已下载到本地的 CUR parquet 文件，逐批发送到 dataCh
func (a *AWSAdapter) parseParquetLocalFile(ctx context.Context, reportKey, tmpPath string, billingDate time.Time, dataCh chan<- []map[string]interface{}) error {
	startParse := time.Now()
	log.Printf("[AWSSync] opening parquet file: %s", tmpPath)

	fr, err := local.NewLocalFileReader(tmpPath)
	if err != nil {
		return fmt.Errorf("open parquet file failed: %w", err)
	}
	defer fr.Close()
	log.Printf("[AWSSync] creating parquet reader for %s (elapsed=%s)", reportKey, time.Since(startParse))

	pr, err := pq.NewParquetReader(fr, nil, 1000)
	if err != nil {
		return fmt.Errorf("create parquet reader failed: %w", err)
	}
	defer pr.ReadStop()

	targetMonth := billingDate.Format("2006-01")
	batch := make([]map[string]interface{}, 0, 1000)
	numRows := int(pr.GetNumRows())
	log.Printf("[AWSSync] parquet file has %d rows (elapsed=%s)", numRows, time.Since(startParse))

	filtered := 0
	printedKeys := false
	for i := 0; i < numRows; i += 1000 {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i > 0 && i%50000 == 0 {
			log.Printf("[AWSSync] parquet progress: %s processed %d/%d rows (elapsed=%s)", reportKey, i, numRows, time.Since(startParse))
		}
		rows, err := pr.ReadByNumber(1000)
		if err != nil {
			return fmt.Errorf("read parquet rows failed: %w", err)
		}
		for _, row := range rows {
			typed := parquetRowToMap(row)
			normalizeCURFields(typed)

			// 打印第一条记录的字段名
			if !printedKeys {
				keys := make([]string, 0, len(typed))
				for k := range typed {
					keys = append(keys, k)
				}
				log.Printf("[AWSSync] parquet row keys (first 20): %v", keys[:min(20, len(keys))])
				printedKeys = true
			}

			ts := CURDayFromRow(typed, "Line_item_usage_start_date", "lineItem/UsageStartDate")
			if ts != "" {
				if len(ts) < 7 || ts[:7] != targetMonth {
					filtered++
					continue
				}
			} else if intervalDay := CURDayFromRow(typed, "Identity_time_interval", "identity/TimeInterval", "Bill_billing_period_start_date", "bill/BillingPeriodStartDate"); intervalDay != "" {
				if len(intervalDay) < 7 || intervalDay[:7] != targetMonth {
					filtered++
					continue
				}
			} else {
				// 无用量日且无账单周期 → 丢弃，避免多计
				filtered++
				continue
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

	log.Printf("[AWSSync] parquet processing done: total=%d, filtered=%d, sent=%d (elapsed=%s)", numRows, filtered, numRows-filtered, time.Since(startParse))
	return nil
}

var curFieldNormalization = map[string]string{
	// v2 path format → v1 underscored
	"lineItem/UnblendedCost":               "Line_item_unblended_cost",
	"lineItem/UnblendedRate":               "Line_item_unblended_rate",
	"lineItem/BlendedCost":                 "Line_item_blended_cost",
	"lineItem/BlendedRate":                 "Line_item_blended_rate",
	"lineItem/UsageStartDate":              "Line_item_usage_start_date",
	"lineItem/UsageEndDate":                "Line_item_usage_end_date",
	"lineItem/ProductCode":                 "Line_item_product_code",
	"lineItem/UsageAccountId":              "Line_item_usage_account_id",
	"lineItem/ResourceId":                  "Line_item_resource_id",
	"lineItem/UsageType":                   "Line_item_usage_type",
	"lineItem/Operation":                   "Line_item_operation",
	"lineItem/AvailabilityZone":            "Line_item_availability_zone",
	"lineItem/LineItemType":                "Line_item_line_item_type",
	"lineItem/CurrencyCode":                "Line_item_currency_code",
	"lineItem/NormalizedUsageAmount":       "Line_item_normalized_usage_amount",
	"lineItem/UsageAmount":                 "Line_item_usage_amount",
	"lineItem/NetUnblendedCost":            "Line_item_net_unblended_cost",
	"lineItem/NetUnblendedRate":            "Line_item_net_unblended_rate",
	"identity/TimeInterval":                "Identity_time_interval",
	"identity/LineItemId":                  "Identity_line_item_id",
	"bill/BillingPeriodStartDate":          "Bill_billing_period_start_date",
	"bill/BillingPeriodEndDate":            "Bill_billing_period_end_date",
	"bill/InvoiceId":                       "Bill_invoice_id",
	"pricing/term":                         "Pricing_term",
	"pricing/unit":                         "Pricing_unit",
	"pricing/publicOnDemandCost":           "Pricing_public_on_demand_cost",
	"pricing/publicOnDemandRate":           "Pricing_public_on_demand_rate",
	"product/ProductName":                  "Product_product_name",
	"product/productFamily":                "Product_product_family",
	"product/instanceType":                 "Product_instance_type",
	"product/instanceTypeFamily":           "Product_instance_type_family",
	"product/region":                       "Product_region",
	"product/regionCode":                   "Product_region_code",
	"product/servicecode":                  "Product_servicecode",
	"product/servicename":                  "Product_servicename",
	"product/availabilityZone":             "Product_availability_zone",
	"product/operatingSystem":              "Product_operating_system",
	"product/tenancy":                      "Product_tenancy",
	"product/licenseModel":                 "Product_license_model",
	"product/databaseEngine":               "Product_database_engine",
	"product/storageMedia":                 "Product_storage_media",
	"product/volumeType":                   "Product_volume_type",
	"product/fromRegionCode":               "Product_from_region_code",
	"product/toRegionCode":                 "Product_to_region_code",
	"product/currentGeneration":            "Product_current_generation",
	"product/leaseContractLength":          "Product_lease_contract_length",
	"product/offeringClass":                "Product_offering_class",
	"product/purchaseOption":               "Product_purchase_option",
	"reservation/ReservationARN":           "Reservation_reservation_arn",
	"reservation/EffectiveCost":            "Reservation_effective_cost",
	"savingsPlan/SavingsPlanARN":           "Savings_plan_savings_plan_arn",
	"savingsPlan/SavingsPlanRate":          "Savings_plan_savings_plan_rate",
	"savingsPlan/SavingsPlanEffectiveCost": "Savings_plan_savings_plan_effective_cost",
}

func normalizeCURFields(row map[string]interface{}) {
	for v2Name, canonical := range curFieldNormalization {
		if val, ok := row[v2Name]; ok {
			row[canonical] = val
			delete(row, v2Name)
		}
	}
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
		result[field.Name] = normalizeParquetField(field.Name, val)
	}
	return result
}

func normalizeParquetField(name string, v interface{}) interface{} {
	if isCURDateField(name) {
		if day := CURDayFromValue(v); day != "" {
			return day
		}
	}
	return normalizeParquetValue(v)
}

func isCURDateField(name string) bool {
	switch name {
	case "Line_item_usage_start_date", "Line_item_usage_end_date",
		"Bill_billing_period_start_date", "Bill_billing_period_end_date":
		return true
	default:
		return strings.HasSuffix(name, "_usage_start_date") ||
			strings.HasSuffix(name, "_usage_end_date") ||
			strings.HasSuffix(name, "_billing_period_start_date") ||
			strings.HasSuffix(name, "_billing_period_end_date")
	}
}

// normalizeParquetValue 将 parquet 返回值转为标准类型
func normalizeParquetValue(v interface{}) interface{} {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case float32:
		return float64(val)
	case float64:
		return val
	case *float64:
		if val == nil {
			return float64(0)
		}
		return *val
	case *int64:
		if val == nil {
			return int64(0)
		}
		return *val
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
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(a.accessKeyID, a.secretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create AWS config failed: %w", err)
	}

	svc := pricing.NewFromConfig(cfg)
	var allPrices []map[string]interface{}

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []types.Filter{
			{Type: types.FilterTypeTermMatch, Field: aws.String("servicecode"), Value: aws.String("AmazonEC2")},
		},
		MaxResults: aws.Int32(100),
	}

	pages := pricing.NewGetProductsPaginator(svc, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(context.Background())
		if err != nil {
			return allPrices, err
		}
		for _, priceItemStr := range page.PriceList {
			var item map[string]interface{}
			if err := json.Unmarshal([]byte(priceItemStr), &item); err != nil {
				continue
			}
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
								"cloud_type":     "aws",
								"service_code":   serviceCode,
								"instance_type":  instanceType,
								"region":         region,
								"price_per_unit": price,
								"currency":       currency,
								"unit":           unit,
								"sku":            sku,
							}
							allPrices = append(allPrices, priceEntry)
						}
					}
				}
			}
		}
	}

	return allPrices, nil
}
