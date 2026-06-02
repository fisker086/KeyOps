package rulefile

import (
	"fmt"
	"net/http"
	"time"
)

// DatasourceClient 数据源客户端接口
type DatasourceClient interface {
	Reload() error
	HealthCheck() error
}

// baseDatasourceClient 公共 HTTP 客户端字段和方法
type baseDatasourceClient struct {
	Address string
	Client  *http.Client
}

func newBaseClient(address string) baseDatasourceClient {
	return baseDatasourceClient{
		Address: address,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *baseDatasourceClient) reload(path string, allowedStatuses ...int) error {
	url := fmt.Sprintf("%s%s", c.Address, path)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	for _, status := range allowedStatuses {
		if resp.StatusCode == status {
			return nil
		}
	}
	return fmt.Errorf("reload 失败，状态码: %d", resp.StatusCode)
}

func (c *baseDatasourceClient) healthCheck(path string) error {
	url := fmt.Sprintf("%s%s", c.Address, path)
	resp, err := c.Client.Get(url)
	if err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("健康检查失败，状态码: %d", resp.StatusCode)
	}
	return nil
}

// PrometheusClient Prometheus 客户端
type PrometheusClient struct {
	baseDatasourceClient
}

func NewPrometheusClient(address string) *PrometheusClient {
	return &PrometheusClient{baseDatasourceClient: newBaseClient(address)}
}

func (c *PrometheusClient) Reload() error {
	return c.reload("/-/reload", http.StatusOK)
}

func (c *PrometheusClient) HealthCheck() error {
	return c.healthCheck("/-/healthy")
}

// ThanosClient Thanos 客户端（兼容 Prometheus API）
type ThanosClient struct {
	baseDatasourceClient
}

func NewThanosClient(address string) *ThanosClient {
	return &ThanosClient{baseDatasourceClient: newBaseClient(address)}
}

func (c *ThanosClient) Reload() error {
	return c.reload("/-/reload", http.StatusOK)
}

func (c *ThanosClient) HealthCheck() error {
	return c.healthCheck("/-/healthy")
}

// VictoriaMetricsClient VictoriaMetrics 客户端
type VictoriaMetricsClient struct {
	baseDatasourceClient
}

func NewVictoriaMetricsClient(address string) *VictoriaMetricsClient {
	return &VictoriaMetricsClient{baseDatasourceClient: newBaseClient(address)}
}

func (c *VictoriaMetricsClient) Reload() error {
	return c.reload("/-/reload", http.StatusOK, http.StatusNoContent)
}

func (c *VictoriaMetricsClient) HealthCheck() error {
	return c.healthCheck("/health")
}

// NewDatasourceClient 根据数据源类型创建客户端
func NewDatasourceClient(sourceType string, address string) (DatasourceClient, error) {
	switch sourceType {
	case "prometheus":
		return NewPrometheusClient(address), nil
	case "thanos":
		return NewThanosClient(address), nil
	case "victoriametrics":
		return NewVictoriaMetricsClient(address), nil
	default:
		return nil, fmt.Errorf("不支持的数据源类型: %s", sourceType)
	}
}
