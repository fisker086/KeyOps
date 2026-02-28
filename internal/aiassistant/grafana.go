package aiassistant

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GrafanaClient Grafana HTTP 客户端
type GrafanaClient struct {
	baseURL    string
	token      string
	verifySSL  bool
	httpClient *http.Client
}

// NewGrafanaClient 创建
func NewGrafanaClient(baseURL, token string, verifySSL bool) *GrafanaClient {
	baseURL = strings.TrimSuffix(baseURL, "/")
	client := &http.Client{}
	return &GrafanaClient{baseURL: baseURL, token: token, verifySSL: verifySSL, httpClient: client}
}

// ListAllDashboards 列出所有仪表盘
func (g *GrafanaClient) ListAllDashboards() (interface{}, error) {
	u := g.baseURL + "/api/search?type=dash-db"
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error listing dashboards: %v", err), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data []map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body), nil
	}
	compressed := make([]map[string]interface{}, 0, len(data))
	for _, item := range data {
		compressed = append(compressed, map[string]interface{}{
			"title": item["title"],
			"uid":   item["uid"],
			"tags":  item["tags"],
		})
	}
	return compressed, nil
}

// GetDashboardMetadata 获取仪表盘中的 PromQL 查询
func (g *GrafanaClient) GetDashboardMetadata(uid string) (interface{}, error) {
	u := g.baseURL + "/api/dashboards/uid/" + uid
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error getting dashboard: %v", err), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Dashboard map[string]interface{} `json:"dashboard"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	panels := result.Dashboard["panels"]
	if panels == nil {
		return []map[string]interface{}{}, nil
	}
	panelsList, _ := panels.([]interface{})
	queries := extractTargets(panelsList)
	return queries, nil
}

func extractTargets(panels []interface{}) []map[string]interface{} {
	var queries []map[string]interface{}
	for _, p := range panels {
		panel, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if targets, ok := panel["targets"].([]interface{}); ok {
			for _, t := range targets {
				target, _ := t.(map[string]interface{})
				if expr, ok := target["expr"].(string); ok && expr != "" {
					queries = append(queries, map[string]interface{}{
						"panel_title": panel["title"],
						"promql":      expr,
					})
				}
			}
		}
		if nested, ok := panel["panels"].([]interface{}); ok {
			queries = append(queries, extractTargets(nested)...)
		}
	}
	return queries
}
