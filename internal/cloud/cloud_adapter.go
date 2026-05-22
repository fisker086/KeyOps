package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/cloud/aliyun"
	"github.com/fisker086/keyops/internal/cloud/tencent"
)

const (
	CloudTypeAWS     = "aws"
	CloudTypeAliyun  = "aliyun"
	CloudTypeTencent = "tencent"
)

type CloudAdapter interface {
	ValidateCredentials() (map[string]interface{}, error)
	// GetRawBillingData 全量获取（保留兼容）
	GetRawBillingData(billingDate time.Time) ([]map[string]interface{}, error)
	// StreamRawBillingData 流式获取，边读边写，避免OOM
	StreamRawBillingData(ctx context.Context, billingDate time.Time) (<-chan []map[string]interface{}, <-chan error)
	// GetPricing 获取云资源定价信息
	GetPricing() ([]map[string]interface{}, error)
}

// normalizeCloudType maps UI / common aliases to internal adapter keys (aws, aliyun).

func normalizeCloudType(cloudType string) string {
	t := strings.ToLower(strings.TrimSpace(cloudType))
	switch t {
	case "aliyun", "alibaba", "alicloud":
		return CloudTypeAliyun
	case "tencent", "tencentcloud", "txyun", "tx":
		return CloudTypeTencent
	default:
		return t
	}
}

func NewCloudAdapter(cloudType string, config map[string]interface{}) (CloudAdapter, error) {
	switch normalizeCloudType(cloudType) {
	case CloudTypeAWS:
		return NewAWSAdapter(config), nil
	case CloudTypeAliyun:
		return aliyun.NewAliyunAdapter(config), nil
	case CloudTypeTencent:
		return tencent.NewTencentAdapter(config), nil
	default:
		return nil, fmt.Errorf("unsupported cloud type: %s", cloudType)
	}
}
