package model

// DeployType 部署类型常量
const (
	DeployTypeJenkins = "jenkins"
	DeployTypeK8s     = "k8s"
	DeployTypeGitOps  = "gitops"
	DeployTypeArgoCD  = "argocd"
	DeployTypeHelm    = "helm"
)

// DeployTypeConfig 部署类型配置接口（用于不同发布方式的特定配置）
type DeployTypeConfig interface {
	GetDeployType() string
	Validate() error
}

// JenkinsDeployConfig Jenkins发布配置
type JenkinsDeployConfig struct {
	DeployType      string `json:"deploy_type"`       // jenkins
	JenkinsServerID int    `json:"jenkins_server_id"` // Jenkins服务器ID
	JenkinsJob      string `json:"jenkins_job"`       // Jenkins Job名称
	BuildParameters map[string]string `json:"build_parameters,omitempty"` // 构建参数
}

func (c *JenkinsDeployConfig) GetDeployType() string {
	return DeployTypeJenkins
}

func (c *JenkinsDeployConfig) Validate() error {
	if c.JenkinsServerID == 0 {
		return ErrInvalidConfig("jenkins_server_id is required")
	}
	if c.JenkinsJob == "" {
		return ErrInvalidConfig("jenkins_job is required")
	}
	return nil
}

// GitOpsDeployConfig GitOps发布配置
type GitOpsDeployConfig struct {
	DeployType      string `json:"deploy_type"`       // gitops
	GitRepository   string `json:"git_repository"`   // Git仓库地址
	GitBranch       string `json:"git_branch"`        // Git分支
	GitPath         string `json:"git_path"`          // Git路径（如：/manifests/app1）
	CommitMessage   string `json:"commit_message,omitempty"` // 提交信息
	AutoSync        bool   `json:"auto_sync"`          // 是否自动同步
}

func (c *GitOpsDeployConfig) GetDeployType() string {
	return DeployTypeGitOps
}

func (c *GitOpsDeployConfig) Validate() error {
	if c.GitRepository == "" {
		return ErrInvalidConfig("git_repository is required")
	}
	if c.GitBranch == "" {
		return ErrInvalidConfig("git_branch is required")
	}
	return nil
}

// ArgoCDDeployConfig ArgoCD发布配置
type ArgoCDDeployConfig struct {
	DeployType        string            `json:"deploy_type"`         // argocd
	ArgoCDServer      string            `json:"argocd_server"`       // ArgoCD服务器地址
	ApplicationName   string            `json:"application_name"`     // ArgoCD应用名称
	TargetRevision    string            `json:"target_revision"`     // 目标版本/分支
	SyncPolicy        *ArgoCDSyncPolicy `json:"sync_policy,omitempty"` // 同步策略
	SyncOptions       []string          `json:"sync_options,omitempty"` // 同步选项
}

type ArgoCDSyncPolicy struct {
	Automated *ArgoCDAutomatedSyncPolicy `json:"automated,omitempty"`
	SyncOptions []string `json:"sync_options,omitempty"`
}

type ArgoCDAutomatedSyncPolicy struct {
	Prune    bool `json:"prune"`
	SelfHeal bool `json:"self_heal"`
}

func (c *ArgoCDDeployConfig) GetDeployType() string {
	return DeployTypeArgoCD
}

func (c *ArgoCDDeployConfig) Validate() error {
	if c.ArgoCDServer == "" {
		return ErrInvalidConfig("argocd_server is required")
	}
	if c.ApplicationName == "" {
		return ErrInvalidConfig("application_name is required")
	}
	return nil
}

// HelmDeployConfig Helm发布配置
type HelmDeployConfig struct {
	DeployType    string            `json:"deploy_type"`     // helm
	ChartName     string            `json:"chart_name"`      // Chart名称
	ChartVersion  string            `json:"chart_version"`    // Chart版本
	Repository    string            `json:"repository"`     // Helm仓库地址
	ReleaseName   string            `json:"release_name"`   // Release名称
	Namespace     string            `json:"namespace"`      // 命名空间
	Values        map[string]interface{} `json:"values,omitempty"` // Values配置
	ValuesFile    string            `json:"values_file,omitempty"` // Values文件路径
	Wait          bool              `json:"wait"`            // 是否等待部署完成
	Timeout       int               `json:"timeout,omitempty"` // 超时时间（秒）
}

func (c *HelmDeployConfig) GetDeployType() string {
	return DeployTypeHelm
}

func (c *HelmDeployConfig) Validate() error {
	if c.ChartName == "" {
		return ErrInvalidConfig("chart_name is required")
	}
	if c.ReleaseName == "" {
		return ErrInvalidConfig("release_name is required")
	}
	return nil
}

// K8sDeployConfig K8s原生发布配置（保留向后兼容）
type K8sDeployConfig struct {
	DeployType string `json:"deploy_type"` // k8s
	K8sYAML    string `json:"k8s_yaml"`    // K8s YAML内容
	K8sKind    string `json:"k8s_kind"`    // K8s资源类型
}

func (c *K8sDeployConfig) GetDeployType() string {
	return DeployTypeK8s
}

func (c *K8sDeployConfig) Validate() error {
	if c.K8sYAML == "" {
		return ErrInvalidConfig("k8s_yaml is required")
	}
	return nil
}

// ErrInvalidConfig 配置错误
type ErrInvalidConfig string

func (e ErrInvalidConfig) Error() string {
	return string(e)
}

