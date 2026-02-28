package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetJobList 获取 Job 列表
func (s *K8sService) GetJobList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*Job, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	jobURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/batch/v1/namespaces/" + ns + "/jobs"
	httpReq, client, err := s.createK8sHTTPClient(cluster, jobURL)
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

	var jobListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Completions *int32 `json:"completions"`
			} `json:"spec"`
			Status struct {
				Succeeded      int32   `json:"succeeded"`
				Failed         int32   `json:"failed"`
				StartTime      string  `json:"startTime"`
				CompletionTime *string `json:"completionTime"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &jobListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	jobs := make([]*Job, 0, len(jobListResponse.Items))
	for _, item := range jobListResponse.Items {
		job := &Job{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		if item.Spec.Completions != nil {
			job.Completions = *item.Spec.Completions
		} else {
			job.Completions = 1
		}

		// 计算持续时间
		if item.Status.StartTime != "" {
			startTime, err := time.Parse(time.RFC3339, item.Status.StartTime)
			if err == nil {
				var endTime time.Time
				if item.Status.CompletionTime != nil && *item.Status.CompletionTime != "" {
					endTime, err = time.Parse(time.RFC3339, *item.Status.CompletionTime)
					if err != nil {
						endTime = time.Now()
					}
				} else {
					endTime = time.Now()
				}
				duration := endTime.Sub(startTime)
				if duration.Hours() >= 1 {
					job.Duration = fmt.Sprintf("%.0fh%.0fm", duration.Hours(), duration.Minutes()-(duration.Hours()*60))
				} else {
					job.Duration = fmt.Sprintf("%.0fm", duration.Minutes())
				}
			} else {
				job.Duration = "N/A"
			}
		} else {
			job.Duration = "N/A"
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJobDetail 获取 Job 详情
func (s *K8sService) GetJobDetail(clusterID string, clusterName string, namespace string, jobName string) (*JobDetail, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 Job 详情
	jobURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/batch/v1/namespaces/" + ns + "/jobs/" + jobName
	httpReq, client, err := s.createK8sHTTPClient(cluster, jobURL)
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

	var jobResponse struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
			Annotations       map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Completions  *int32 `json:"completions"`
			Parallelism  *int32 `json:"parallelism"`
			BackoffLimit *int32 `json:"backoffLimit"`
			Selector     struct {
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
			Succeeded      int32   `json:"succeeded"`
			Failed         int32   `json:"failed"`
			Active         int32   `json:"active"`
			StartTime      string  `json:"startTime"`
			CompletionTime *string `json:"completionTime"`
			Conditions     []struct {
				Type               string `json:"type"`
				Status             string `json:"status"`
				LastTransitionTime string `json:"lastTransitionTime"`
				Reason             string `json:"reason"`
				Message            string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &jobResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	detail := &JobDetail{
		Job: Job{
			Name:      jobResponse.Metadata.Name,
			Namespace: jobResponse.Metadata.Namespace,
			Age:       formatAge(jobResponse.Metadata.CreationTimestamp),
		},
		Labels:            jobResponse.Metadata.Labels,
		Annotations:       jobResponse.Metadata.Annotations,
		Succeeded:         jobResponse.Status.Succeeded,
		Failed:            jobResponse.Status.Failed,
		Active:            jobResponse.Status.Active,
		Completions:       jobResponse.Spec.Completions,
		Parallelism:       jobResponse.Spec.Parallelism,
		BackoffLimit:     jobResponse.Spec.BackoffLimit,
		StartTime:         jobResponse.Status.StartTime,
		CompletionTime:    jobResponse.Status.CompletionTime,
		ServiceAccount:    jobResponse.Spec.Template.Spec.ServiceAccountName,
		NodeSelector:     jobResponse.Spec.Template.Spec.NodeSelector,
		CreationTimestamp: jobResponse.Metadata.CreationTimestamp,
	}

	// 计算持续时间
	if jobResponse.Status.StartTime != "" {
		startTime, err := time.Parse(time.RFC3339, jobResponse.Status.StartTime)
		if err == nil {
			var endTime time.Time
			if jobResponse.Status.CompletionTime != nil && *jobResponse.Status.CompletionTime != "" {
				endTime, err = time.Parse(time.RFC3339, *jobResponse.Status.CompletionTime)
				if err != nil {
					endTime = time.Now()
				}
			} else {
				endTime = time.Now()
			}
			duration := endTime.Sub(startTime)
			if duration.Hours() >= 1 {
				detail.Duration = fmt.Sprintf("%.0fh%.0fm", duration.Hours(), duration.Minutes()-(duration.Hours()*60))
			} else {
				detail.Duration = fmt.Sprintf("%.0fm", duration.Minutes())
			}
		} else {
			detail.Duration = "N/A"
		}
	} else {
		detail.Duration = "N/A"
	}

	// 处理 ImagePullSecrets
	for _, secret := range jobResponse.Spec.Template.Spec.ImagePullSecrets {
		detail.ImagePullSecrets = append(detail.ImagePullSecrets, secret.Name)
	}

	// 处理 Containers
	for _, container := range jobResponse.Spec.Template.Spec.Containers {
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
	for _, volume := range jobResponse.Spec.Template.Spec.Volumes {
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
	for _, tol := range jobResponse.Spec.Template.Spec.Tolerations {
		detail.Tolerations = append(detail.Tolerations, TolerationInfo{
			Key:      tol.Key,
			Operator: tol.Operator,
			Value:    tol.Value,
			Effect:   tol.Effect,
		})
	}

	// 处理 Affinity
	if jobResponse.Spec.Template.Spec.Affinity != nil {
		detail.Affinity = &AffinityInfo{
			NodeAffinity:    jobResponse.Spec.Template.Spec.Affinity,
			PodAffinity:     jobResponse.Spec.Template.Spec.Affinity,
			PodAntiAffinity: jobResponse.Spec.Template.Spec.Affinity,
		}
	}

	// 处理 Conditions
	for _, condition := range jobResponse.Status.Conditions {
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
	if len(jobResponse.Spec.Selector.MatchLabels) > 0 {
		for k, v := range jobResponse.Spec.Selector.MatchLabels {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
	} else if len(jobResponse.Metadata.Labels) > 0 {
		for k, v := range jobResponse.Metadata.Labels {
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
								PodIP             string `json:"podIP"`
								HostIP            string `json:"hostIP"`
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
								PodIP:     item.Status.PodIP,
								HostIP:    item.Status.HostIP,
							})
						}
					}
				}
			}
		}
	}

	// 获取关联的 Events
	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events?fieldSelector=involvedObject.name=" + jobName + ",involvedObject.kind=Job"
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

