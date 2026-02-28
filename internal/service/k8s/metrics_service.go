package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetPodMetrics 获取Pod指标
func (s *K8sService) GetPodMetrics(clusterID string, clusterName string, namespace string, podName, metricsName string, lastTime, step uint) (interface{}, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	if namespace == "" || podName == "" || metricsName == "" {
		return nil, fmt.Errorf("namespace, pod_name 和 metrics_name 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)

	// 构建 metrics API URL
	// Kubernetes metrics-server API: /apis/metrics.k8s.io/v1beta1/namespaces/{namespace}/pods/{pod}
	metricsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/metrics.k8s.io/v1beta1/namespaces/" + ns + "/pods/" + podName
	httpReq, client, err := s.createK8sHTTPClient(cluster, metricsURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 如果 metrics-server 不可用，返回空数据而不是错误
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
			return map[string]interface{}{
				"metrics": []interface{}{},
				"message": "Metrics server 不可用",
			}, nil
		}
		return nil, fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	var metricsResponse struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Timestamp string `json:"timestamp"`
		Window    string `json:"window"`
		Containers []struct {
			Name  string `json:"name"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	}

	if err := json.Unmarshal(body, &metricsResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 根据 metricsName 返回对应的指标
	result := make(map[string]interface{})
	result["pod"] = metricsResponse.Metadata.Name
	result["namespace"] = metricsResponse.Metadata.Namespace
	result["timestamp"] = metricsResponse.Timestamp
	result["window"] = metricsResponse.Window

	// 查找指定容器的指标
	for _, container := range metricsResponse.Containers {
		if metricsName == "cpu" {
			result["value"] = container.Usage.CPU
			result["unit"] = "cores"
		} else if metricsName == "memory" {
			result["value"] = container.Usage.Memory
			result["unit"] = "bytes"
		}
	}

	// 如果指定了容器名称，只返回该容器的指标
	// 否则返回所有容器的指标
	if len(metricsResponse.Containers) > 0 {
		result["containers"] = metricsResponse.Containers
	}

	return result, nil
}

