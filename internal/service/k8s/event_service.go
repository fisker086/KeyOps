package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetEventList 获取 Event 列表
func (s *K8sService) GetEventList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, objectName, objectKind string) ([]*Event, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events"
	// 如果指定了对象名称和类型，添加过滤参数
	if objectName != "" && objectKind != "" {
		eventsURL += "?fieldSelector=involvedObject.name=" + objectName + ",involvedObject.kind=" + objectKind
	}
	httpReq, client, err := s.createK8sHTTPClient(cluster, eventsURL)
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

	var eventListResponse struct {
		Items []struct {
			Type           string `json:"type"`
			Reason         string `json:"reason"`
			Message        string `json:"message"`
			Count          int32  `json:"count"`
			FirstTimestamp string `json:"firstTimestamp"`
			LastTimestamp  string `json:"lastTimestamp"`
			InvolvedObject struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"involvedObject"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &eventListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	events := make([]*Event, 0, len(eventListResponse.Items))
	for _, item := range eventListResponse.Items {
		event := &Event{
			Type:    item.Type,
			Reason:  item.Reason,
			Message: item.Message,
			Count:   int(item.Count),
			Object:  fmt.Sprintf("%s/%s", item.InvolvedObject.Kind, item.InvolvedObject.Name),
		}

		// 格式化时间
		if item.FirstTimestamp != "" {
			event.FirstSeen = formatAge(item.FirstTimestamp)
		} else {
			event.FirstSeen = "N/A"
		}
		if item.LastTimestamp != "" {
			event.LastSeen = formatAge(item.LastTimestamp)
		} else {
			event.LastSeen = "N/A"
		}

		events = append(events, event)
	}

	return events, nil
}

