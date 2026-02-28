package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetNodeList 获取 Node 列表
func (s *K8sService) GetNodeList(clusterID string, clusterName string, nodeID uint, envID uint) ([]*Node, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	// 构建 Kubernetes API 请求
	nodesURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/nodes"
	httpReq, client, err := s.createK8sHTTPClient(cluster, nodesURL)
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
	var nodeListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					KernelVersion           string `json:"kernelVersion"`
					OSImage                 string `json:"osImage"`
					ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
					KubeletVersion          string `json:"kubeletVersion"`
				} `json:"nodeInfo"`
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
				Allocatable map[string]string `json:"allocatable"`
				Capacity    map[string]string `json:"capacity"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &nodeListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 转换为 Node 结构
	nodes := make([]*Node, 0, len(nodeListResponse.Items))
	for _, item := range nodeListResponse.Items {
		node := &Node{
			Name:             item.Metadata.Name,
			Version:          item.Status.NodeInfo.KubeletVersion,
			OSImage:          item.Status.NodeInfo.OSImage,
			KernelVersion:    item.Status.NodeInfo.KernelVersion,
			ContainerRuntime: item.Status.NodeInfo.ContainerRuntimeVersion,
		}

		// 提取角色
		roles := make([]string, 0)
		for key := range item.Metadata.Labels {
			if key == "node-role.kubernetes.io/master" || key == "node-role.kubernetes.io/control-plane" {
				roles = append(roles, "master")
			} else if strings.HasPrefix(key, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
				roles = append(roles, role)
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}
		node.Roles = strings.Join(roles, ",")

		// 提取状态
		ready := false
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" {
				ready = condition.Status == "True"
				break
			}
		}
		if ready {
			node.Status = "Ready"
		} else {
			node.Status = "NotReady"
		}

		// 提取 IP 地址
		for _, addr := range item.Status.Addresses {
			if addr.Type == "InternalIP" {
				node.InternalIP = addr.Address
			} else if addr.Type == "ExternalIP" {
				node.ExternalIP = addr.Address
			}
		}

		// 提取资源信息
		if cpu, ok := item.Status.Capacity["cpu"]; ok {
			node.CPU = cpu
		}
		if memory, ok := item.Status.Capacity["memory"]; ok {
			node.Memory = memory
		}

		// 计算运行时间
		if item.Metadata.CreationTimestamp != "" {
			createdTime, err := time.Parse(time.RFC3339, item.Metadata.CreationTimestamp)
			if err == nil {
				duration := time.Since(createdTime)
				if duration.Hours() >= 24 {
					node.Age = fmt.Sprintf("%dd", int(duration.Hours()/24))
				} else if duration.Hours() >= 1 {
					node.Age = fmt.Sprintf("%dh", int(duration.Hours()))
				} else {
					node.Age = fmt.Sprintf("%dm", int(duration.Minutes()))
				}
			}
		}

		// Pod 数量需要从其他 API 获取，这里先设为空
		node.Pods = "-"

		nodes = append(nodes, node)
	}

	return nodes, nil
}

