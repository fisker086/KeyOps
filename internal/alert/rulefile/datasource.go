package rulefile

import (
	"fmt"
	"net/http"
	"time"
)

// DatasourceClient 数据源客户端接口
type DatasourceClient interface {
	// Reload 触发数据源重新加载配置
	Reload() error
	// HealthCheck 健康检查
	HealthCheck() error
}

// PrometheusClient Prometheus 客户端
type PrometheusClient struct {
	Address string // Prometheus 地址，如 http://prometheus:9090
	Client  *http.Client
}

// NewPrometheusClient 创建 Prometheus 客户端
func NewPrometheusClient(address string) *PrometheusClient {
	return &PrometheusClient{
		Address: address,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Reload 触发 Prometheus reload
func (c *PrometheusClient) Reload() error {
	url := fmt.Sprintf("%s/-/reload", c.Address)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload 失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck 健康检查
func (c *PrometheusClient) HealthCheck() error {
	url := fmt.Sprintf("%s/-/healthy", c.Address)
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

// ThanosClient Thanos 客户端（兼容 Prometheus API）
type ThanosClient struct {
	Address string
	Client  *http.Client
}

// NewThanosClient 创建 Thanos 客户端
func NewThanosClient(address string) *ThanosClient {
	return &ThanosClient{
		Address: address,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Reload 触发 Thanos reload（Thanos 使用 Prometheus API）
func (c *ThanosClient) Reload() error {
	url := fmt.Sprintf("%s/-/reload", c.Address)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload 失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck 健康检查
func (c *ThanosClient) HealthCheck() error {
	url := fmt.Sprintf("%s/-/healthy", c.Address)
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

// VictoriaMetricsClient VictoriaMetrics 客户端
// VictoriaMetrics 使用 vmalert 来执行告警规则，规则文件需要通过配置管理
type VictoriaMetricsClient struct {
	Address string
	Client  *http.Client
}

// NewVictoriaMetricsClient 创建 VictoriaMetrics 客户端
func NewVictoriaMetricsClient(address string) *VictoriaMetricsClient {
	return &VictoriaMetricsClient{
		Address: address,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Reload 触发 VictoriaMetrics vmalert reload
// vmalert 使用 /-/reload 端点（如果启用）
func (c *VictoriaMetricsClient) Reload() error {
	// VictoriaMetrics vmalert 的 reload 端点
	url := fmt.Sprintf("%s/-/reload", c.Address)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// VictoriaMetrics 可能返回 200 或 204
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("reload 失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck 健康检查
func (c *VictoriaMetricsClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", c.Address)
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

