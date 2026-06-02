package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/config"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
)

// DeploymentService 部署记录服务
type DeploymentService struct {
	deploymentRepo repository.DeploymentRepository
	kubedogService *KubeDogService
	k8sService     *K8sService
	clusterRepo    repository.K8sClusterRepository
}

// CreateDeploymentRequest 创建部署请求
type CreateDeploymentRequest struct {
	ProjectName   string                 `json:"project_name"`
	ProjectID     string                 `json:"project_id"`
	EnvID         string                 `json:"env_id"`
	EnvName       string                 `json:"env_name"`
	ClusterID     string                 `json:"cluster_id"`
	ClusterName   string                 `json:"cluster_name"`
	Namespace     string                 `json:"namespace"`
	DeployType    string                 `json:"deploy_type"` // k8s
	DeployConfig  map[string]interface{} `json:"deploy_config"`
	Version       string                 `json:"version"`
	ArtifactURL   string                 `json:"artifact_url"`
	K8sYAML       string                 `json:"k8s_yaml"`
	K8sKind       string                 `json:"k8s_kind"`
	VerifyEnabled bool                   `json:"verify_enabled"`
	VerifyTimeout int                    `json:"verify_timeout"`
	Description   string                 `json:"description"`
	CreatedBy     string                 `json:"created_by"`
	CreatedByName string                 `json:"created_by_name"`
}

// NewDeploymentService 创建部署记录服务
func NewDeploymentService(
	deploymentRepo repository.DeploymentRepository,
	kubedogService *KubeDogService,
	k8sService *K8sService,
	clusterRepo repository.K8sClusterRepository,
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
	if len(req.DeployConfig) > 0 {
		if raw, err := json.Marshal(req.DeployConfig); err == nil {
			deployConfig = string(raw)
		}
	}
	deployment := &model.Deployment{
		ID:           deploymentID,
		ProjectName:  req.ProjectName,
		ProjectID:    req.ProjectID,
		EnvID:        req.EnvID,
		EnvName:      req.EnvName,
		ClusterID:    req.ClusterID,
		ClusterName:  req.ClusterName,
		Namespace:    req.Namespace,
		DeployType:   req.DeployType,
		DeployConfig: deployConfig,
		Version:      req.Version,
		ArtifactURL:  req.ArtifactURL,

		K8sYAML:       req.K8sYAML,
		K8sKind:       req.K8sKind,
		VerifyEnabled: req.VerifyEnabled,
		VerifyTimeout: req.VerifyTimeout,
		Description:   req.Description,
		Status:        model.DeploymentStatusPending,
		CreatedBy:     req.CreatedBy,
		CreatedByName: req.CreatedByName,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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

	if deployment.DeployType == model.DeployTypeHelm {
		return s.executeHelmDeployment(deployment)
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

func (s *DeploymentService) executeHelmDeployment(deployment *model.Deployment) error {
	startTime := time.Now()
	var logBuffer strings.Builder

	// 辅助函数：记录日志并追加到 buffer
	logInfo := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logger.Infof(msg)
		logBuffer.WriteString(fmt.Sprintf("[INFO] %s %s\n", time.Now().Format("15:04:05"), msg))
	}

	logError := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logger.Errorf(msg)
		logBuffer.WriteString(fmt.Sprintf("[ERROR] %s %s\n", time.Now().Format("15:04:05"), msg))
	}

	// 保存日志到数据库的 build_log 字段（前端可以直接读取）
	saveLogToDB := func(status string) {
		logContent := logBuffer.String()
		// 更新 build_log 字段，前端可以直接显示
		s.deploymentRepo.UpdateBuildLog(deployment.ID, logContent)

		// 更新状态
		if status != "" {
			duration := int(time.Since(startTime).Seconds())
			s.deploymentRepo.UpdateStatus(deployment.ID, status, &duration, "")
		}
	}

	logInfo("开始 Helm 部署: deployment_id=%s project=%s", deployment.ID, deployment.ProjectName)

	if err := s.deploymentRepo.UpdateStatus(deployment.ID, model.DeploymentStatusRunning, nil, ""); err != nil {
		return fmt.Errorf("更新部署状态失败: %v", err)
	}

	// 获取集群配置
	logInfo("获取集群配置: cluster_id=%s", deployment.ClusterID)
	cluster, err := s.k8sService.GetClusterConfig(deployment.ClusterID, deployment.ClusterName)
	if err != nil {
		logError("获取集群配置失败: %v", err)
		saveLogToDB(model.DeploymentStatusFailed)
		return fmt.Errorf("获取集群配置失败: %v", err)
	}
	logInfo("集群配置获取成功: cluster=%s", cluster.Name)

	// 解析 Helm 配置
	logInfo("解析 Helm 配置...")
	var cfg model.HelmDeployConfig
	if deployment.DeployConfig != "" && deployment.DeployConfig != "{}" {
		if err := json.Unmarshal([]byte(deployment.DeployConfig), &cfg); err != nil {
			logError("解析 Helm 配置失败: %v", err)
			saveLogToDB(model.DeploymentStatusFailed)
			return fmt.Errorf("解析 Helm 配置失败: %v", err)
		}
	}

	// 自动填充 chart_name：如果未指定，使用配置文件中的默认值
	if cfg.ChartName == "" {
		helmCfg := config.GlobalConfig.Helm
		if helmCfg.DefaultChartName != "" {
			cfg.ChartName = helmCfg.DefaultChartName
			logInfo("未指定 chart_name，使用默认值: %s", cfg.ChartName)
		}
	}

	// 自动填充 chart_version：如果未指定，使用配置文件中的默认值
	if cfg.ChartVersion == "" {
		helmCfg := config.GlobalConfig.Helm
		if helmCfg.DefaultChartVersion != "" {
			cfg.ChartVersion = helmCfg.DefaultChartVersion
			logInfo("未指定 chart_version，使用默认值: %s", cfg.ChartVersion)
		} else {
			logInfo("未指定 chart_version，将使用最新版本")
		}
	}

	// 打印配置详情
	configJSON, _ := json.MarshalIndent(cfg, "", "  ")
	logInfo("Helm 配置:\n%s", string(configJSON))

	// 验证必需字段
	if cfg.ChartName == "" {
		logError("Helm 配置缺少 chart_name，请在 deploy_config 中指定或在配置文件中设置 helm.default_chart_name")
		saveLogToDB(model.DeploymentStatusFailed)
		return fmt.Errorf("Helm 配置缺少 chart_name，请在 deploy_config 中指定或在配置文件中设置 helm.default_chart_name")
	}

	// 验证 Values 中的镜像配置
	if cfg.Values != nil {
		if imageMap, ok := cfg.Values["image"].(map[string]interface{}); ok {
			repo, _ := imageMap["repository"].(string)
			tag, _ := imageMap["tag"].(string)
			if repo == "" {
				logError("镜像仓库地址为空 (image.repository)")
				logError("请在数据库中配置参数: INSERT INTO app_deploy_param_configs (app_id, env, param_name, param_value) VALUES ('app-id', 'prod', 'image.repository', 'your-image-repo')")
				saveLogToDB(model.DeploymentStatusFailed)
				return fmt.Errorf("镜像仓库地址为空，请在数据库中配置 image.repository 参数")
			}
			if tag == "" {
				logError("镜像标签为空 (image.tag)")
				saveLogToDB(model.DeploymentStatusFailed)
				return fmt.Errorf("镜像标签为空，请在数据库中配置 image.tag 参数")
			}
			logInfo("镜像配置: %s:%s", repo, tag)
		}
	}

	releaseName := cfg.ReleaseName
	if releaseName == "" {
		releaseName = deployment.ProjectName
	}
	namespace := deployment.Namespace
	if namespace == "" {
		namespace = cfg.Namespace
	}
	if namespace == "" {
		namespace = cluster.DefaultNamespace
	}
	if namespace == "" {
		namespace = "default"
	}

	logInfo("Release 名称: %s", releaseName)
	logInfo("命名空间: %s", namespace)

	// 创建临时 kubeconfig
	logInfo("创建临时 kubeconfig...")
	kubeconfigPath, err := s.writeTempKubeconfig(cluster)
	if err != nil {
		logError("创建 kubeconfig 失败: %v", err)
		saveLogToDB(model.DeploymentStatusFailed)
		return err
	}
	defer os.Remove(kubeconfigPath)
	logInfo("kubeconfig 创建成功")

	// 初始化 Helm 客户端
	logInfo("初始化 Helm 客户端...")
	settings := cli.New()
	settings.KubeConfig = kubeconfigPath
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret"); err != nil {
		logError("初始化 Helm 客户端失败: %v", err)
		saveLogToDB(model.DeploymentStatusFailed)
		return fmt.Errorf("初始化 Helm 客户端失败: %v", err)
	}
	logInfo("Helm 客户端初始化成功")

	// 定位 Chart
	var chartPath string
	if strings.Contains(cfg.ChartName, "/") || strings.HasPrefix(cfg.ChartName, ".") {
		chartPath = cfg.ChartName
		logInfo("使用本地 Helm Chart: %s", chartPath)
	} else if cfg.Repository != "" {
		logInfo("从远程仓库拉取 Chart: repo=%s chart=%s version=%s", cfg.Repository, cfg.ChartName, cfg.ChartVersion)
		chartPathOpts := action.ChartPathOptions{
			RepoURL: cfg.Repository,
			Version: cfg.ChartVersion,
		}
		var err error
		chartPath, err = chartPathOpts.LocateChart(cfg.ChartName, settings)
		if err != nil {
			logError("定位 Helm Chart 失败: %v", err)
			saveLogToDB(model.DeploymentStatusFailed)
			return fmt.Errorf("定位 Helm Chart 失败: %v", err)
		}
		logInfo("Chart 拉取成功: %s", chartPath)
	} else {
		helmCfg := config.GlobalConfig.Helm
		if helmCfg.Source == "local" {
			chartPath = helmCfg.LocalPath
			logInfo("使用配置文件中的本地 Helm Chart: %s", chartPath)
		} else if helmCfg.Source == "remote" && helmCfg.RemoteRepo.URL != "" {
			logInfo("从配置文件远程仓库拉取 Chart: repo=%s chart=%s", helmCfg.RemoteRepo.URL, cfg.ChartName)
			chartPathOpts := action.ChartPathOptions{
				RepoURL: helmCfg.RemoteRepo.URL,
				Version: cfg.ChartVersion,
			}
			var err error
			chartPath, err = chartPathOpts.LocateChart(cfg.ChartName, settings)
			if err != nil {
				logError("从配置文件远程仓库定位 Helm Chart 失败: %v", err)
				saveLogToDB(model.DeploymentStatusFailed)
				return fmt.Errorf("从配置文件远程仓库定位 Helm Chart 失败: %v", err)
			}
			logInfo("Chart 拉取成功: %s", chartPath)
		} else {
			logError("Helm Chart 配置不完整：未指定 chart 路径或远程仓库")
			saveLogToDB(model.DeploymentStatusFailed)
			return fmt.Errorf("Helm Chart 配置不完整：未指定 chart 路径或远程仓库")
		}
	}

	// 加载 Chart
	logInfo("加载 Helm Chart: %s", chartPath)
	ch, err := loader.Load(chartPath)
	if err != nil {
		logError("加载 Helm Chart 失败: %v", err)
		saveLogToDB(model.DeploymentStatusFailed)
		return fmt.Errorf("加载 Helm Chart 失败: %v", err)
	}
	logInfo("Chart 加载成功")

	values := map[string]interface{}{}
	for k, v := range cfg.Values {
		values[k] = v
	}

	// 打印 Helm values
	valuesYAML, _ := yaml.Marshal(values)
	logInfo("Helm Values:\n%s", string(valuesYAML))

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	logInfo("部署超时时间: %v", timeout)

	// 检查 Release 是否存在
	logInfo("检查 Release 是否存在: %s", releaseName)
	_, getErr := action.NewGet(actionConfig).Run(releaseName)
	if getErr == nil {
		// Release 存在，执行升级
		logInfo("Release 已存在，执行升级...")
		upgrade := action.NewUpgrade(actionConfig)
		upgrade.Namespace = namespace
		upgrade.Timeout = timeout
		// Helm v4 要求必须显式设置 WaitStrategy，否则报 "wait strategy not set"
		if cfg.Wait {
			upgrade.WaitStrategy = kube.StatusWatcherStrategy
			upgrade.WaitForJobs = true
		} else {
			upgrade.WaitStrategy = kube.HookOnlyStrategy
		}
		if _, err := upgrade.Run(releaseName, ch, values); err != nil {
			logError("Helm 升级失败: %v", err)
			saveLogToDB(model.DeploymentStatusFailed)
			return fmt.Errorf("Helm 升级失败: %v", err)
		}
		logInfo("Helm 升级成功")
	} else {
		// Release 不存在，执行安装
		logInfo("Release 不存在，执行安装...")
		install := action.NewInstall(actionConfig)
		install.ReleaseName = releaseName
		install.Namespace = namespace
		install.Timeout = timeout
		// Helm v4 要求必须显式设置 WaitStrategy，否则报 "wait strategy not set"
		if cfg.Wait {
			install.WaitStrategy = kube.StatusWatcherStrategy
			install.WaitForJobs = true
		} else {
			install.WaitStrategy = kube.HookOnlyStrategy
		}
		if _, err := install.Run(ch, values); err != nil {
			logError("Helm 安装失败: %v", err)
			saveLogToDB(model.DeploymentStatusFailed)
			return fmt.Errorf("Helm 安装失败: %v", err)
		}
		logInfo("Helm 安装成功")
	}

	duration := int(time.Since(startTime).Seconds())
	logInfo("部署完成，耗时: %d 秒", duration)
	saveLogToDB(model.DeploymentStatusSuccess)

	return nil
}

func (s *DeploymentService) writeTempKubeconfig(cluster *model.K8sCluster) (string, error) {
	if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		file, err := os.CreateTemp("", "keyops-helm-kubeconfig-*.yaml")
		if err != nil {
			return "", fmt.Errorf("创建临时 kubeconfig 失败: %v", err)
		}
		if _, err := file.WriteString(cluster.Kubeconfig); err != nil {
			file.Close()
			return "", fmt.Errorf("写入临时 kubeconfig 失败: %v", err)
		}
		file.Close()
		return file.Name(), nil
	}
	if cluster.AuthType == "token" && cluster.Token != "" && cluster.APIServer != "" {
		kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: keyops
  cluster:
    server: %s
    insecure-skip-tls-verify: true
users:
- name: keyops-user
  user:
    token: %s
contexts:
- name: keyops
  context:
    cluster: keyops
    user: keyops-user
current-context: keyops
`, cluster.APIServer, cluster.Token)
		file, err := os.CreateTemp("", "keyops-helm-kubeconfig-*.yaml")
		if err != nil {
			return "", fmt.Errorf("创建临时 kubeconfig 失败: %v", err)
		}
		if _, err := file.WriteString(kubeconfig); err != nil {
			file.Close()
			return "", fmt.Errorf("写入临时 kubeconfig 失败: %v", err)
		}
		file.Close()
		return file.Name(), nil
	}
	return "", fmt.Errorf("集群认证信息不完整：请在该集群的 K8s 管理中配置 token 或 kubeconfig")
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
		authInfo, err := clusterService.getClusterAuth(cluster)
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
		Timeout:   30 * time.Second,
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
