package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetServiceList 获取 Service 列表
func (s *K8sService) GetServiceList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Service, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	servicesURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/services"
	httpReq, client, err := s.createK8sHTTPClient(cluster, servicesURL)
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

	var serviceListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Type      string `json:"type"`
				ClusterIP string `json:"clusterIP"`
				Ports     []struct {
					Port       int32       `json:"port"`
					Protocol   string      `json:"protocol"`
					TargetPort interface{} `json:"targetPort"`
				} `json:"ports"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &serviceListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	services := make([]*Service, 0, len(serviceListResponse.Items))
	for _, item := range serviceListResponse.Items {
		service := &Service{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Type:      item.Spec.Type,
			ClusterIP: item.Spec.ClusterIP,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		// 格式化端口信息
		var portStrs []string
		for _, port := range item.Spec.Ports {
			var targetPort string
			switch v := port.TargetPort.(type) {
			case string:
				targetPort = v
			case float64:
				targetPort = fmt.Sprintf("%.0f", v)
			default:
				targetPort = fmt.Sprintf("%v", v)
			}
			portStrs = append(portStrs, fmt.Sprintf("%d/%s->%s", port.Port, port.Protocol, targetPort))
		}
		service.Ports = strings.Join(portStrs, ",")

		services = append(services, service)
	}

	return services, nil
}

