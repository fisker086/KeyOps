package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ConfigMap ConfigMap 信息
type ConfigMap struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Data      string `json:"data"` // 数据项数，如 "3" 表示 3 个 key
	Age       string `json:"age"`
}

// GetConfigMapList 获取 ConfigMap 列表（命名空间级别）
func (s *K8sService) GetConfigMapList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*ConfigMap, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	configmapsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/configmaps"
	httpReq, client, err := s.createK8sHTTPClient(cluster, configmapsURL)
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
		return nil, fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	var cmListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &cmListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	configMaps := make([]*ConfigMap, 0, len(cmListResponse.Items))
	for _, item := range cmListResponse.Items {
		dataCount := len(item.Data)
		dataStr := fmt.Sprintf("%d", dataCount)
		if dataCount == 0 {
			dataStr = "0"
		}

		configMaps = append(configMaps, &ConfigMap{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Data:      dataStr,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		})
	}

	return configMaps, nil
}
