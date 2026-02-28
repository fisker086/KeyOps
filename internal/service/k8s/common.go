package k8s

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// K8sService K8s 服务
type K8sService struct {
	clusterRepo *repository.K8sClusterRepository
}

// NewK8sService 创建 K8s 服务
func NewK8sService(clusterRepo *repository.K8sClusterRepository) *K8sService {
	return &K8sService{
		clusterRepo: clusterRepo,
	}
}

// GetClusterConfig 根据 cluster_id 或 cluster_name 获取集群配置
func (s *K8sService) GetClusterConfig(clusterID string, clusterName string) (*model.K8sCluster, error) {
	var cluster *model.K8sCluster
	var err error

	if clusterID != "" {
		cluster, err = s.clusterRepo.FindByID(clusterID)
	} else if clusterName != "" {
		cluster, err = s.clusterRepo.FindByName(clusterName)
	} else {
		return nil, fmt.Errorf("cluster_id 或 cluster_name 必须提供一个")
	}

	if err != nil {
		return nil, fmt.Errorf("获取集群配置失败: %v", err)
	}

	if cluster == nil {
		return nil, fmt.Errorf("集群不存在")
	}

	if cluster.Status != "active" {
		return nil, fmt.Errorf("集群 %s 状态为 %s，无法使用", cluster.Name, cluster.Status)
	}

	return cluster, nil
}

// createK8sHTTPClient 创建 K8s HTTP 客户端（辅助函数）
func (s *K8sService) createK8sHTTPClient(cluster *model.K8sCluster, url string) (*http.Request, *http.Client, error) {
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var client *http.Client
	var tlsConfig *tls.Config

	if cluster.AuthType == "token" && cluster.Token != "" {
		// Token 认证
		httpReq.Header.Set("Authorization", "Bearer "+cluster.Token)
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		// Kubeconfig 认证
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return nil, nil, fmt.Errorf("解析Kubeconfig失败: %v", err)
		}

		// 设置认证头或证书
		if authInfo.Token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+authInfo.Token)
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			// 使用客户端证书认证
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return nil, nil, fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			return nil, nil, fmt.Errorf("Kubeconfig中未找到有效的认证信息")
		}
	} else {
		return nil, nil, fmt.Errorf("缺少认证信息，无法连接集群")
	}

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 10 * time.Second,
	}

	return httpReq, client, nil
}

// formatAge 格式化运行时间（辅助函数）
func formatAge(creationTimestamp string) string {
	if creationTimestamp == "" {
		return "N/A"
	}
	createdTime, err := time.Parse(time.RFC3339, creationTimestamp)
	if err != nil {
		return "N/A"
	}
	duration := time.Since(createdTime)
	if duration.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	} else if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
}

// getNamespace 获取命名空间，如果为空则使用默认值
func (s *K8sService) getNamespace(cluster *model.K8sCluster, namespace string) string {
	ns := namespace
	if ns == "" && cluster != nil {
		ns = cluster.DefaultNamespace
	}
	if ns == "" {
		ns = "default"
	}
	return ns
}

// 公共数据结构定义
// BaseInfo 基础信息
type BaseInfo struct {
	Cluster      string `json:"cluster"`
	Namespace    string `json:"namespace"`
	NodeCount    int    `json:"nodeCount"`
	PodCount     int    `json:"podCount"`
	ServiceCount int    `json:"serviceCount"`
	IngressCount int    `json:"ingressCount"`
}

// Pod Pod 信息
type Pod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Node      string `json:"node"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
	PodIP     string `json:"podIP"`  // Pod IP 地址
	HostIP    string `json:"hostIP"` // Host IP 地址
}

// Service Service 信息
type Service struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"clusterIP"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

// Ingress Ingress 信息
type Ingress struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Address   string `json:"address"`
	Hosts     string `json:"hosts"`
	Age       string `json:"age"`
}

// HPA HPA 信息
type HPA struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Reference string `json:"reference"`
	Targets   string `json:"targets"`
	MinPods   int    `json:"minPods"`
	MaxPods   int    `json:"maxPods"`
	Replicas  int    `json:"replicas"`
	Age       string `json:"age"`
}

// Event Event 信息
type Event struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Count     int    `json:"count"`
}

// Container Container 信息
type Container struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
	State        string `json:"state"`
}

// ReplicaCounts 副本数信息
type ReplicaCounts struct {
	DesiredReplicas int32 `json:"desired_replicas"`
	ActualReplicas  int32 `json:"actual_replicas"`
	Desired         int32 `json:"desired,omitempty"`
	Current         int32 `json:"current,omitempty"`
	Ready           int32 `json:"ready,omitempty"`
	Available       int32 `json:"available,omitempty"`
}

// Deployment Deployment 信息
type Deployment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     int32  `json:"ready"`
	UpToDate  int32  `json:"upToDate"`
	Available int32  `json:"available"`
	Age       string `json:"age"`
}

// DeploymentDetail Deployment 详情信息
type DeploymentDetail struct {
	Deployment
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Replicas          int32             `json:"replicas"`
	Strategy          string            `json:"strategy"`
	ImagePullSecrets  []string          `json:"imagePullSecrets"`
	Containers        []ContainerInfo   `json:"containers"`
	Volumes           []VolumeInfo      `json:"volumes"`
	ServiceAccount    string            `json:"serviceAccount"`
	NodeSelector      map[string]string `json:"nodeSelector"`
	Tolerations       []TolerationInfo  `json:"tolerations"`
	Affinity          *AffinityInfo     `json:"affinity"`
	Pods              []Pod             `json:"pods"`
	Services          []Service         `json:"services"`
	Ingresses         []Ingress         `json:"ingresses"`
	Events            []Event           `json:"events"`
	Conditions        []ConditionInfo   `json:"conditions"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	ImagePullPolicy string               `json:"imagePullPolicy"`
	Ports           []ContainerPort      `json:"ports"`
	Env             []EnvVar             `json:"env"`
	Resources       ResourceRequirements `json:"resources"`
	VolumeMounts    []VolumeMount        `json:"volumeMounts"`
}

// ContainerPort 容器端口
type ContainerPort struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// EnvVar 环境变量
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResourceRequirements 资源需求
type ResourceRequirements struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// TolerationInfo 容忍度信息
type TolerationInfo struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Effect   string `json:"effect"`
}

// AffinityInfo 亲和性信息
type AffinityInfo struct {
	NodeAffinity    interface{} `json:"nodeAffinity"`
	PodAffinity     interface{} `json:"podAffinity"`
	PodAntiAffinity interface{} `json:"podAntiAffinity"`
}

// ConditionInfo 条件信息
type ConditionInfo struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
}

// DaemonSet DaemonSet 信息
type DaemonSet struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Desired   int32  `json:"desired"`
	Current   int32  `json:"current"`
	Ready     int32  `json:"ready"`
	UpToDate  int32  `json:"upToDate"`
	Available int32  `json:"available"`
	Age       string `json:"age"`
}

// StatefulSet StatefulSet 信息
type StatefulSet struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     int32  `json:"ready"`
	Age       string `json:"age"`
}

// CronJob CronJob 信息
type CronJob struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Schedule     string `json:"schedule"`
	Suspend      bool   `json:"suspend"`
	Active       int32  `json:"active"`
	LastSchedule string `json:"lastSchedule"`
	Age          string `json:"age"`
}

// Job Job 信息
type Job struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Completions int32  `json:"completions"`
	Duration    string `json:"duration"`
	Age         string `json:"age"`
}

// JobDetail Job 详情信息
type JobDetail struct {
	Job
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Succeeded         int32             `json:"succeeded"`
	Failed            int32             `json:"failed"`
	Active            int32             `json:"active"`
	Completions       *int32            `json:"completions"`
	Parallelism       *int32            `json:"parallelism"`
	BackoffLimit      *int32            `json:"backoffLimit"`
	StartTime         string            `json:"startTime"`
	CompletionTime    *string           `json:"completionTime"`
	ImagePullSecrets  []string          `json:"imagePullSecrets"`
	Containers        []ContainerInfo   `json:"containers"`
	Volumes           []VolumeInfo      `json:"volumes"`
	ServiceAccount    string            `json:"serviceAccount"`
	NodeSelector      map[string]string `json:"nodeSelector"`
	Tolerations       []TolerationInfo  `json:"tolerations"`
	Affinity          *AffinityInfo     `json:"affinity"`
	Pods              []Pod             `json:"pods"`
	Events            []Event           `json:"events"`
	Conditions        []ConditionInfo   `json:"conditions"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

// CronJobDetail CronJob 详情信息
type CronJobDetail struct {
	CronJob
	Labels                     map[string]string `json:"labels"`
	Annotations                map[string]string `json:"annotations"`
	Schedule                   string            `json:"schedule"`
	Suspend                    *bool             `json:"suspend"`
	ConcurrencyPolicy          string            `json:"concurrencyPolicy"`
	SuccessfulJobsHistoryLimit *int32            `json:"successfulJobsHistoryLimit"`
	FailedJobsHistoryLimit     *int32            `json:"failedJobsHistoryLimit"`
	LastScheduleTime           *string           `json:"lastScheduleTime"`
	LastSuccessfulTime         *string           `json:"lastSuccessfulTime"`
	ImagePullSecrets           []string          `json:"imagePullSecrets"`
	Containers                 []ContainerInfo   `json:"containers"`
	Volumes                    []VolumeInfo      `json:"volumes"`
	ServiceAccount             string            `json:"serviceAccount"`
	NodeSelector               map[string]string `json:"nodeSelector"`
	Tolerations                []TolerationInfo  `json:"tolerations"`
	Affinity                   *AffinityInfo     `json:"affinity"`
	Pods                       []Pod             `json:"pods"`
	RecentJobs                 []Job             `json:"recentJobs"` // 最近执行的 Jobs
	Events                     []Event           `json:"events"`
	Conditions                 []ConditionInfo   `json:"conditions"`
	CreationTimestamp          string            `json:"creationTimestamp"`
}

// Node Node 信息
type Node struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Roles            string `json:"roles"`
	Age              string `json:"age"`
	Version          string `json:"version"`
	InternalIP       string `json:"internalIP"`
	ExternalIP       string `json:"externalIP"`
	OSImage          string `json:"osImage"`
	KernelVersion    string `json:"kernelVersion"`
	ContainerRuntime string `json:"containerRuntime"`
	CPU              string `json:"cpu"`
	Memory           string `json:"memory"`
	Pods             string `json:"pods"`
}

// Namespace 命名空间信息
type Namespace struct {
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Age               string            `json:"age"`
	Labels            map[string]string `json:"labels"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

// parseCPUValue 解析 CPU 值（例如 "100m" = 0.1, "1" = 1.0）
func parseCPUValue(cpuStr string) float64 {
	cpuStr = strings.TrimSpace(cpuStr)
	if strings.HasSuffix(cpuStr, "m") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(cpuStr, "m"), 64)
		return value / 1000.0
	}
	value, _ := strconv.ParseFloat(cpuStr, 64)
	return value
}

// parseMemoryValue 解析 Memory 值（例如 "100Mi" = 104857600, "100M" = 100000000）
func parseMemoryValue(memoryStr string) float64 {
	memoryStr = strings.TrimSpace(memoryStr)
	if strings.HasSuffix(memoryStr, "Ki") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "Ki"), 64)
		return value * 1024
	}
	if strings.HasSuffix(memoryStr, "Mi") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "Mi"), 64)
		return value * 1024 * 1024
	}
	if strings.HasSuffix(memoryStr, "Gi") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "Gi"), 64)
		return value * 1024 * 1024 * 1024
	}
	if strings.HasSuffix(memoryStr, "K") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "K"), 64)
		return value * 1000
	}
	if strings.HasSuffix(memoryStr, "M") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "M"), 64)
		return value * 1000 * 1000
	}
	if strings.HasSuffix(memoryStr, "G") {
		value, _ := strconv.ParseFloat(strings.TrimSuffix(memoryStr, "G"), 64)
		return value * 1000 * 1000 * 1000
	}
	value, _ := strconv.ParseFloat(memoryStr, 64)
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
