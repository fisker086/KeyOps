package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PodDetail Pod 详情信息
type PodDetail struct {
	Pod
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Containers        []Container        `json:"containers"`
	Volumes           []VolumeInfo       `json:"volumes"`
	Events            []Event            `json:"events"`
	CreationTimestamp string             `json:"creationTimestamp"`
	StartTime         string             `json:"startTime"`
	QoSClass          string             `json:"qosClass"`
	ServiceAccount    string             `json:"serviceAccount"`
	NodeSelector      map[string]string `json:"nodeSelector"`
	Tolerations       []TolerationInfo  `json:"tolerations"`
}

// GetPodDetail 获取 Pod 详情
func (s *K8sService) GetPodDetail(clusterID string, clusterName string, namespace string, podName string) (*PodDetail, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
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
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
			Annotations       map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			NodeName       string            `json:"nodeName"`
			ServiceAccount string            `json:"serviceAccountName"`
			NodeSelector   map[string]string `json:"nodeSelector"`
			Containers     []struct {
				Name            string `json:"name"`
				Image           string `json:"image"`
				ImagePullPolicy string `json:"imagePullPolicy"`
				Ports           []struct {
					Name          string `json:"name"`
					ContainerPort int32  `json:"containerPort"`
					Protocol      string `json:"protocol"`
				} `json:"ports"`
				Env []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"env"`
				Resources struct {
					Requests map[string]string `json:"requests"`
					Limits   map[string]string `json:"limits"`
				} `json:"resources"`
				VolumeMounts []struct {
					Name      string `json:"name"`
					MountPath string `json:"mountPath"`
					ReadOnly  bool   `json:"readOnly"`
				} `json:"volumeMounts"`
			} `json:"containers"`
			Volumes []struct {
				Name      string `json:"name"`
				ConfigMap *struct {
					Name string `json:"name"`
				} `json:"configMap"`
				Secret *struct {
					SecretName string `json:"secretName"`
				} `json:"secret"`
				PersistentVolumeClaim *struct {
					ClaimName string `json:"claimName"`
				} `json:"persistentVolumeClaim"`
				EmptyDir *struct{} `json:"emptyDir"`
			} `json:"volumes"`
			Tolerations []struct {
				Key      string `json:"key"`
				Operator string `json:"operator"`
				Value    string `json:"value"`
				Effect   string `json:"effect"`
			} `json:"tolerations"`
		} `json:"spec"`
		Status struct {
			Phase      string `json:"phase"`
			HostIP     string `json:"hostIP"`
			PodIP      string `json:"podIP"`
			StartTime  string `json:"startTime"`
			QOSClass   string `json:"qosClass"`
			Conditions []struct {
				Type               string `json:"type"`
				Status             string `json:"status"`
				LastTransitionTime string `json:"lastTransitionTime"`
				Reason             string `json:"reason"`
				Message            string `json:"message"`
			} `json:"conditions"`
			ContainerStatuses []struct {
				Name         string `json:"name"`
				Image        string `json:"image"`
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

	// 计算重启次数
	restarts := 0
	for _, containerStatus := range podResponse.Status.ContainerStatuses {
		restarts += containerStatus.RestartCount
	}

	// 计算运行时间
	age := "N/A"
	if podResponse.Metadata.CreationTimestamp != "" {
		createdTime, err := time.Parse(time.RFC3339, podResponse.Metadata.CreationTimestamp)
		if err == nil {
			duration := time.Since(createdTime)
			if duration.Hours() >= 24 {
				age = fmt.Sprintf("%dd", int(duration.Hours()/24))
			} else if duration.Hours() >= 1 {
				age = fmt.Sprintf("%dh", int(duration.Hours()))
			} else {
				age = fmt.Sprintf("%dm", int(duration.Minutes()))
			}
		}
	}

	detail := &PodDetail{
		Pod: Pod{
			Name:      podResponse.Metadata.Name,
			Namespace: podResponse.Metadata.Namespace,
			Status:    podResponse.Status.Phase,
			Node:      podResponse.Spec.NodeName,
			Restarts:  restarts,
			Age:       age,
			PodIP:     podResponse.Status.PodIP,
			HostIP:    podResponse.Status.HostIP,
		},
		Labels:            podResponse.Metadata.Labels,
		Annotations:       podResponse.Metadata.Annotations,
		CreationTimestamp: podResponse.Metadata.CreationTimestamp,
		StartTime:         podResponse.Status.StartTime,
		QoSClass:          podResponse.Status.QOSClass,
		ServiceAccount:    podResponse.Spec.ServiceAccount,
		NodeSelector:      podResponse.Spec.NodeSelector,
	}

	// 处理 Containers
	for _, container := range podResponse.Spec.Containers {
		containerInfo := Container{
			Name:         container.Name,
			Image:        container.Image,
			RestartCount: 0,
			State:        "Unknown",
			Ready:        false,
		}

		// 查找对应的容器状态
		for _, cs := range podResponse.Status.ContainerStatuses {
			if cs.Name == container.Name {
				containerInfo.RestartCount = cs.RestartCount
				containerInfo.Ready = cs.Ready
				if cs.State.Running != nil {
					containerInfo.State = "Running"
				} else if cs.State.Waiting != nil {
					containerInfo.State = "Waiting"
				} else if cs.State.Terminated != nil {
					containerInfo.State = "Terminated"
				}
				break
			}
		}

		detail.Containers = append(detail.Containers, containerInfo)
	}

	// 处理 Volumes
	for _, volume := range podResponse.Spec.Volumes {
		volumeInfo := VolumeInfo{Name: volume.Name}
		if volume.ConfigMap != nil {
			volumeInfo.Type = "ConfigMap"
			volumeInfo.Config = map[string]interface{}{"name": volume.ConfigMap.Name}
		} else if volume.Secret != nil {
			volumeInfo.Type = "Secret"
			volumeInfo.Config = map[string]interface{}{"secretName": volume.Secret.SecretName}
		} else if volume.PersistentVolumeClaim != nil {
			volumeInfo.Type = "PersistentVolumeClaim"
			volumeInfo.Config = map[string]interface{}{"claimName": volume.PersistentVolumeClaim.ClaimName}
		} else if volume.EmptyDir != nil {
			volumeInfo.Type = "EmptyDir"
		}
		detail.Volumes = append(detail.Volumes, volumeInfo)
	}

	// 处理 Tolerations
	for _, tol := range podResponse.Spec.Tolerations {
		detail.Tolerations = append(detail.Tolerations, TolerationInfo{
			Key:      tol.Key,
			Operator: tol.Operator,
			Value:    tol.Value,
			Effect:   tol.Effect,
		})
	}

	// 获取关联的 Events
	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events?fieldSelector=involvedObject.name=" + podName + ",involvedObject.kind=Pod"
	eventsHttpReq, eventsClient, err := s.createK8sHTTPClient(cluster, eventsURL)
	if err == nil {
		eventsResp, err := eventsClient.Do(eventsHttpReq)
		if err == nil {
			defer eventsResp.Body.Close()
			if eventsResp.StatusCode == http.StatusOK {
				eventsBody, _ := io.ReadAll(eventsResp.Body)
				var eventsResponse struct {
					Items []struct {
						Type           string `json:"type"`
						Reason         string `json:"reason"`
						Message        string `json:"message"`
						Count          int32  `json:"count"`
						FirstTimestamp string `json:"firstTimestamp"`
						LastTimestamp  string `json:"lastTimestamp"`
					} `json:"items"`
				}
				if json.Unmarshal(eventsBody, &eventsResponse) == nil {
					for _, item := range eventsResponse.Items {
						detail.Events = append(detail.Events, Event{
							Type:      item.Type,
							Reason:    item.Reason,
							Message:   item.Message,
							Count:     int(item.Count),
							FirstSeen: formatAge(item.FirstTimestamp),
							LastSeen:  formatAge(item.LastTimestamp),
						})
					}
				}
			}
		}
	}

	return detail, nil
}

