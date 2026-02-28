package k8s

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type K8sClusterService struct {
	clusterRepo *repository.K8sClusterRepository
	k8sService  *K8sService
}

func NewK8sClusterService(clusterRepo *repository.K8sClusterRepository) *K8sClusterService {
	return &K8sClusterService{
		clusterRepo: clusterRepo,
		k8sService:  NewK8sService(clusterRepo),
	}
}

// testClusterConnection 测试集群连接并获取版本
func (s *K8sClusterService) testClusterConnection(cluster *model.K8sCluster) (string, error) {
	// 使用HTTP请求获取Kubernetes版本
	// Kubernetes API Server提供 /version 端点
	versionURL := cluster.APIServer + "/version"

	httpReq, err := http.NewRequest("GET", versionURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 创建 HTTP 客户端
	var client *http.Client
	var tlsConfig *tls.Config

	if cluster.AuthType == "token" && cluster.Token != "" {
		// Token 认证
		httpReq.Header.Set("Authorization", "Bearer "+cluster.Token)
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		// Kubeconfig 认证
		authInfo, err := s.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return "", fmt.Errorf("解析Kubeconfig失败: %v", err)
		}

		// 如果 kubeconfig 中有 API Server 地址，使用它（优先使用用户提供的）
		if cluster.APIServer == "" && authInfo.APIServer != "" {
			cluster.APIServer = authInfo.APIServer
			// 更新请求 URL
			versionURL = cluster.APIServer + "/version"
			httpReq, err = http.NewRequest("GET", versionURL, nil)
			if err != nil {
				return "", fmt.Errorf("创建请求失败: %v", err)
			}
		}

		// 设置认证头或证书
		if authInfo.Token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+authInfo.Token)
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			// 使用客户端证书认证
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return "", fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
		} else {
			return "", fmt.Errorf("Kubeconfig中未找到有效的认证信息（token或client-certificate+client-key）")
		}
	} else {
		return "", fmt.Errorf("缺少认证信息，无法连接集群")
	}

	// 创建带超时的 context（5秒超时，更快响应）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将 context 绑定到请求
	httpReq = httpReq.WithContext(ctx)

	// 创建 HTTP 客户端，设置连接超时和总超时
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout: 3 * time.Second, // 连接超时3秒
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
		Timeout: 5 * time.Second, // 总超时5秒
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		// 检查是否是超时错误
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("连接超时：无法在5秒内连接到集群 %s，请检查网络连接和API Server地址是否正确", cluster.APIServer)
		}
		// 检查是否是网络错误
		if netErr, ok := err.(net.Error); ok {
			if netErr.Timeout() {
				return "", fmt.Errorf("连接超时：无法连接到集群 %s，请检查网络连接和API Server地址是否正确", cluster.APIServer)
			}
			if netErr.Temporary() {
				return "", fmt.Errorf("临时网络错误：%v，请稍后重试", err)
			}
		}
		return "", fmt.Errorf("连接失败：%v，请检查API Server地址、网络连接和认证信息", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API Server返回错误: %d, %s", resp.StatusCode, string(body))
	}

	// 解析版本响应
	var versionInfo struct {
		GitVersion string `json:"gitVersion"`
		Major      string `json:"major"`
		Minor      string `json:"minor"`
		Patch      string `json:"patch"`
		BuildDate  string `json:"buildDate"`
		Platform   string `json:"platform"`
	}

	if err := json.Unmarshal(body, &versionInfo); err != nil {
		return "", fmt.Errorf("解析版本信息失败: %v, 响应内容: %s", err, string(body))
	}

	// 优先使用 GitVersion，如果没有则组合 Major.Minor
	version := versionInfo.GitVersion
	if version == "" {
		if versionInfo.Major != "" && versionInfo.Minor != "" {
			version = fmt.Sprintf("v%s.%s", versionInfo.Major, versionInfo.Minor)
			if versionInfo.Patch != "" {
				version = fmt.Sprintf("v%s.%s.%s", versionInfo.Major, versionInfo.Minor, versionInfo.Patch)
			}
		} else {
			return "", fmt.Errorf("无法从响应中提取版本信息，响应内容: %s", string(body))
		}
	}

	// 确保版本号以 v 开头
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return version, nil
}

// KubeconfigAuthInfo kubeconfig 认证信息
type KubeconfigAuthInfo struct {
	APIServer  string // API Server 地址
	Token      string // Bearer token
	ClientCert string // 客户端证书内容
	ClientKey  string // 客户端私钥内容
}

// parseKubeconfigAuth 解析 kubeconfig 并提取认证信息和 API Server 地址
func (s *K8sClusterService) parseKubeconfigAuth(kubeconfigContent string) (*KubeconfigAuthInfo, error) {
	// 解析 YAML
	var config struct {
		Clusters []struct {
			Name    string `yaml:"name"`
			Cluster struct {
				Server string `yaml:"server"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
		Users []struct {
			Name string `yaml:"name"`
			User struct {
				Token                 string `yaml:"token"`
				ClientCertificate     string `yaml:"client-certificate"`
				ClientCertificateData string `yaml:"client-certificate-data"`
				ClientKey             string `yaml:"client-key"`
				ClientKeyData         string `yaml:"client-key-data"`
			} `yaml:"user"`
		} `yaml:"users"`
		Contexts []struct {
			Name    string `yaml:"name"`
			Context struct {
				Cluster string `yaml:"cluster"`
				User    string `yaml:"user"`
			} `yaml:"context"`
		} `yaml:"contexts"`
		CurrentContext string `yaml:"current-context"`
	}

	if err := yaml.Unmarshal([]byte(kubeconfigContent), &config); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	// 确定当前使用的上下文
	currentContextName := config.CurrentContext
	var currentClusterName string
	var currentUser string

	if currentContextName != "" {
		for _, ctx := range config.Contexts {
			if ctx.Name == currentContextName {
				currentClusterName = ctx.Context.Cluster
				currentUser = ctx.Context.User
				break
			}
		}
	}

	// 如果没找到当前上下文，使用第一个
	if currentClusterName == "" && len(config.Contexts) > 0 {
		currentClusterName = config.Contexts[0].Context.Cluster
		currentUser = config.Contexts[0].Context.User
	}
	if currentUser == "" && len(config.Users) > 0 {
		currentUser = config.Users[0].Name
	}

	authInfo := &KubeconfigAuthInfo{}

	// 提取 API Server 地址
	if currentClusterName != "" {
		for _, cluster := range config.Clusters {
			if cluster.Name == currentClusterName {
				authInfo.APIServer = cluster.Cluster.Server
				break
			}
		}
	}
	// 如果没找到，使用第一个集群
	if authInfo.APIServer == "" && len(config.Clusters) > 0 {
		authInfo.APIServer = config.Clusters[0].Cluster.Server
	}

	// 查找对应用户的认证信息
	for _, user := range config.Users {
		if user.Name == currentUser || currentUser == "" {
			// 提取 token
			if user.User.Token != "" {
				token := user.User.Token
				// 尝试 base64 解码（如果失败则使用原值）
				if decoded, err := base64.StdEncoding.DecodeString(token); err == nil {
					decodedStr := strings.TrimSpace(string(decoded))
					if decodedStr != "" {
						token = decodedStr
					}
				}
				authInfo.Token = token
			}

			// 提取客户端证书
			var certData []byte
			if user.User.ClientCertificateData != "" {
				// 从 base64 数据中提取
				decoded, err := base64.StdEncoding.DecodeString(user.User.ClientCertificateData)
				if err != nil {
					return nil, fmt.Errorf("解码client-certificate-data失败: %v", err)
				}
				certData = decoded
			} else if user.User.ClientCertificate != "" {
				// 从文件路径读取
				certPath := user.User.ClientCertificate
				// 如果是相对路径，可能需要相对于 kubeconfig 文件的位置
				// 这里简化处理，假设是绝对路径或相对于当前目录
				data, err := os.ReadFile(certPath)
				if err != nil {
					return nil, fmt.Errorf("读取client-certificate文件失败: %v", err)
				}
				certData = data
			}

			// 提取客户端私钥
			var keyData []byte
			if user.User.ClientKeyData != "" {
				// 从 base64 数据中提取
				decoded, err := base64.StdEncoding.DecodeString(user.User.ClientKeyData)
				if err != nil {
					return nil, fmt.Errorf("解码client-key-data失败: %v", err)
				}
				keyData = decoded
			} else if user.User.ClientKey != "" {
				// 从文件路径读取
				keyPath := user.User.ClientKey
				data, err := os.ReadFile(keyPath)
				if err != nil {
					return nil, fmt.Errorf("读取client-key文件失败: %v", err)
				}
				keyData = data
			}

			// 如果找到了证书和私钥，设置它们
			if len(certData) > 0 && len(keyData) > 0 {
				authInfo.ClientCert = string(certData)
				authInfo.ClientKey = string(keyData)
			}

			// 如果至少有一种认证方式，返回
			if authInfo.Token != "" || (authInfo.ClientCert != "" && authInfo.ClientKey != "") {
				return authInfo, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到有效的认证信息（token或client-certificate+client-key）")
}

// CreateCluster 创建集群
func (s *K8sClusterService) CreateCluster(cluster *model.K8sCluster) error {
	// 检查集群名称是否已存在
	existingCluster, err := s.clusterRepo.FindByName(cluster.Name)
	if err != nil {
		return fmt.Errorf("检查集群名称失败: %v", err)
	}
	if existingCluster != nil {
		return fmt.Errorf("集群名称 '%s' 已存在，请使用其他名称", cluster.Name)
	}

	// 如果使用 kubeconfig 且未提供 API Server，从 kubeconfig 中提取
	if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" && cluster.APIServer == "" {
		authInfo, err := s.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return fmt.Errorf("解析Kubeconfig失败: %v", err)
		}
		if authInfo.APIServer == "" {
			return fmt.Errorf("Kubeconfig中未找到API Server地址，请手动填写API Server")
		}
		cluster.APIServer = authInfo.APIServer
	}

	// 验证 API Server 是否已设置
	if cluster.APIServer == "" {
		return fmt.Errorf("API Server地址不能为空")
	}

	// 检查 API Server 是否已存在
	existingClusterByAPI, err := s.clusterRepo.FindByAPIServer(cluster.APIServer)
	if err != nil {
		return fmt.Errorf("检查API Server地址失败: %v", err)
	}
	if existingClusterByAPI != nil {
		return fmt.Errorf("API Server地址 '%s' 已被集群 '%s' 使用，不能重复添加", cluster.APIServer, existingClusterByAPI.Name)
	}

	// 测试集群连接并获取版本
	version, err := s.testClusterConnection(cluster)
	if err != nil {
		return fmt.Errorf("集群连接测试失败: %v", err)
	}

	// 验证版本信息
	if version == "" {
		return fmt.Errorf("未能获取到有效的版本信息")
	}

	// 自动填充版本信息
	cluster.Version = version
	cluster.Status = "active"
	now := time.Now()
	cluster.LastCheckedAt = &now

	if cluster.ID == "" {
		cluster.ID = uuid.New().String()
	}

	// 保存到数据库
	if err := s.clusterRepo.Create(cluster); err != nil {
		// 检查是否是重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE constraint") {
			// 检查是名称还是API Server重复
			if strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "idx_name") {
				return fmt.Errorf("集群名称 '%s' 已存在，请使用其他名称", cluster.Name)
			}
			if strings.Contains(err.Error(), "api_server") || strings.Contains(err.Error(), "idx_api_server") {
				return fmt.Errorf("API Server地址 '%s' 已被使用，不能重复添加", cluster.APIServer)
			}
			return fmt.Errorf("数据重复，请检查集群名称或API Server地址")
		}
		return fmt.Errorf("保存集群信息失败: %v", err)
	}

	return nil
}

// UpdateCluster 更新集群
func (s *K8sClusterService) UpdateCluster(cluster *model.K8sCluster) error {
	return s.clusterRepo.Update(cluster)
}

// DeleteCluster 删除集群
func (s *K8sClusterService) DeleteCluster(id string) error {
	return s.clusterRepo.Delete(id)
}

// GetCluster 获取集群
func (s *K8sClusterService) GetCluster(id string) (*model.K8sCluster, error) {
	return s.clusterRepo.FindByID(id)
}

// GetClusterByName 根据名称获取集群
func (s *K8sClusterService) GetClusterByName(name string) (*model.K8sCluster, error) {
	return s.clusterRepo.FindByName(name)
}

// ListClusters 列出所有集群
func (s *K8sClusterService) ListClusters() ([]model.K8sCluster, error) {
	return s.clusterRepo.FindAll()
}

// ClusterSummary 集群摘要信息
type ClusterSummary struct {
	ClusterID     string  `json:"clusterId"`
	ClusterName   string  `json:"clusterName"`
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	Environment   string  `json:"environment"`
	Region        string  `json:"region"`
	LastCheckedAt *string `json:"lastCheckedAt,omitempty"`
	// K8s状态信息
	K8sStatus *K8sClusterStatus `json:"k8sStatus,omitempty"`
}

// K8sClusterStatus K8s集群状态
type K8sClusterStatus struct {
	// 节点信息
	NodeCount     int `json:"nodeCount"`
	ReadyNodes    int `json:"readyNodes"`
	NotReadyNodes int `json:"notReadyNodes"`

	// Pod统计
	TotalPods   int `json:"totalPods"`
	RunningPods int `json:"runningPods"`
	PendingPods int `json:"pendingPods"`
	FailedPods  int `json:"failedPods"`

	// 工作负载统计
	DeploymentCount  int `json:"deploymentCount"`
	StatefulSetCount int `json:"statefulSetCount"`
	DaemonSetCount   int `json:"daemonSetCount"`

	// 服务统计
	ServiceCount int `json:"serviceCount"`
	IngressCount int `json:"ingressCount"`

	// 命名空间统计
	NamespaceCount int `json:"namespaceCount"`

	// 资源使用情况（如果可用）
	CPUUsage    *ResourceUsage `json:"cpuUsage,omitempty"`
	MemoryUsage *ResourceUsage `json:"memoryUsage,omitempty"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	Total        string  `json:"total"`        // 总量，如 "1000m" 或 "10Gi"
	Used         string  `json:"used"`         // 已使用
	Available    string  `json:"available"`    // 可用
	UsagePercent float64 `json:"usagePercent"` // 使用百分比
}

// GetClusterSummary 获取集群摘要信息
func (s *K8sClusterService) GetClusterSummary(clusterID string) (*ClusterSummary, error) {
	cluster, err := s.clusterRepo.FindByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("获取集群信息失败: %v", err)
	}

	summary := &ClusterSummary{
		ClusterID:   cluster.ID,
		ClusterName: cluster.Name,
		Status:      cluster.Status,
		Version:     cluster.Version,
		Environment: cluster.Environment,
		Region:      cluster.Region,
	}

	if cluster.LastCheckedAt != nil {
		lastCheckedStr := cluster.LastCheckedAt.Format("2006-01-02 15:04:05")
		summary.LastCheckedAt = &lastCheckedStr
	}

	// 获取真实的 K8s 集群状态信息
	k8sStatus, err := s.getK8sClusterStatus(clusterID)
	if err != nil {
		// 如果获取失败，返回空状态（不返回错误，避免影响集群列表显示）
		summary.K8sStatus = &K8sClusterStatus{
			NodeCount:        0,
			ReadyNodes:       0,
			NotReadyNodes:    0,
			TotalPods:        0,
			RunningPods:      0,
			PendingPods:      0,
			FailedPods:       0,
			DeploymentCount:  0,
			StatefulSetCount: 0,
			DaemonSetCount:   0,
			ServiceCount:     0,
			IngressCount:     0,
			NamespaceCount:   0,
		}
	} else {
		summary.K8sStatus = k8sStatus
	}

	return summary, nil
}

// getK8sClusterStatus 获取 K8s 集群状态信息
func (s *K8sClusterService) getK8sClusterStatus(clusterID string) (*K8sClusterStatus, error) {
	status := &K8sClusterStatus{}

	// 1. 获取节点列表（节点是集群级别的，不需要 namespace）
	nodes, err := s.k8sService.GetNodeList(clusterID, "", 0, 0)
	if err == nil {
		status.NodeCount = len(nodes)
		readyNodes := 0
		notReadyNodes := 0
		for _, node := range nodes {
			if node.Status == "Ready" {
				readyNodes++
			} else {
				notReadyNodes++
			}
		}
		status.ReadyNodes = readyNodes
		status.NotReadyNodes = notReadyNodes
	}

	// 2. 获取所有 namespace
	namespaces, err := s.k8sService.GetNamespaceList(clusterID, "")
	if err != nil {
		return status, fmt.Errorf("获取命名空间列表失败: %v", err)
	}
	status.NamespaceCount = len(namespaces)

	// 3. 遍历所有 namespace 获取资源统计
	totalPods := 0
	runningPods := 0
	pendingPods := 0
	failedPods := 0
	totalDeployments := 0
	totalStatefulSets := 0
	totalDaemonSets := 0
	totalServices := 0
	totalIngresses := 0

	for _, ns := range namespaces {
		namespace := ns.Name

		// 获取 Pod 列表
		pods, err := s.k8sService.GetPodList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalPods += len(pods)
			for _, pod := range pods {
				switch pod.Status {
				case "Running":
					runningPods++
				case "Pending":
					pendingPods++
				case "Failed":
					failedPods++
				}
			}
		}

		// 获取 Deployment 列表
		deployments, err := s.k8sService.GetDeploymentList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalDeployments += len(deployments)
		}

		// 获取 StatefulSet 列表
		statefulSets, err := s.k8sService.GetStatefulSetList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalStatefulSets += len(statefulSets)
		}

		// 获取 DaemonSet 列表
		daemonSets, err := s.k8sService.GetDaemonSetList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalDaemonSets += len(daemonSets)
		}

		// 获取 Service 列表
		services, err := s.k8sService.GetServiceList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalServices += len(services)
		}

		// 获取 Ingress 列表
		ingresses, err := s.k8sService.GetIngressList(clusterID, "", 0, 0, namespace)
		if err == nil {
			totalIngresses += len(ingresses)
		}
	}

	status.TotalPods = totalPods
	status.RunningPods = runningPods
	status.PendingPods = pendingPods
	status.FailedPods = failedPods
	status.DeploymentCount = totalDeployments
	status.StatefulSetCount = totalStatefulSets
	status.DaemonSetCount = totalDaemonSets
	status.ServiceCount = totalServices
	status.IngressCount = totalIngresses

	return status, nil
}

// GetAllClustersSummary 获取所有集群的摘要信息
func (s *K8sClusterService) GetAllClustersSummary() ([]*ClusterSummary, error) {
	clusters, err := s.clusterRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("获取集群列表失败: %v", err)
	}

	summaries := make([]*ClusterSummary, 0, len(clusters))
	for _, cluster := range clusters {
		summary := &ClusterSummary{
			ClusterID:   cluster.ID,
			ClusterName: cluster.Name,
			Status:      cluster.Status,
			Version:     cluster.Version,
			Environment: cluster.Environment,
			Region:      cluster.Region,
		}

		if cluster.LastCheckedAt != nil {
			lastCheckedStr := cluster.LastCheckedAt.Format("2006-01-02 15:04:05")
			summary.LastCheckedAt = &lastCheckedStr
		}

		// TODO: 实际连接K8s集群获取状态信息
		summary.K8sStatus = &K8sClusterStatus{
			NodeCount:        0,
			ReadyNodes:       0,
			NotReadyNodes:    0,
			TotalPods:        0,
			RunningPods:      0,
			PendingPods:      0,
			FailedPods:       0,
			DeploymentCount:  0,
			StatefulSetCount: 0,
			DaemonSetCount:   0,
			ServiceCount:     0,
			IngressCount:     0,
			NamespaceCount:   0,
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}
