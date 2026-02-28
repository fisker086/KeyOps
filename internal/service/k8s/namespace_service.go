package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// GetNamespaceList 获取命名空间列表
func (s *K8sService) GetNamespaceList(clusterID string, clusterName string) ([]*Namespace, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	namespacesURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces"
	httpReq, client, err := s.createK8sHTTPClient(cluster, namespacesURL)
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

	var namespaceListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &namespaceListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	namespaces := make([]*Namespace, 0, len(namespaceListResponse.Items))
	for _, item := range namespaceListResponse.Items {
		namespace := &Namespace{
			Name:              item.Metadata.Name,
			Status:            item.Status.Phase,
			Labels:            item.Metadata.Labels,
			CreationTimestamp: item.Metadata.CreationTimestamp,
			Age:               formatAge(item.Metadata.CreationTimestamp),
		}
		namespaces = append(namespaces, namespace)
	}

	// 按名称排序：default 放首位，其它按字母排序
	sort.Slice(namespaces, func(i, j int) bool {
		ni, nj := namespaces[i].Name, namespaces[j].Name
		if ni == "default" {
			return true
		}
		if nj == "default" {
			return false
		}
		return strings.Compare(ni, nj) < 0
	})

	return namespaces, nil
}
