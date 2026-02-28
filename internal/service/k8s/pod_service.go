package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetPodList 获取 Pod 列表
func (s *K8sService) GetPodList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Pod, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 构建 Kubernetes API 请求 URL
	podsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods"
	httpReq, client, err := s.createK8sHTTPClient(cluster, podsURL)
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

	// 解析 Kubernetes API 响应
	var podListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				NodeName string `json:"nodeName"`
			} `json:"spec"`
			Status struct {
				Phase      string `json:"phase"`
				HostIP     string `json:"hostIP"`
				PodIP      string `json:"podIP"`
				StartTime  string `json:"startTime"`
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				ContainerStatuses []struct {
					RestartCount int `json:"restartCount"`
					State        struct {
						Running    interface{} `json:"running"`
						Waiting    interface{} `json:"waiting"`
						Terminated interface{} `json:"terminated"`
					} `json:"state"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &podListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 转换为 Pod 结构
	pods := make([]*Pod, 0, len(podListResponse.Items))
	for _, item := range podListResponse.Items {
		pod := &Pod{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Status:    item.Status.Phase,
			Node:      item.Spec.NodeName,
			PodIP:     item.Status.PodIP,
			HostIP:    item.Status.HostIP,
		}

		// 计算重启次数
		restarts := 0
		for _, containerStatus := range item.Status.ContainerStatuses {
			restarts += containerStatus.RestartCount
		}
		pod.Restarts = restarts

		// 计算运行时间
		if item.Metadata.CreationTimestamp != "" {
			createdTime, err := time.Parse(time.RFC3339, item.Metadata.CreationTimestamp)
			if err == nil {
				duration := time.Since(createdTime)
				if duration.Hours() >= 24 {
					pod.Age = fmt.Sprintf("%dd", int(duration.Hours()/24))
				} else if duration.Hours() >= 1 {
					pod.Age = fmt.Sprintf("%dh", int(duration.Hours()))
				} else {
					pod.Age = fmt.Sprintf("%dm", int(duration.Minutes()))
				}
			} else {
				pod.Age = "N/A"
			}
		} else {
			pod.Age = "N/A"
		}

		pods = append(pods, pod)
	}

	return pods, nil
}

