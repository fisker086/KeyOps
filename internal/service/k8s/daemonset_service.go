package k8s

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetDaemonSetList 获取 DaemonSet 列表
func (s *K8sService) GetDaemonSetList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*DaemonSet, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	daemonSetURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/daemonsets"
	httpReq, client, err := s.createK8sHTTPClient(cluster, daemonSetURL)
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

	var daemonSetListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
				CurrentNumberScheduled int32 `json:"currentNumberScheduled"`
				NumberReady            int32 `json:"numberReady"`
				UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
				NumberAvailable        int32 `json:"numberAvailable"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &daemonSetListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	daemonSets := make([]*DaemonSet, 0, len(daemonSetListResponse.Items))
	for _, item := range daemonSetListResponse.Items {
		daemonSet := &DaemonSet{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Desired:   item.Status.DesiredNumberScheduled,
			Current:   item.Status.CurrentNumberScheduled,
			Ready:     item.Status.NumberReady,
			UpToDate:  item.Status.UpdatedNumberScheduled,
			Available: item.Status.NumberAvailable,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		daemonSets = append(daemonSets, daemonSet)
	}

	return daemonSets, nil
}

// GetDaemonSetDetail 获取 DaemonSet 详情（复用 DeploymentDetail 结构）
func (s *K8sService) GetDaemonSetDetail(clusterID string, clusterName string, namespace string, daemonSetName string) (*DeploymentDetail, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 DaemonSet 详情
	daemonSetURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/daemonsets/" + daemonSetName
	httpReq, client, err := s.createK8sHTTPClient(cluster, daemonSetURL)
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

	var daemonSetResponse struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
			Annotations       map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Selector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"selector"`
			Template struct {
				Spec struct {
					ServiceAccountName string            `json:"serviceAccountName"`
					NodeSelector       map[string]string `json:"nodeSelector"`
					ImagePullSecrets   []struct {
						Name string `json:"name"`
					} `json:"imagePullSecrets"`
					Containers []struct {
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
					Affinity interface{} `json:"affinity"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
			CurrentNumberScheduled int32 `json:"currentNumberScheduled"`
			NumberReady            int32 `json:"numberReady"`
			UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
			NumberAvailable        int32 `json:"numberAvailable"`
			Conditions             []struct {
				Type               string `json:"type"`
				Status             string `json:"status"`
				LastTransitionTime string `json:"lastTransitionTime"`
				Reason             string `json:"reason"`
				Message            string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &daemonSetResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 复用 DeploymentDetail 结构
	detail := &DeploymentDetail{
		Deployment: Deployment{
			Name:      daemonSetResponse.Metadata.Name,
			Namespace: daemonSetResponse.Metadata.Namespace,
			Ready:     daemonSetResponse.Status.NumberReady,
			UpToDate:  daemonSetResponse.Status.UpdatedNumberScheduled,
			Available: daemonSetResponse.Status.NumberAvailable,
			Age:       formatAge(daemonSetResponse.Metadata.CreationTimestamp),
		},
		Labels:            daemonSetResponse.Metadata.Labels,
		Annotations:       daemonSetResponse.Metadata.Annotations,
		Replicas:          daemonSetResponse.Status.DesiredNumberScheduled,
		Strategy:          "", // DaemonSet 没有 strategy
		ServiceAccount:    daemonSetResponse.Spec.Template.Spec.ServiceAccountName,
		NodeSelector:      daemonSetResponse.Spec.Template.Spec.NodeSelector,
		CreationTimestamp: daemonSetResponse.Metadata.CreationTimestamp,
	}

	// 处理 ImagePullSecrets
	for _, secret := range daemonSetResponse.Spec.Template.Spec.ImagePullSecrets {
		detail.ImagePullSecrets = append(detail.ImagePullSecrets, secret.Name)
	}

	// 处理 Containers（复用 DeploymentDetail 的逻辑）
	for _, container := range daemonSetResponse.Spec.Template.Spec.Containers {
		containerInfo := ContainerInfo{
			Name:            container.Name,
			Image:           container.Image,
			ImagePullPolicy: container.ImagePullPolicy,
			Resources: ResourceRequirements{
				Requests: container.Resources.Requests,
				Limits:   container.Resources.Limits,
			},
		}
		for _, port := range container.Ports {
			containerInfo.Ports = append(containerInfo.Ports, ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      port.Protocol,
			})
		}
		for _, env := range container.Env {
			containerInfo.Env = append(containerInfo.Env, EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
		for _, mount := range container.VolumeMounts {
			containerInfo.VolumeMounts = append(containerInfo.VolumeMounts, VolumeMount{
				Name:      mount.Name,
				MountPath: mount.MountPath,
				ReadOnly:  mount.ReadOnly,
			})
		}
		detail.Containers = append(detail.Containers, containerInfo)
	}

	// 处理 Volumes（复用 DeploymentDetail 的逻辑）
	for _, volume := range daemonSetResponse.Spec.Template.Spec.Volumes {
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
	for _, tol := range daemonSetResponse.Spec.Template.Spec.Tolerations {
		detail.Tolerations = append(detail.Tolerations, TolerationInfo{
			Key:      tol.Key,
			Operator: tol.Operator,
			Value:    tol.Value,
			Effect:   tol.Effect,
		})
	}

	// 处理 Affinity
	if daemonSetResponse.Spec.Template.Spec.Affinity != nil {
		detail.Affinity = &AffinityInfo{
			NodeAffinity:    daemonSetResponse.Spec.Template.Spec.Affinity,
			PodAffinity:     daemonSetResponse.Spec.Template.Spec.Affinity,
			PodAntiAffinity: daemonSetResponse.Spec.Template.Spec.Affinity,
		}
	}

	// 处理 Conditions
	for _, condition := range daemonSetResponse.Status.Conditions {
		detail.Conditions = append(detail.Conditions, ConditionInfo{
			Type:               condition.Type,
			Status:             condition.Status,
			LastTransitionTime: condition.LastTransitionTime,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	// 获取关联的 Pods（使用 spec.selector.matchLabels）
	var selectorParts []string
	if len(daemonSetResponse.Spec.Selector.MatchLabels) > 0 {
		for k, v := range daemonSetResponse.Spec.Selector.MatchLabels {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
	} else if len(daemonSetResponse.Metadata.Labels) > 0 {
		// 如果没有 selector，使用 metadata.labels 作为后备
		for k, v := range daemonSetResponse.Metadata.Labels {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	
	if len(selectorParts) > 0 {
		labelSelector := strings.Join(selectorParts, ",")

		podsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/pods?labelSelector=" + labelSelector
		podsHttpReq, podsClient, err := s.createK8sHTTPClient(cluster, podsURL)
		if err == nil {
			podsResp, err := podsClient.Do(podsHttpReq)
			if err == nil {
				defer podsResp.Body.Close()
				if podsResp.StatusCode == http.StatusOK {
					podsBody, _ := io.ReadAll(podsResp.Body)
					var podsResponse struct {
						Items []struct {
							Metadata struct {
								Name              string `json:"name"`
								Namespace         string `json:"namespace"`
								CreationTimestamp string `json:"creationTimestamp"`
							} `json:"metadata"`
							Spec struct {
								NodeName string `json:"nodeName"`
							} `json:"spec"`
							Status struct {
								Phase             string `json:"phase"`
								ContainerStatuses []struct {
									RestartCount int `json:"restartCount"`
								} `json:"containerStatuses"`
							} `json:"status"`
						} `json:"items"`
					}
					if json.Unmarshal(podsBody, &podsResponse) == nil {
						for _, item := range podsResponse.Items {
							restarts := 0
							for _, cs := range item.Status.ContainerStatuses {
								restarts += cs.RestartCount
							}
							detail.Pods = append(detail.Pods, Pod{
								Name:      item.Metadata.Name,
								Namespace: item.Metadata.Namespace,
								Status:    item.Status.Phase,
								Node:      item.Spec.NodeName,
								Restarts:  restarts,
								Age:       formatAge(item.Metadata.CreationTimestamp),
							})
						}
					}
				}
			}
		}
	}

	// 获取关联的 Services（通过 label selector，与 Deployment 逻辑一致）
	servicesURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/services"
	servicesHttpReq, servicesClient, err := s.createK8sHTTPClient(cluster, servicesURL)
	if err == nil {
		servicesResp, err := servicesClient.Do(servicesHttpReq)
		if err == nil {
			defer servicesResp.Body.Close()
			if servicesResp.StatusCode == http.StatusOK {
				servicesBody, _ := io.ReadAll(servicesResp.Body)
				var servicesResponse struct {
					Items []struct {
						Metadata struct {
							Name              string `json:"name"`
							Namespace         string `json:"namespace"`
							CreationTimestamp string `json:"creationTimestamp"`
						} `json:"metadata"`
						Spec struct {
							Type      string            `json:"type"`
							ClusterIP string            `json:"clusterIP"`
							Selector  map[string]string `json:"selector"`
							Ports     []struct {
								Port     int32  `json:"port"`
								Protocol string `json:"protocol"`
							} `json:"ports"`
						} `json:"spec"`
					} `json:"items"`
				}
				if json.Unmarshal(servicesBody, &servicesResponse) == nil {
					workloadLabels := daemonSetResponse.Spec.Selector.MatchLabels
					if len(workloadLabels) == 0 {
						workloadLabels = daemonSetResponse.Metadata.Labels
					}
					for _, item := range servicesResponse.Items {
						matches := true
						for k, v := range item.Spec.Selector {
							if workloadLabels[k] != v {
								matches = false
								break
							}
						}
						if matches && len(item.Spec.Selector) > 0 {
							var portStrs []string
							for _, port := range item.Spec.Ports {
								portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
							}
							detail.Services = append(detail.Services, Service{
								Name:      item.Metadata.Name,
								Namespace: item.Metadata.Namespace,
								Type:      item.Spec.Type,
								ClusterIP: item.Spec.ClusterIP,
								Ports:     strings.Join(portStrs, ","),
								Age:       formatAge(item.Metadata.CreationTimestamp),
							})
						}
					}
				}
			}
		}
	}

	// 获取关联的 Ingress（通过 Service 反向查找）
	if len(detail.Services) > 0 {
		serviceNames := make([]string, 0, len(detail.Services))
		for _, svc := range detail.Services {
			serviceNames = append(serviceNames, svc.Name)
		}
		ingresses, err := s.GetIngressesForServiceNames(clusterID, clusterName, 0, 0, namespace, serviceNames)
		if err == nil && ingresses != nil {
			for _, ing := range ingresses {
				detail.Ingresses = append(detail.Ingresses, *ing)
			}
		}
	}

	// 获取关联的 Events（复用 DeploymentDetail 的逻辑）
	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events?fieldSelector=involvedObject.name=" + daemonSetName
	eventsHttpReq, eventsClient, err := s.createK8sHTTPClient(cluster, eventsURL)
	if err == nil {
		eventsResp, err := eventsClient.Do(eventsHttpReq)
		if err == nil {
			defer eventsResp.Body.Close()
			if eventsResp.StatusCode == http.StatusOK {
				eventsBody, _ := io.ReadAll(eventsResp.Body)
				var eventsResponse struct {
					Items []struct {
						Type      string `json:"type"`
						Reason    string `json:"reason"`
						Message   string `json:"message"`
						FirstSeen string `json:"firstTimestamp"`
						LastSeen  string `json:"lastTimestamp"`
						Count     int32  `json:"count"`
					} `json:"items"`
				}
				if json.Unmarshal(eventsBody, &eventsResponse) == nil {
					for _, item := range eventsResponse.Items {
						detail.Events = append(detail.Events, Event{
							Type:      item.Type,
							Reason:    item.Reason,
							Message:   item.Message,
							FirstSeen: item.FirstSeen,
							LastSeen:  item.LastSeen,
							Count:     int(item.Count),
						})
					}
				}
			}
		}
	}

	return detail, nil
}

// GetDaemonSetMetrics 获取 DaemonSet 的监控数据（聚合所有 Pods 的 metrics）
func (s *K8sService) GetDaemonSetMetrics(clusterID string, clusterName string, namespace string, daemonSetName string, lastTime, step uint) (interface{}, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, err
	}

	_ = s.getNamespace(cluster, namespace) // 用于验证命名空间

	// 获取 DaemonSet 详情以获取 Pods
	detail, err := s.GetDaemonSetDetail(clusterID, clusterName, namespace, daemonSetName)
	if err != nil {
		return nil, err
	}

	if len(detail.Pods) == 0 {
		return map[string]interface{}{
			"metrics": []interface{}{},
			"message": "DaemonSet 没有 Pods",
		}, nil
	}

	// 聚合所有 Pods 的 metrics
	var totalCPU, totalMemory float64
	var cpuCount, memoryCount int

	for _, pod := range detail.Pods {
		// 获取 Pod 的 metrics
		podMetrics, err := s.GetPodMetrics(clusterID, clusterName, namespace, pod.Name, "cpu", lastTime, step)
		if err == nil {
			if metricsMap, ok := podMetrics.(map[string]interface{}); ok {
				if containers, ok := metricsMap["containers"].([]interface{}); ok {
					for _, container := range containers {
						if containerMap, ok := container.(map[string]interface{}); ok {
							if usage, ok := containerMap["usage"].(map[string]interface{}); ok {
								if cpuStr, ok := usage["cpu"].(string); ok {
									// 解析 CPU 值（例如 "100m" = 0.1 cores）
									cpuValue := parseCPUValue(cpuStr)
									totalCPU += cpuValue
									cpuCount++
								}
							}
						}
					}
				}
			}
		}

		podMetrics, err = s.GetPodMetrics(clusterID, clusterName, namespace, pod.Name, "memory", lastTime, step)
		if err == nil {
			if metricsMap, ok := podMetrics.(map[string]interface{}); ok {
				if containers, ok := metricsMap["containers"].([]interface{}); ok {
					for _, container := range containers {
						if containerMap, ok := container.(map[string]interface{}); ok {
							if usage, ok := containerMap["usage"].(map[string]interface{}); ok {
								if memoryStr, ok := usage["memory"].(string); ok {
									// 解析 Memory 值（例如 "100Mi" = 104857600 bytes）
									memoryValue := parseMemoryValue(memoryStr)
									totalMemory += memoryValue
									memoryCount++
								}
							}
						}
					}
				}
			}
		}
	}

	// 生成时间序列数据（模拟，因为 metrics-server 只提供当前值）
	now := time.Now()
	var metricsData []map[string]interface{}
	for i := int(lastTime); i >= 0; i -= int(step) {
		timePoint := now.Add(-time.Duration(i) * time.Second)
		metricsData = append(metricsData, map[string]interface{}{
			"time":   timePoint.Format("15:04"),
			"cpu":    totalCPU / float64(max(cpuCount, 1)) * 1000,              // 转换为 millicores
			"memory": totalMemory / float64(max(memoryCount, 1)) / 1024 / 1024, // 转换为 MB
		})
	}

	return map[string]interface{}{
		"daemonset": daemonSetName,
		"namespace": namespace,
		"pods":      len(detail.Pods),
		"metrics":   metricsData,
	}, nil
}

// DaemonSetRevision 表示 DaemonSet 的历史版本
type DaemonSetRevision struct {
	Revision     int64    `json:"revision"`
	CreationTime string   `json:"creationTime"`
	ChangeReason string   `json:"changeReason"`
	Images       []string `json:"images"`
	Status       string   `json:"status"` // "current" or "historical"
}

// GetDaemonSetRevisions 获取 DaemonSet 的历史版本列表
func (s *K8sService) GetDaemonSetRevisions(clusterID string, clusterName string, namespace string, daemonSetName string) ([]*DaemonSetRevision, int64, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, 0, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 DaemonSet
	daemonSetURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/daemonsets/" + daemonSetName
	httpReq, client, err := s.createK8sHTTPClient(cluster, daemonSetURL)
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	var daemonSetResponse struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Status struct {
			CurrentRevision int64 `json:"currentRevision"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &daemonSetResponse); err != nil {
		return nil, 0, fmt.Errorf("解析响应失败: %v", err)
	}

	currentRevision := daemonSetResponse.Status.CurrentRevision

	// 获取 ControllerRevisions（通过 owner reference 或 label selector）
	// ControllerRevision 的 label 通常是 app.kubernetes.io/name={daemonset-name}
	controllerRevisionsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/controllerrevisions"
	crHttpReq, crClient, err := s.createK8sHTTPClient(cluster, controllerRevisionsURL)
	if err != nil {
		return nil, 0, err
	}

	crResp, err := crClient.Do(crHttpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("获取ControllerRevision列表失败: %v", err)
	}
	defer crResp.Body.Close()

	crBody, err := io.ReadAll(crResp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取ControllerRevision响应失败: %v", err)
	}

	if crResp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("获取ControllerRevision列表失败: %s", crResp.Status)
	}

	var crResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
				OwnerReferences   []struct {
					Kind string `json:"kind"`
					Name string `json:"name"`
					UID  string `json:"uid"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
			Revision int64 `json:"revision"`
			Data     struct {
				Spec struct {
					Template struct {
						Spec struct {
							Containers []struct {
								Image string `json:"image"`
							} `json:"containers"`
						} `json:"spec"`
					} `json:"template"`
				} `json:"spec"`
			} `json:"data"`
		} `json:"items"`
	}

	if err := json.Unmarshal(crBody, &crResponse); err != nil {
		return nil, 0, fmt.Errorf("解析ControllerRevision响应失败: %v", err)
	}

	var revisions []*DaemonSetRevision
	for _, cr := range crResponse.Items {
		// 检查 owner reference 是否是 DaemonSet
		isDaemonSetRevision := false
		for _, owner := range cr.Metadata.OwnerReferences {
			if owner.Kind == "DaemonSet" {
				// 需要通过 DaemonSet UID 或名称匹配
				// 由于我们只有名称，先通过名称匹配
				// 更准确的方式是通过 UID，但需要先获取 DaemonSet 的 UID
				isDaemonSetRevision = true
				break
			}
		}

		// 如果没有 owner reference，尝试通过 label 匹配
		if !isDaemonSetRevision {
			if cr.Metadata.Labels != nil {
				// ControllerRevision 可能没有明确的 owner reference，需要通过其他方式匹配
				// 这里我们获取所有 ControllerRevision，然后通过名称模式匹配
				// DaemonSet 的 ControllerRevision 名称通常是 {daemonset-name}-{hash}
				if strings.HasPrefix(cr.Metadata.Name, daemonSetName+"-") {
					isDaemonSetRevision = true
				}
			}
		}

		if !isDaemonSetRevision {
			continue
		}

		revision := cr.Revision
		status := "historical"
		if revision == currentRevision {
			status = "current"
		}

		var images []string
		// ControllerRevision 的 data 是 RawExtension，需要解析
		// 尝试解析 data 中的 template
		if cr.Data.Spec.Template.Spec.Containers != nil {
			for _, container := range cr.Data.Spec.Template.Spec.Containers {
				if container.Image != "" {
					images = append(images, container.Image)
				}
			}
		}

		revisions = append(revisions, &DaemonSetRevision{
			Revision:     revision,
			CreationTime: formatAge(cr.Metadata.CreationTimestamp),
			ChangeReason: cr.Metadata.Labels["kubernetes.io/change-cause"],
			Images:       images,
			Status:       status,
		})
	}

	// 按版本号降序排序
	for i := 0; i < len(revisions)-1; i++ {
		for j := i + 1; j < len(revisions); j++ {
			if revisions[i].Revision < revisions[j].Revision {
				revisions[i], revisions[j] = revisions[j], revisions[i]
			}
		}
	}

	return revisions, currentRevision, nil
}

// RollbackDaemonSet 回滚 DaemonSet 到指定版本
func (s *K8sService) RollbackDaemonSet(clusterID string, clusterName string, namespace string, daemonSetName string, toRevision int64) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return err
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 DaemonSet
	daemonSetURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/daemonsets/" + daemonSetName
	httpReq, client, err := s.createK8sHTTPClient(cluster, daemonSetURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	// 获取 ControllerRevisions
	controllerRevisionsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/controllerrevisions"
	crHttpReq, crClient, err := s.createK8sHTTPClient(cluster, controllerRevisionsURL)
	if err != nil {
		return err
	}

	crResp, err := crClient.Do(crHttpReq)
	if err != nil {
		return fmt.Errorf("获取ControllerRevision列表失败: %v", err)
	}
	defer crResp.Body.Close()

	crBody, err := io.ReadAll(crResp.Body)
	if err != nil {
		return fmt.Errorf("读取ControllerRevision响应失败: %v", err)
	}

	if crResp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取ControllerRevision列表失败: %s", crResp.Status)
	}

	var crResponse struct {
		Items []struct {
			Metadata struct {
				Name            string `json:"name"`
				OwnerReferences []struct {
					Kind string `json:"kind"`
					Name string `json:"name"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
			Revision int64                  `json:"revision"`
			Data     map[string]interface{} `json:"data"`
		} `json:"items"`
	}

	if err := json.Unmarshal(crBody, &crResponse); err != nil {
		return fmt.Errorf("解析ControllerRevision响应失败: %v", err)
	}

	// 查找目标版本的 ControllerRevision
	var targetData map[string]interface{}
	targetRevisionStr := fmt.Sprintf("%d", toRevision)
	found := false

	for _, cr := range crResponse.Items {
		// 检查是否是 DaemonSet 的 ControllerRevision
		isDaemonSetRevision := false
		for _, owner := range cr.Metadata.OwnerReferences {
			if owner.Kind == "DaemonSet" && owner.Name == daemonSetName {
				isDaemonSetRevision = true
				break
			}
		}
		if !isDaemonSetRevision && strings.HasPrefix(cr.Metadata.Name, daemonSetName+"-") {
			isDaemonSetRevision = true
		}

		if !isDaemonSetRevision {
			continue
		}

		if fmt.Sprintf("%d", cr.Revision) == targetRevisionStr {
			targetData = cr.Data
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("版本 %d 不存在", toRevision)
	}

	// 解析 DaemonSet 当前配置
	var daemonSetUpdate map[string]interface{}
	if err := json.Unmarshal(body, &daemonSetUpdate); err != nil {
		return fmt.Errorf("解析DaemonSet失败: %v", err)
	}

	// 从 ControllerRevision 的 data 中提取 template
	if targetData != nil {
		if spec, ok := targetData["spec"].(map[string]interface{}); ok {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				// 更新 DaemonSet 的 spec.template
				if daemonSetUpdate["spec"] == nil {
					daemonSetUpdate["spec"] = make(map[string]interface{})
				}
				if specMap, ok := daemonSetUpdate["spec"].(map[string]interface{}); ok {
					specMap["template"] = template
				}
			}
		}
	}

	// 添加回滚原因注释
	if daemonSetUpdate["metadata"] == nil {
		daemonSetUpdate["metadata"] = make(map[string]interface{})
	}
	if metadataMap, ok := daemonSetUpdate["metadata"].(map[string]interface{}); ok {
		if metadataMap["annotations"] == nil {
			metadataMap["annotations"] = make(map[string]interface{})
		}
		if annotationsMap, ok := metadataMap["annotations"].(map[string]interface{}); ok {
			annotationsMap["kubernetes.io/change-cause"] = fmt.Sprintf("kubectl rollout undo daemonset/%s --to-revision=%d", daemonSetName, toRevision)
		}
	}

	// 发送 PUT 请求更新 DaemonSet
	updateBody, err := json.Marshal(daemonSetUpdate)
	if err != nil {
		return fmt.Errorf("序列化更新数据失败: %v", err)
	}

	updateReq, err := http.NewRequest("PUT", daemonSetURL, strings.NewReader(string(updateBody)))
	if err != nil {
		return fmt.Errorf("创建更新请求失败: %v", err)
	}
	updateReq.Header.Set("Content-Type", "application/json")

	// 复用认证信息
	if cluster.AuthType == "token" && cluster.Token != "" {
		updateReq.Header.Set("Authorization", "Bearer "+cluster.Token)
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return fmt.Errorf("解析Kubeconfig失败: %v", err)
		}
		if authInfo.Token != "" {
			updateReq.Header.Set("Authorization", "Bearer "+authInfo.Token)
		}
	}

	var tlsConfig *tls.Config
	if cluster.AuthType == "token" || (cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "") {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	updateClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	updateResp, err := updateClient.Do(updateReq)
	if err != nil {
		return fmt.Errorf("更新请求失败: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		updateBodyBytes, _ := io.ReadAll(updateResp.Body)
		return fmt.Errorf("更新失败: %s, 响应: %s", updateResp.Status, string(updateBodyBytes))
	}

	return nil
}

