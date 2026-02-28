package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetCronJobList 获取 CronJob 列表
func (s *K8sService) GetCronJobList(clusterID string, clusterName string, nodeID uint, envID uint, namespace string) ([]*CronJob, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil && (clusterID == "" && clusterName == "") {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}
	if err != nil {
		return nil, err
	}

	ns := s.getNamespace(cluster, namespace)

	cronJobURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/batch/v1/namespaces/" + ns + "/cronjobs"
	httpReq, client, err := s.createK8sHTTPClient(cluster, cronJobURL)
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

	var cronJobListResponse struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Schedule string `json:"schedule"`
				Suspend  *bool  `json:"suspend"`
			} `json:"spec"`
			Status struct {
				Active           []interface{} `json:"active"`
				LastScheduleTime string        `json:"lastScheduleTime"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &cronJobListResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	cronJobs := make([]*CronJob, 0, len(cronJobListResponse.Items))
	for _, item := range cronJobListResponse.Items {
		cronJob := &CronJob{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Schedule:  item.Spec.Schedule,
			Active:    int32(len(item.Status.Active)),
			Age:       formatAge(item.Metadata.CreationTimestamp),
		}

		if item.Spec.Suspend != nil {
			cronJob.Suspend = *item.Spec.Suspend
		}

		if item.Status.LastScheduleTime != "" {
			cronJob.LastSchedule = formatAge(item.Status.LastScheduleTime)
		} else {
			cronJob.LastSchedule = "N/A"
		}

		cronJobs = append(cronJobs, cronJob)
	}

	return cronJobs, nil
}

// GetCronJobDetail 获取 CronJob 详情
func (s *K8sService) GetCronJobDetail(clusterID string, clusterName string, namespace string, cronJobName string) (*CronJobDetail, error) {
	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		return nil, fmt.Errorf("请提供 cluster_id 或 cluster_name")
	}

	ns := s.getNamespace(cluster, namespace)

	// 获取 CronJob 详情
	cronJobURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/batch/v1/namespaces/" + ns + "/cronjobs/" + cronJobName
	httpReq, client, err := s.createK8sHTTPClient(cluster, cronJobURL)
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

	var cronJobResponse struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			Labels            map[string]string `json:"labels"`
			Annotations       map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Schedule                   string `json:"schedule"`
			Suspend                    *bool  `json:"suspend"`
			ConcurrencyPolicy          string `json:"concurrencyPolicy"`
			SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit"`
			FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit"`
			JobTemplate struct {
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
			} `json:"jobTemplate"`
		} `json:"spec"`
		Status struct {
			Active            []interface{} `json:"active"`
			LastScheduleTime  *string       `json:"lastScheduleTime"`
			LastSuccessfulTime *string      `json:"lastSuccessfulTime"`
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &cronJobResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	suspend := false
	if cronJobResponse.Spec.Suspend != nil {
		suspend = *cronJobResponse.Spec.Suspend
	}

	detail := &CronJobDetail{
		CronJob: CronJob{
			Name:      cronJobResponse.Metadata.Name,
			Namespace: cronJobResponse.Metadata.Namespace,
			Schedule:  cronJobResponse.Spec.Schedule,
			Suspend:   suspend,
			Active:    int32(len(cronJobResponse.Status.Active)),
			Age:       formatAge(cronJobResponse.Metadata.CreationTimestamp),
		},
		Labels:                    cronJobResponse.Metadata.Labels,
		Annotations:               cronJobResponse.Metadata.Annotations,
		Schedule:                  cronJobResponse.Spec.Schedule,
		Suspend:                   cronJobResponse.Spec.Suspend,
		ConcurrencyPolicy:         cronJobResponse.Spec.ConcurrencyPolicy,
		SuccessfulJobsHistoryLimit: cronJobResponse.Spec.SuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     cronJobResponse.Spec.FailedJobsHistoryLimit,
		LastScheduleTime:           cronJobResponse.Status.LastScheduleTime,
		LastSuccessfulTime:         cronJobResponse.Status.LastSuccessfulTime,
		ServiceAccount:             cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName,
		NodeSelector:              cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.NodeSelector,
		CreationTimestamp:          cronJobResponse.Metadata.CreationTimestamp,
	}

	if cronJobResponse.Status.LastScheduleTime != nil && *cronJobResponse.Status.LastScheduleTime != "" {
		detail.LastSchedule = formatAge(*cronJobResponse.Status.LastScheduleTime)
	} else {
		detail.LastSchedule = "N/A"
	}

	// 处理 ImagePullSecrets
	for _, secret := range cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets {
		detail.ImagePullSecrets = append(detail.ImagePullSecrets, secret.Name)
	}

	// 处理 Containers
	for _, container := range cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Containers {
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
	for _, volume := range cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Volumes {
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
	for _, tol := range cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Tolerations {
		detail.Tolerations = append(detail.Tolerations, TolerationInfo{
			Key:      tol.Key,
			Operator: tol.Operator,
			Value:    tol.Value,
			Effect:   tol.Effect,
		})
	}

	// 处理 Affinity
	if cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Affinity != nil {
		detail.Affinity = &AffinityInfo{
			NodeAffinity:    cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Affinity,
			PodAffinity:     cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Affinity,
			PodAntiAffinity: cronJobResponse.Spec.JobTemplate.Spec.Template.Spec.Affinity,
		}
	}

	// 获取关联的 Pods（通过 label selector）
	var selectorParts []string
	if len(cronJobResponse.Spec.JobTemplate.Spec.Selector.MatchLabels) > 0 {
		for k, v := range cronJobResponse.Spec.JobTemplate.Spec.Selector.MatchLabels {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
	} else if len(cronJobResponse.Metadata.Labels) > 0 {
		for k, v := range cronJobResponse.Metadata.Labels {
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

	// 获取最近执行的 Jobs（通过 owner reference 或 label selector）
	// 查找属于此 CronJob 的 Jobs
	jobsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/apis/batch/v1/namespaces/" + ns + "/jobs"
	jobsHttpReq, jobsClient, err := s.createK8sHTTPClient(cluster, jobsURL)
	if err == nil {
		jobsResp, err := jobsClient.Do(jobsHttpReq)
		if err == nil {
			defer jobsResp.Body.Close()
			if jobsResp.StatusCode == http.StatusOK {
				jobsBody, _ := io.ReadAll(jobsResp.Body)
				var jobsResponse struct {
					Items []struct {
						Metadata struct {
							Name              string            `json:"name"`
							Namespace         string            `json:"namespace"`
							CreationTimestamp string            `json:"creationTimestamp"`
							OwnerReferences   []struct {
								Kind string `json:"kind"`
								Name string `json:"name"`
							} `json:"ownerReferences"`
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
				if json.Unmarshal(jobsBody, &jobsResponse) == nil {
					for _, item := range jobsResponse.Items {
						// 检查是否是此 CronJob 的子 Job
						isOwned := false
						for _, owner := range item.Metadata.OwnerReferences {
							if owner.Kind == "CronJob" && owner.Name == cronJobName {
								isOwned = true
								break
							}
						}
						if isOwned {
							job := Job{
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
							detail.RecentJobs = append(detail.RecentJobs, job)
						}
					}
					// 按创建时间倒序排列（最新的在前）
					// 简单实现：按 Age 排序（实际上应该按 CreationTimestamp）
				}
			}
		}
	}

	// 获取关联的 Events
	eventsURL := strings.TrimSuffix(cluster.APIServer, "/") + "/api/v1/namespaces/" + ns + "/events?fieldSelector=involvedObject.name=" + cronJobName + ",involvedObject.kind=CronJob"
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

