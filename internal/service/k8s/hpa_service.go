package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetHPAList 获取 HPA 列表
func (s *K8sService) GetHPAList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*HPA, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	hpaURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/autoscaling/v2/namespaces/" + ns + "/horizontalpodautoscalers"
	httpReq, client, err := s.createK8sHTTPClient(cluster, hpaURL)
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

	var hpaListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				MinReplicas    *int32 `json:"minReplicas"`
				MaxReplicas    int32  `json:"maxReplicas"`
				ScaleTargetRef struct {
					Kind       string `json:"kind"`
					Name       string `json:"name"`
					APIVersion string `json:"apiVersion"`
				} `json:"scaleTargetRef"`
				Metrics []struct {
					Type     string `json:"type"`
					Resource *struct {
						Name   string `json:"name"`
						Target struct {
							Type               string      `json:"type"`
							AverageValue       interface{} `json:"averageValue"`
							AverageUtilization *int32      `json:"averageUtilization"`
						} `json:"target"`
					} `json:"resource"`
				} `json:"metrics"`
			} `json:"spec"`
			Status struct {
				CurrentReplicas int32 `json:"currentReplicas"`
				DesiredReplicas int32 `json:"desiredReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &hpaListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	hpas := make([]*HPA, 0, len(hpaListResponse.Items))
	for _, item := range hpaListResponse.Items {
		minReplicas := int32(1)
		if item.Spec.MinReplicas != nil {
			minReplicas = *item.Spec.MinReplicas
		}
		hpa := &HPA{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Reference: fmt.Sprintf("%s/%s", item.Spec.ScaleTargetRef.Kind, item.Spec.ScaleTargetRef.Name),
			MinPods:   int(minReplicas),
			MaxPods:   int(item.Spec.MaxReplicas),
			Replicas:  int(item.Status.CurrentReplicas),
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		// 格式化目标指标
		var targets []string
		for _, metric := range item.Spec.Metrics {
			if metric.Resource != nil {
				var targetStr string
				if metric.Resource.Target.AverageUtilization != nil {
					targetStr = fmt.Sprintf("%s:%d%%", metric.Resource.Name, *metric.Resource.Target.AverageUtilization)
				} else if metric.Resource.Target.AverageValue != nil {
					targetStr = fmt.Sprintf("%s:%v", metric.Resource.Name, metric.Resource.Target.AverageValue)
				} else {
					targetStr = metric.Resource.Name
				}
				targets = append(targets, targetStr)
			}
		}
		hpa.Targets = strings.Join(targets, ",")

		hpas = append(hpas, hpa)
	}

	return hpas, nil
}

