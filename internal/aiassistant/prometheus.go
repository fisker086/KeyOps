package aiassistant

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PrometheusClient Prometheus HTTP 客户端
type PrometheusClient struct {
	baseURL    string
	verifySSL  bool
	httpClient *http.Client
}

// NewPrometheusClient 创建
func NewPrometheusClient(baseURL string, verifySSL bool) *PrometheusClient {
	baseURL = strings.TrimSuffix(baseURL, "/")
	client := &http.Client{Timeout: 15 * time.Second}
	if !verifySSL {
		client.Transport = &http.Transport{}
		// 生产环境可注入 InsecureSkipVerify
	}
	return &PrometheusClient{baseURL: baseURL, verifySSL: verifySSL, httpClient: client}
}

// PromQueryRangeResult 与 Prometheus query_range 返回格式一致
type PromQueryRangeResult struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// ExecutePromQLQuery 执行范围查询
func (p *PrometheusClient) ExecutePromQLQuery(query, duration, step string, startTime, endTime interface{}) (interface{}, error) {
	var start, end int64
	now := time.Now().Unix()
	if startTime != nil {
		switch v := startTime.(type) {
		case float64:
			start = int64(v)
		case int:
			start = int64(v)
		case string:
			start, _ = strconv.ParseInt(v, 10, 64)
		}
	} else {
		start = now - parseDurationSec(duration)
	}
	if endTime != nil {
		switch v := endTime.(type) {
		case float64:
			end = int64(v)
		case int:
			end = int64(v)
		case string:
			end, _ = strconv.ParseInt(v, 10, 64)
		}
	} else {
		end = now
	}
	u := p.baseURL + "/api/v1/query_range"
	reqURL := u + "?query=" + url.QueryEscape(query) + "&start=" + strconv.FormatInt(start, 10) + "&end=" + strconv.FormatInt(end, 10) + "&step=" + url.QueryEscape(step)
	resp, err := p.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Sprintf("Error executing PromQL: %v", err), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result PromQueryRangeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	if result.Status != "success" {
		return "Error: " + string(body), nil
	}
	// 转为与 Python 一致的 []map: metric + values
	out := make([]map[string]interface{}, 0, len(result.Data.Result))
	for _, r := range result.Data.Result {
		values := make([][]interface{}, 0, len(r.Values))
		for _, v := range r.Values {
			if len(v) >= 2 {
				ts, _ := toFloat(v[0])
				val, _ := toFloat(v[1])
				values = append(values, []interface{}{ts, val})
			}
		}
		out = append(out, map[string]interface{}{
			"metric": r.Metric,
			"values": values,
		})
	}
	return out, nil
}

// SummarizeResults 对查询结果做摘要以控制 token
func (p *PrometheusClient) SummarizeResults(results interface{}, totalPointsBudget int) map[string]interface{} {
	var list []map[string]interface{}
	switch v := results.(type) {
	case []map[string]interface{}:
		list = v
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				list = append(list, m)
			}
		}
	}
	if len(list) == 0 {
		return map[string]interface{}{"summary": results, "total_series": 0}
	}
	numSeries := len(list)
	pointsPerSeries := 5
	if numSeries > 50 {
		pointsPerSeries = 0
	} else if numSeries > 20 {
		pointsPerSeries = 3
	} else if numSeries > 0 {
		pointsPerSeries = totalPointsBudget / numSeries
		if pointsPerSeries < 5 {
			pointsPerSeries = 5
		}
	}
	summary := make([]map[string]interface{}, 0, numSeries)
	for _, series := range list {
		metric := series["metric"]
		valuesRaw := series["values"]
		values, _ := valuesRaw.([]interface{})
		item := map[string]interface{}{"metric": metric}
		if len(values) == 0 {
			item["info"] = "no data points"
			summary = append(summary, item)
			continue
		}
		var nums []float64
		for _, v := range values {
			pair, _ := v.([]interface{})
			if len(pair) >= 2 {
				_, f := toFloat(pair[1])
				nums = append(nums, f)
			}
		}
		if len(nums) == 0 {
			item["info"] = "data error"
			summary = append(summary, item)
			continue
		}
		min, max, avg, last := nums[0], nums[0], 0.0, nums[len(nums)-1]
		for _, n := range nums {
			if n < min {
				min = n
			}
			if n > max {
				max = n
			}
			avg += n
		}
		avg /= float64(len(nums))
		item["stats"] = map[string]interface{}{
			"min": round4(min), "max": round4(max), "avg": round4(avg), "last": round4(last), "count": len(values),
		}
		if pointsPerSeries > 0 && len(values) > 0 {
			sampled := sampleSlice(values, pointsPerSeries)
			item["sampled_values"] = sampled
		}
		summary = append(summary, item)
	}
	return map[string]interface{}{
		"summary":               summary,
		"total_series":          numSeries,
		"compression_strategy":  "full_dimensions_dynamic_sampling",
		"points_per_series":     pointsPerSeries,
	}
}

func sampleSlice(arr []interface{}, n int) []interface{} {
	if len(arr) <= n {
		return arr
	}
	out := make([]interface{}, 0, n)
	step := float64(len(arr)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		idx := int(step * float64(i))
		if idx >= len(arr) {
			idx = len(arr) - 1
		}
		out = append(out, arr[idx])
	}
	return out
}

func round4(x float64) float64 { return float64(int(x*10000+0.5)) / 10000 }
func toFloat(v interface{}) (int64, float64) {
	switch x := v.(type) {
	case float64:
		return int64(x), x
	case int:
		return int64(x), float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return int64(f), f
	}
	return 0, 0
}

// FindMetricsByKeyword 按关键字搜索指标名
func (p *PrometheusClient) FindMetricsByKeyword(keyword string) (interface{}, error) {
	u := p.baseURL + "/api/v1/label/__name__/values"
	resp, err := p.httpClient.Get(u)
	if err != nil {
		return fmt.Sprintf("Error fetching metrics: %v", err), nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Error decoding response", nil
	}
	var out []string
	for _, m := range result.Data {
		if strings.Contains(m, keyword) {
			out = append(out, m)
		}
	}
	return out, nil
}

// GetMetricDimension 查询指标的标签维度
func (p *PrometheusClient) GetMetricDimension(metricName string) (interface{}, error) {
	u := p.baseURL + "/api/v1/series?match[]=" + url.QueryEscape(metricName)
	resp, err := p.httpClient.Get(u)
	if err != nil {
		return fmt.Sprintf("Error fetching dimensions: %v", err), nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Error decoding response", nil
	}
	seen := make(map[string]bool)
	var unique []map[string]string
	for _, item := range result.Data {
		key := fmt.Sprintf("%v", item)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, item)
		if len(unique) >= 20 {
			break
		}
	}
	return unique, nil
}

func parseDurationSec(d string) int64 {
	re := regexp.MustCompile(`(\d+)([hmsd])`)
	m := re.FindStringSubmatch(d)
	if len(m) != 3 {
		return 3600
	}
	val, _ := strconv.ParseInt(m[1], 10, 64)
	unit := m[2]
	switch unit {
	case "s":
		return val
	case "m":
		return val * 60
	case "h":
		return val * 3600
	case "d":
		return val * 86400
	}
	return 3600
}
