package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Secret Secret 信息
type Secret struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Data      string `json:"data"` // 数据项数
	Age       string `json:"age"`
}

// GetSecretList 获取 Secret 列表（命名空间级别）
func (s *K8sService) GetSecretList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Secret, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	secretsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/secrets"
	httpReq, client, err := s.createK8sHTTPClient(cluster, secretsURL)
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

	var secretListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Type string            `json:"type"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &secretListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	secrets := make([]*Secret, 0, len(secretListResponse.Items))
	for _, item := range secretListResponse.Items {
		secretType := item.Type
		if secretType == "" {
			secretType = "Opaque"
		}

		dataCount := len(item.Data)
		dataStr := fmt.Sprintf("%d", dataCount)

		secrets = append(secrets, &Secret{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Type:      secretType,
			Data:      dataStr,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		})
	}

	return secrets, nil
}
