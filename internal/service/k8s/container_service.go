package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetContainersList 获取容器列表
func (s *K8sService) GetContainersList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string, podName string) ([]*Container, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	if podName == "" {
		return nil, fmt.Errorf("pod_name 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 Pod 详情
	podURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods/" + podName
	httpReq, client, err := s.createK8sHTTPClient(cluster, podURL)
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

	var podResponse struct {
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			ContainerStatuses []struct {
				Name         string `json:"name"`
				Ready        bool   `json:"ready"`
				RestartCount int    `json:"restartCount"`
				State        struct {
					Running    interface{} `json:"running"`
					Waiting    interface{} `json:"waiting"`
					Terminated interface{} `json:"terminated"`
				} `json:"state"`
			} `json:"containerStatuses"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &podResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 创建容器名称到镜像的映射
	imageMap := make(map[string]string)
	for _, container := range podResponse.Spec.Containers {
		imageMap[container.Name] = container.Image
	}

	// 转换为 Container 结构
	containers := make([]*Container, 0, len(podResponse.Status.ContainerStatuses))
	for _, status := range podResponse.Status.ContainerStatuses {
		container := &Container{
			Name:         status.Name,
			Image:        imageMap[status.Name],
			Ready:        status.Ready,
			RestartCount: status.RestartCount,
		}

		// 确定容器状态
		if status.State.Running != nil {
			container.State = "Running"
		} else if status.State.Waiting != nil {
			container.State = "Waiting"
		} else if status.State.Terminated != nil {
			container.State = "Terminated"
		} else {
			container.State = "Unknown"
		}

		containers = append(containers, container)
	}

	return containers, nil
}

