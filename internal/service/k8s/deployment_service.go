package k8s

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GetDeploymentList 获取 Deployment 列表
func (s *K8sService) GetDeploymentList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Deployment, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 构建 Kubernetes API 请求 URL (Deployment 使用 apps/v1 API)
	deploymentsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments"
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentsURL)
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
	var deploymentListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Replicas            int32 `json:"replicas"`
				ReadyReplicas       int32 `json:"readyReplicas"`
				UpdatedReplicas     int32 `json:"updatedReplicas"`
				AvailableReplicas   int32 `json:"availableReplicas"`
				UnavailableReplicas int32 `json:"unavailableReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &deploymentListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 转换为 Deployment 结构
	deployments := make([]*Deployment, 0, len(deploymentListResponse.Items))
	for _, item := range deploymentListResponse.Items {
		deployment := &Deployment{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Ready:     item.Status.ReadyReplicas,
			UpToDate:  item.Status.UpdatedReplicas,
			Available: item.Status.AvailableReplicas,
		}

		// 计算运行时间
		if item.Metadata.CreationTimestamp != "" {
			createdTime, err := time.Parse(time.RFC3339, item.Metadata.CreationTimestamp)
			if err == nil {
				duration := time.Since(createdTime)
				if duration.Hours() >= 24 {
					deployment.Age = fmt.Sprintf("%dd", int(duration.Hours()/24))
				} else if duration.Hours() >= 1 {
					deployment.Age = fmt.Sprintf("%dh", int(duration.Hours()))
				} else {
					deployment.Age = fmt.Sprintf("%dm", int(duration.Minutes()))
				}
			} else {
				deployment.Age = "N/A"
			}
		} else {
			deployment.Age = "N/A"
		}

		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// GetDeploymentDetail 获取 Deployment 详情
func (s *K8sService) GetDeploymentDetail(clusterID string, clusterName string, namespace string, deploymentName string) (*DeploymentDetail, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 Deployment 详情
	deploymentURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments/" + deploymentName
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentURL)
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

	var deploymentResponse struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
			Annotations       map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Replicas *int32 `json:"replicas"`
			Selector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"selector"`
			Strategy struct {
				Type string `json:"type"`
			} `json:"strategy"`
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
			Replicas          int32 `json:"replicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
			Conditions        []struct {
				Type               string `json:"type"`
				Status             string `json:"status"`
				LastTransitionTime string `json:"lastTransitionTime"`
				Reason             string `json:"reason"`
				Message            string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &deploymentResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	detail := &DeploymentDetail{
		Deployment: Deployment{
			Name:      deploymentResponse.Metadata.Name,
			Namespace: deploymentResponse.Metadata.Namespace,
			Ready:     deploymentResponse.Status.ReadyReplicas,
			UpToDate:  deploymentResponse.Status.UpdatedReplicas,
			Available: deploymentResponse.Status.AvailableReplicas,
			Age:       formatAge(deploymentResponse.Metadata.CreationTimestamp),
		},
		Labels:            deploymentResponse.Metadata.Labels,
		Annotations:       deploymentResponse.Metadata.Annotations,
		Replicas:          deploymentResponse.Status.Replicas,
		Strategy:          deploymentResponse.Spec.Strategy.Type,
		ServiceAccount:    deploymentResponse.Spec.Template.Spec.ServiceAccountName,
		NodeSelector:      deploymentResponse.Spec.Template.Spec.NodeSelector,
		CreationTimestamp: deploymentResponse.Metadata.CreationTimestamp,
	}

	// 处理 ImagePullSecrets
	for _, secret := range deploymentResponse.Spec.Template.Spec.ImagePullSecrets {
		detail.ImagePullSecrets = append(detail.ImagePullSecrets, secret.Name)
	}

	// 处理 Containers
	for _, container := range deploymentResponse.Spec.Template.Spec.Containers {
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

	// 处理 Volumes
	for _, volume := range deploymentResponse.Spec.Template.Spec.Volumes {
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
	for _, tol := range deploymentResponse.Spec.Template.Spec.Tolerations {
		detail.Tolerations = append(detail.Tolerations, TolerationInfo{
			Key:      tol.Key,
			Operator: tol.Operator,
			Value:    tol.Value,
			Effect:   tol.Effect,
		})
	}

	// 处理 Affinity
	if deploymentResponse.Spec.Template.Spec.Affinity != nil {
		detail.Affinity = &AffinityInfo{
			NodeAffinity:    deploymentResponse.Spec.Template.Spec.Affinity,
			PodAffinity:     deploymentResponse.Spec.Template.Spec.Affinity,
			PodAntiAffinity: deploymentResponse.Spec.Template.Spec.Affinity,
		}
	}

	// 处理 Conditions
	for _, condition := range deploymentResponse.Status.Conditions {
		detail.Conditions = append(detail.Conditions, ConditionInfo{
			Type:               condition.Type,
			Status:             condition.Status,
			LastTransitionTime: condition.LastTransitionTime,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	// 获取关联的 Pods（通过 label selector）
	var selectorParts []string
	if len(deploymentResponse.Spec.Selector.MatchLabels) > 0 {
		// 使用 spec.selector.matchLabels
		for k, v := range deploymentResponse.Spec.Selector.MatchLabels {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
	} else if len(deploymentResponse.Metadata.Labels) > 0 {
		// 如果没有 selector，使用 metadata.labels 作为后备
		for k, v := range deploymentResponse.Metadata.Labels {
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

	// 获取关联的 Services（通过 label selector）
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
					for _, item := range servicesResponse.Items {
						// 检查 service selector 是否匹配 deployment selector
						matches := true
						deploymentLabels := deploymentResponse.Spec.Selector.MatchLabels
						if len(deploymentLabels) == 0 {
							deploymentLabels = deploymentResponse.Metadata.Labels
						}
						for k, v := range item.Spec.Selector {
							if deploymentLabels[k] != v {
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

	// 获取关联的 Events
	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events?fieldSelector=involvedObject.name=" + deploymentName + ",involvedObject.kind=Deployment"
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

// DeploymentRevision 表示 Deployment 的历史版本
type DeploymentRevision struct {
	Revision     int64    `json:"revision"`
	CreationTime string   `json:"creation_time"`
	ChangeReason string   `json:"change_reason"`
	Images       []string `json:"images"`
	Status       string   `json:"status"` // "current" or "historical"
	Replicas     int32    `json:"replicas"`
	Ready        int32    `json:"ready"`
	Available    int32    `json:"available"`
}

// GetDeploymentRevisions 获取 Deployment 的历史版本列表
func (s *K8sService) GetDeploymentRevisions(clusterID string, clusterName string, namespace string, deploymentName string) ([]*DeploymentRevision, int64, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, 0, err
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 Deployment
	deploymentURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments/" + deploymentName
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentURL)
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

	var deploymentResponse struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Selector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"selector"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(body, &deploymentResponse); err != nil {
		return nil, 0, fmt.Errorf("解析响应失败: %v", err)
	}

	currentRevisionStr := deploymentResponse.Metadata.Annotations["deployment.kubernetes.io/revision"]
	currentRevision, _ := strconv.ParseInt(currentRevisionStr, 10, 64)

	// 获取 ReplicaSets
	var selectorParts []string
	for k, v := range deploymentResponse.Spec.Selector.MatchLabels {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}
	labelSelector := strings.Join(selectorParts, ",")

	replicaSetsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/replicasets?labelSelector=" + labelSelector
	rsHttpReq, rsClient, err := s.createK8sHTTPClient(cluster, replicaSetsURL)
	if err != nil {
		return nil, 0, err
	}

	rsResp, err := rsClient.Do(rsHttpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("获取ReplicaSet列表失败: %v", err)
	}
	defer rsResp.Body.Close()

	rsBody, err := io.ReadAll(rsResp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取ReplicaSet响应失败: %v", err)
	}

	if rsResp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("获取ReplicaSet列表失败: %s", rsResp.Status)
	}

	var rsResponse struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp string            `json:"creationTimestamp"`
				Annotations       map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec struct {
				Replicas *int32 `json:"replicas"`
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				Replicas          int32 `json:"replicas"`
				ReadyReplicas     int32 `json:"readyReplicas"`
				AvailableReplicas int32 `json:"availableReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(rsBody, &rsResponse); err != nil {
		return nil, 0, fmt.Errorf("解析ReplicaSet响应失败: %v", err)
	}

	var revisions []*DeploymentRevision
	for _, rs := range rsResponse.Items {
		rsRevisionStr, exists := rs.Metadata.Annotations["deployment.kubernetes.io/revision"]
		if !exists {
			continue
		}

		revision, _ := strconv.ParseInt(rsRevisionStr, 10, 64)
		status := "historical"
		if rsRevisionStr == currentRevisionStr {
			status = "current"
		}

		var images []string
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		replicas := int32(0)
		if rs.Spec.Replicas != nil {
			replicas = *rs.Spec.Replicas
		}

		revisions = append(revisions, &DeploymentRevision{
			Revision:     revision,
			CreationTime: formatAge(rs.Metadata.CreationTimestamp),
			ChangeReason: rs.Metadata.Annotations["kubernetes.io/change-cause"],
			Images:       images,
			Status:       status,
			Replicas:     replicas,
			Ready:        rs.Status.ReadyReplicas,
			Available:    rs.Status.AvailableReplicas,
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

// RollbackDeployment 回滚 Deployment 到指定版本
func (s *K8sService) RollbackDeployment(clusterID string, clusterName string, namespace string, deploymentName string, toRevision int64) error {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return err
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 Deployment
	deploymentURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/deployments/" + deploymentName
	httpReq, client, err := s.createK8sHTTPClient(cluster, deploymentURL)
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

	var deploymentResponse struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Selector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"selector"`
			Template interface{} `json:"template"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(body, &deploymentResponse); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	// 获取 ReplicaSets
	var selectorParts []string
	for k, v := range deploymentResponse.Spec.Selector.MatchLabels {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}
	labelSelector := strings.Join(selectorParts, ",")

	replicaSetsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/apps/v1/namespaces/" + ns + "/replicasets?labelSelector=" + labelSelector
	rsHttpReq, rsClient, err := s.createK8sHTTPClient(cluster, replicaSetsURL)
	if err != nil {
		return err
	}

	rsResp, err := rsClient.Do(rsHttpReq)
	if err != nil {
		return fmt.Errorf("获取ReplicaSet列表失败: %v", err)
	}
	defer rsResp.Body.Close()

	rsBody, err := io.ReadAll(rsResp.Body)
	if err != nil {
		return fmt.Errorf("读取ReplicaSet响应失败: %v", err)
	}

	if rsResp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取ReplicaSet列表失败: %s", rsResp.Status)
	}

	var rsResponse struct {
		Items []struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec struct {
				Template interface{} `json:"template"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal(rsBody, &rsResponse); err != nil {
		return fmt.Errorf("解析ReplicaSet响应失败: %v", err)
	}

	// 查找目标版本的 ReplicaSet
	var targetTemplate interface{}
	targetRevisionStr := fmt.Sprintf("%d", toRevision)
	found := false

	for _, rs := range rsResponse.Items {
		if rs.Metadata.Annotations["deployment.kubernetes.io/revision"] == targetRevisionStr {
			targetTemplate = rs.Spec.Template
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("版本 %d 不存在", toRevision)
	}

	// 更新 Deployment 的 template
	var deploymentUpdate struct {
		Spec struct {
			Template interface{} `json:"template"`
		} `json:"spec"`
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(body, &deploymentUpdate); err != nil {
		return fmt.Errorf("解析Deployment失败: %v", err)
	}

	deploymentUpdate.Spec.Template = targetTemplate
	if deploymentUpdate.Metadata.Annotations == nil {
		deploymentUpdate.Metadata.Annotations = make(map[string]string)
	}
	deploymentUpdate.Metadata.Annotations["kubernetes.io/change-cause"] = fmt.Sprintf("kubectl rollout undo deployment/%s --to-revision=%d", deploymentName, toRevision)

	// 发送 PUT 请求更新 Deployment
	updateBody, err := json.Marshal(deploymentUpdate)
	if err != nil {
		return fmt.Errorf("序列化更新数据失败: %v", err)
	}

	updateReq, err := http.NewRequest("PUT", deploymentURL, strings.NewReader(string(updateBody)))
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
		return fmt.Errorf("更新Deployment失败: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK && updateResp.StatusCode != http.StatusCreated {
		updateBodyBytes, _ := io.ReadAll(updateResp.Body)
		return fmt.Errorf("更新Deployment失败: %s, 响应: %s", updateResp.Status, string(updateBodyBytes))
	}

	return nil
}

// GetDeploymentMetrics 获取 Deployment 的监控数据（聚合所有 Pods 的 metrics）
func (s *K8sService) GetDeploymentMetrics(clusterID string, clusterName string, namespace string, deploymentName string, lastTime, step uint) (interface{}, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, err
	}

	_ = s.getNamespace(cluster, namespace) // 用于验证命名空间

	// 获取 Deployment 详情以获取 Pods
	detail, err := s.GetDeploymentDetail(clusterID, clusterName, namespace, deploymentName)
	if err != nil {
		return nil, err
	}

	if len(detail.Pods) == 0 {
		return map[string]interface{}{
			"metrics": []interface{}{},
			"message": "Deployment 没有 Pods",
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
		"deployment": deploymentName,
		"namespace":  namespace,
		"pods":       len(detail.Pods),
		"metrics":    metricsData,
	}, nil
}
