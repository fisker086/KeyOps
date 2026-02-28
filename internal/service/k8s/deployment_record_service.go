package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// DeploymentService 部署记录服务
type DeploymentService struct {
	deploymentRepo *repository.DeploymentRepository
	kubedogService *KubeDogService
	k8sService     *K8sService
	clusterRepo    *repository.K8sClusterRepository
}

// CreateDeploymentRequest 创建部署请求
type CreateDeploymentRequest struct {
	ProjectName   string `json:"project_name"`
	ProjectID     string `json:"project_id"`
	EnvID         string `json:"env_id"`
	EnvName       string `json:"env_name"`
	ClusterID     string `json:"cluster_id"`
	ClusterName   string `json:"cluster_name"`
	Namespace     string `json:"namespace"`
	DeployType    string `json:"deploy_type"` // jenkins, k8s
	Version       string `json:"version"`
	ArtifactURL   string `json:"artifact_url"`
	JenkinsJob    string `json:"jenkins_job"`
	K8sYAML       string `json:"k8s_yaml"`
	K8sKind       string `json:"k8s_kind"`
	VerifyEnabled bool   `json:"verify_enabled"`
	VerifyTimeout int    `json:"verify_timeout"`
	Description   string `json:"description"`
	CreatedBy     string `json:"created_by"`
	CreatedByName string `json:"created_by_name"`
}

// NewDeploymentService 创建部署记录服务
func NewDeploymentService(
	deploymentRepo *repository.DeploymentRepository,
	kubedogService *KubeDogService,
	k8sService *K8sService,
	clusterRepo *repository.K8sClusterRepository,
	cfg interface{}, // config.Config - kept for compatibility but not used directly
) *DeploymentService {
	return &DeploymentService{
		deploymentRepo: deploymentRepo,
		kubedogService: kubedogService,
		k8sService:     k8sService,
		clusterRepo:    clusterRepo,
	}
}

// CreateDeployment 创建部署记录
func (s *DeploymentService) CreateDeployment(req *CreateDeploymentRequest) (*model.Deployment, error) {
	// 生成部署ID
	deploymentID := uuid.New().String()

	// 设置默认值
	if req.VerifyTimeout == 0 {
		req.VerifyTimeout = 300 // 默认300秒
	}

	// deploy_config 为 JSON 类型，空字符串会导致 MySQL Error 3140，用 "{}" 表示无扩展配置
	deployConfig := "{}"
	deployment := &model.Deployment{
		ID:             deploymentID,
		ProjectName:    req.ProjectName,
		ProjectID:      req.ProjectID,
		EnvID:          req.EnvID,
		EnvName:        req.EnvName,
		ClusterID:      req.ClusterID,
		ClusterName:    req.ClusterName,
		Namespace:      req.Namespace,
		DeployType:     req.DeployType,
		DeployConfig:   deployConfig,
		Version:        req.Version,
		ArtifactURL:    req.ArtifactURL,
		JenkinsJob:     req.JenkinsJob,
		K8sYAML:        req.K8sYAML,
		K8sKind:        req.K8sKind,
		VerifyEnabled:  req.VerifyEnabled,
		VerifyTimeout:  req.VerifyTimeout,
		Description:    req.Description,
		Status:         model.DeploymentStatusPending,
		CreatedBy:      req.CreatedBy,
		CreatedByName:  req.CreatedByName,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.deploymentRepo.Create(deployment); err != nil {
		return nil, fmt.Errorf("创建部署记录失败: %v", err)
	}

	return deployment, nil
}

// GetDeployment 获取部署记录
func (s *DeploymentService) GetDeployment(id string) (*model.Deployment, error) {
	return s.deploymentRepo.GetByID(id)
}

// ListDeployments 查询部署记录列表
func (s *DeploymentService) ListDeployments(params *repository.DeploymentListParams) ([]*model.Deployment, int64, error) {
	return s.deploymentRepo.List(params)
}

// UpdateDeploymentStatus 更新部署状态
func (s *DeploymentService) UpdateDeploymentStatus(id string, status string, duration *int, logPath string) error {
	return s.deploymentRepo.UpdateStatus(id, status, duration, logPath)
}

// DeleteDeployment 删除部署记录
func (s *DeploymentService) DeleteDeployment(id string) error {
	return s.deploymentRepo.Delete(id)
}

// ExecuteK8sDeployment 执行 K8s 部署
func (s *DeploymentService) ExecuteK8sDeployment(id string) error {
	// 获取部署记录
	deployment, err := s.deploymentRepo.GetByID(id)
	if err != nil {
		logger.Errorf("获取部署记录失败: %v", err)
		return fmt.Errorf("获取部署记录失败: %v", err)
	}

	// 更新状态为运行中
	if err := s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusRunning, nil, ""); err != nil {
		logger.Errorf("更新部署状态失败: %v", err)
		return fmt.Errorf("更新部署状态失败: %v", err)
	}

	// 获取集群配置
	cluster, err := s.k8sService.GetClusterConfig(deployment.ClusterID, deployment.ClusterName)
	if err != nil {
		logger.Errorf("获取集群配置失败: %v", err)
		s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusFailed, nil, "")
		return fmt.Errorf("获取集群配置失败: %v", err)
	}

	// 确定命名空间
	namespace := deployment.Namespace
	if namespace == "" {
		namespace = cluster.DefaultNamespace
	}
	if namespace == "" {
		namespace = "default"
	}

	// 应用 YAML
	var resourceName string
	var resourceKind string
	if deployment.K8sYAML != "" {
		resourceName, resourceKind, err = s.applyK8sYAML(cluster, namespace, deployment.K8sYAML)
		if err != nil {
			logger.Errorf("应用 K8s YAML 失败: %v", err)
			s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusFailed, nil, "")
			return fmt.Errorf("应用 K8s YAML 失败: %v", err)
		}
	} else {
		// 如果没有 YAML，使用 K8sKind 和项目名称
		resourceKind = deployment.K8sKind
		if resourceKind == "" {
			resourceKind = "Deployment"
		}
		resourceName = deployment.ProjectName
	}

	// 如果启用了验证，使用 kubedog 监听部署状态
	if deployment.VerifyEnabled {
		timeout := deployment.VerifyTimeout
		if timeout == 0 {
			timeout = 300 // 默认300秒
		}

		logPath := fmt.Sprintf("/tmp/%s.log", id)
		startTime := time.Now()

		// 运行 kubedog
		err = s.kubedogService.RunKubeDog(
			id,
			cluster.APIServer,
			cluster.Token,
			resourceKind,
			resourceName,
			namespace,
			timeout,
			logPath,
		)

		duration := int(time.Since(startTime).Seconds())

		if err != nil {
			logger.Errorf("kubedog 验证失败: %v", err)
			s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusFailed, &duration, logPath)
			return fmt.Errorf("kubedog 验证失败: %v", err)
		}

		// 更新状态为成功
		s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusSuccess, &duration, logPath)
		logger.Infof("部署成功完成: %s", id)
	} else {
		// 没有启用验证，直接标记为成功
		s.deploymentRepo.UpdateStatus(id, model.DeploymentStatusSuccess, nil, "")
		logger.Infof("部署完成（未启用验证）: %s", id)
	}

	return nil
}

// applyK8sYAML 应用 K8s YAML 到集群
func (s *DeploymentService) applyK8sYAML(cluster *model.K8sCluster, namespace string, yamlContent string) (string, string, error) {
	// 解析 YAML 获取资源类型和名称
	var yamlDoc map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &yamlDoc); err != nil {
		return "", "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	kind, ok := yamlDoc["kind"].(string)
	if !ok {
		return "", "", fmt.Errorf("YAML 中缺少 kind 字段")
	}

	metadata, ok := yamlDoc["metadata"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("YAML 中缺少 metadata 字段")
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return "", "", fmt.Errorf("YAML metadata 中缺少 name 字段")
	}

	// 如果 YAML 中没有指定 namespace，使用传入的 namespace
	if _, exists := metadata["namespace"]; !exists {
		metadata["namespace"] = namespace
		// 重新序列化 YAML
		updatedYAML, err := yaml.Marshal(yamlDoc)
		if err == nil {
			yamlContent = string(updatedYAML)
		}
	}

	// 确定 API 路径（POST 请求使用集合端点，不包含资源名称）
	apiPath := s.getAPIPath(kind, namespace)

	// 构建请求 URL
	url := strings.TrimSuffix(cluster.APIServer, "/") + apiPath

	// 将 YAML 转换为 JSON（Kubernetes API 接受 JSON）
	var jsonDoc map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &jsonDoc); err != nil {
		return "", "", fmt.Errorf("YAML 转 JSON 失败: %v", err)
	}

	jsonData, err := json.Marshal(jsonDoc)
	if err != nil {
		return "", "", fmt.Errorf("序列化 JSON 失败: %v", err)
	}

	// 设置认证：支持 token 或 kubeconfig
	var authHeader string
	var tlsConfig *tls.Config = &tls.Config{InsecureSkipVerify: true}
	if cluster.AuthType == "token" && cluster.Token != "" {
		authHeader = "Bearer " + cluster.Token
	} else if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" && s.clusterRepo != nil {
		clusterService := NewK8sClusterService(s.clusterRepo)
		authInfo, err := clusterService.parseKubeconfigAuth(cluster.Kubeconfig)
		if err != nil {
			return "", "", fmt.Errorf("解析 Kubeconfig 失败: %v", err)
		}
		if authInfo.Token != "" {
			authHeader = "Bearer " + authInfo.Token
		} else if authInfo.ClientCert != "" && authInfo.ClientKey != "" {
			cert, err := tls.X509KeyPair([]byte(authInfo.ClientCert), []byte(authInfo.ClientKey))
			if err != nil {
				return "", "", fmt.Errorf("解析客户端证书失败: %v", err)
			}
			tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			}
			authHeader = ""
		} else {
			return "", "", fmt.Errorf("Kubeconfig 中未找到有效的认证信息（token 或 client-certificate+client-key）")
		}
	} else {
		return "", "", fmt.Errorf("集群认证信息不完整：请在该集群的 K8s 管理中添加并保存 Token 或 Kubeconfig")
	}

	// 创建 HTTP 请求（先尝试 POST 创建，如果资源已存在则使用 PUT 更新）
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 如果资源已存在（409 Conflict），尝试使用 PUT 更新
	if resp.StatusCode == http.StatusConflict {
		apiPath := s.getAPIPath(kind, namespace, name)
		putURL := strings.TrimSuffix(cluster.APIServer, "/") + apiPath
		putReq, err := http.NewRequest("PUT", putURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", "", fmt.Errorf("创建更新请求失败: %v", err)
		}
		putReq.Header.Set("Content-Type", "application/json")
		if authHeader != "" {
			putReq.Header.Set("Authorization", authHeader)
		}
		resp2, err := client.Do(putReq)
		if err != nil {
			return "", "", fmt.Errorf("更新请求失败: %v", err)
		}
		defer resp2.Body.Close()
		body, err = io.ReadAll(resp2.Body)
		if err != nil {
			return "", "", fmt.Errorf("读取更新响应失败: %v", err)
		}
		if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
			return "", "", fmt.Errorf("API 请求失败: %s, 响应: %s", resp2.Status, string(body))
		}
		return name, kind, nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("API 请求失败: %s, 响应: %s", resp.Status, string(body))
	}

	return name, kind, nil
}

// getAPIPath 根据资源类型获取 API 路径
// 如果 name 为空，返回集合端点（用于 POST 创建）
// 如果 name 不为空，返回资源端点（用于 PUT 更新）
func (s *DeploymentService) getAPIPath(kind, namespace string, name ...string) string {
	kind = strings.ToLower(kind)
	hasName := len(name) > 0 && name[0] != ""

	// 处理常见的资源类型
	var basePath string
	switch kind {
	case "deployment":
		basePath = fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", namespace)
	case "statefulset":
		basePath = fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets", namespace)
	case "daemonset":
		basePath = fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets", namespace)
	case "service":
		basePath = fmt.Sprintf("/api/v1/namespaces/%s/services", namespace)
	case "configmap":
		basePath = fmt.Sprintf("/api/v1/namespaces/%s/configmaps", namespace)
	case "secret":
		basePath = fmt.Sprintf("/api/v1/namespaces/%s/secrets", namespace)
	default:
		// 默认使用 core API
		basePath = fmt.Sprintf("/api/v1/namespaces/%s/%s", namespace, kind+"s")
	}

	if hasName {
		return basePath + "/" + name[0]
	}
	return basePath
}

