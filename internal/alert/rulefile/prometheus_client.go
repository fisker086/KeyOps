package rulefile

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PrometheusRuleResponse Prometheus API 响应结构
type PrometheusRuleResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []PrometheusRuleGroup `json:"groups"`
	} `json:"data"`
}

// PrometheusRuleGroup Prometheus 规则组
type PrometheusRuleGroup struct {
	Name     string            `json:"name"`
	File     string            `json:"file"`
	Interval int64             `json:"interval,omitempty"`
	Rules    []PrometheusRule  `json:"rules"`
}

// PrometheusRule Prometheus 规则
// Prometheus API 返回的规则结构
type PrometheusRule struct {
	Name        string            `json:"name"`
	Query       string            `json:"query"`        // PromQL 表达式
	Duration    float64           `json:"duration"`     // 持续时间（秒，可能是浮点数）
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Health      string            `json:"health"`       // unknown, ok, err
	Type        string            `json:"type"`        // alerting, recording
	State       string            `json:"state,omitempty"` // pending, firing, inactive
}

// GetPrometheusRules 从 Prometheus API 获取规则
func GetPrometheusRules(address string) ([]PrometheusRuleGroup, error) {
	// 构建 API URL
	url := fmt.Sprintf("%s/api/v1/rules", address)
	
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// 发送请求
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求 Prometheus API 失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Prometheus API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}
	
	// 解析响应
	var ruleResponse PrometheusRuleResponse
	if err := json.NewDecoder(resp.Body).Decode(&ruleResponse); err != nil {
		return nil, fmt.Errorf("解析 Prometheus API 响应失败: %w", err)
	}
	
	if ruleResponse.Status != "success" {
		return nil, fmt.Errorf("Prometheus API 返回错误状态: %s", ruleResponse.Status)
	}
	
	return ruleResponse.Data.Groups, nil
}

