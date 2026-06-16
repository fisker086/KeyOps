package release

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	svc "github.com/fisker086/keyops/internal/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Prod 环境常量，走工单审批
const EnvironmentProd = "prod"

// 支持的环境（与 application_deploy_bindings.environment 一致）
var SupportedEnvironments = []string{"dev", "test", "qa", "staging", "prod"}

// DeployProdStarter 生产发布编排启动器（如 Temporal），可选注入
type DeployProdStarter interface {
	StartDeployProd(ctx context.Context, runID, applicationID, environment string) error
}

type Service struct {
	repo              repository.ReleaseRunRepository
	db                *gorm.DB
	appRepo           repository.ApplicationRepository
	settingRepo       repository.SettingRepository
	deployProdStarter DeployProdStarter
	deploymentSvc     *svc.DeploymentService
}

func NewService(repo repository.ReleaseRunRepository) *Service {
	return &Service{repo: repo}
}

// SetDB 注入数据库
func (s *Service) SetDB(db *gorm.DB) {
	s.db = db
}

// SetAppRepo 注入应用仓库
func (s *Service) SetAppRepo(appRepo repository.ApplicationRepository) {
	s.appRepo = appRepo
}

// SetSettingRepository 注入设置仓库（用于读取 release_approval 创建第三方审批）
func (s *Service) SetSettingRepository(repo repository.SettingRepository) {
	s.settingRepo = repo
}

// SetDeployProdStarter 注入生产发布编排器（如 Temporal）；为 nil 时审批通过后直接执行
func (s *Service) SetDeployProdStarter(starter DeployProdStarter) {
	s.deployProdStarter = starter
}

// SetDeploymentService 注入部署服务，用于审批通过后执行 Helm 发布
func (s *Service) SetDeploymentService(deploymentSvc *svc.DeploymentService) {
	s.deploymentSvc = deploymentSvc
}

// CreateFromWebhook 根据 Webhook 负载创建一条发布记录（仅落库，不执行流水线）
func (s *Service) CreateFromWebhook(repoURL, branch, commitSHA, commitMessage, ref, triggeredBy string) (*model.ReleaseRun, error) {
	return s.CreateFromWebhookWithApplication("", repoURL, branch, commitSHA, commitMessage, ref, triggeredBy)
}

// CreateFromWebhookWithApplication 根据 Webhook 负载创建发布记录，并关联到指定应用（用于带 token 的动态推送 URL）
func (s *Service) CreateFromWebhookWithApplication(applicationID, repoURL, branch, commitSHA, commitMessage, ref, triggeredBy string) (*model.ReleaseRun, error) {
	run := &model.ReleaseRun{
		ID:            uuid.New().String(),
		ApplicationID: applicationID,
		RepoURL:       repoURL,
		Branch:        branch,
		CommitSHA:     commitSHA,
		CommitMessage: commitMessage,
		Ref:           ref,
		Source:        model.ReleaseRunSourceWebhook,
		Status:        model.ReleaseRunStatusPending,
		TriggeredBy:   triggeredBy,
	}
	if err := s.repo.Create(run); err != nil {
		return nil, err
	}
	return run, nil
}

// CreateManual 手动触发创建一条发布记录
func (s *Service) CreateManual(repoURL, branch, commitSHA, commitMessage, applicationID, userID string) (*model.ReleaseRun, error) {
	run := &model.ReleaseRun{
		ID:            uuid.New().String(),
		ApplicationID: applicationID,
		RepoURL:       repoURL,
		Branch:        branch,
		CommitSHA:     commitSHA,
		CommitMessage: commitMessage,
		Ref:           "refs/heads/" + branch,
		Source:        model.ReleaseRunSourceManual,
		Status:        model.ReleaseRunStatusPending,
		CreatedBy:     userID,
	}
	if err := s.repo.Create(run); err != nil {
		return nil, err
	}
	return run, nil
}

// CreateRun 直接创建一条部署记录（供 BuildMaster 等模块调用）
func (s *Service) CreateRun(run *model.ReleaseRun) error {
	return s.repo.Create(run)
}

// List 分页列表
func (s *Service) List(repoURL, branch, status string, page, pageSize int) ([]model.ReleaseRun, int64, error) {
	return s.repo.List(repoURL, branch, status, page, pageSize)
}

// GetByID 根据 ID 获取
func (s *Service) GetByID(id string) (*model.ReleaseRun, error) {
	return s.repo.GetByID(id)
}

// UpdateRunStatus 更新 run 状态（人工标记成功/失败后调用，用于回滚源）
func (s *Service) UpdateRunStatus(id string, status string, completedAt *time.Time) error {
	return s.repo.UpdateStatus(id, status, nil, completedAt)
}

// DeployConfigForApproval prod 审批单中 DeployConfig 的 JSON 结构
type DeployConfigForApproval struct {
	ReleaseRunID  string `json:"release_run_id"`
	Environment   string `json:"environment"`
	ApplicationID string `json:"application_id"`
}

// ExecuteRun 执行一条发布记录：按环境直接执行，或提交 prod 工单
// environment: dev/test/qa/staging 直接触发；prod 创建发布审批单，审批通过后自动执行
func (s *Service) ExecuteRun(id string, environment string, applicantID string, applicantName string) (prodApprovalCreated bool, approvalID string, err error) {
	if environment == "" {
		environment = "dev"
	}
	run, err := s.repo.GetByID(id)
	if err != nil {
		return false, "", err
	}
	if run.Status != model.ReleaseRunStatusPending {
		return false, "", fmt.Errorf("release run status is %s, only pending can be executed", run.Status)
	}

	var applicationID string
	if run.ApplicationID != "" {
		applicationID = run.ApplicationID
	} else {
		if s.appRepo == nil {
			return false, "", fmt.Errorf("release execute not configured: missing app repo")
		}
		app, err := s.appRepo.FindByGitURL(run.RepoURL)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, "", fmt.Errorf("no application matched repo_url %q, bind application or set application_id", run.RepoURL)
			}
			return false, "", err
		}
		applicationID = app.ID
	}

	// prod：创建发布审批单，不直接执行
	if environment == EnvironmentProd {
		approvalID, err = s.createProdApproval(run.ID, applicationID, run.RepoURL, run.Branch, run.CommitSHA, applicantID, applicantName)
		if err != nil {
			return false, "", err
		}
		return true, approvalID, nil
	}

	// 非 prod：直接标记为运行中
	now := time.Now()
	return false, "", s.repo.UpdateStatusAndDeployedEnv(run.ID, model.ReleaseRunStatusRunning, environment, &now, nil)
}

// createProdApproval 创建生产发布审批单，DeployConfig 存 release_run_id 等，审批通过后由回调执行
func (s *Service) createProdApproval(releaseRunID, applicationID, repoURL, branch, commitSHA, applicantID, applicantName string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("db not set for creating approval")
	}
	cfg := DeployConfigForApproval{
		ReleaseRunID:  releaseRunID,
		Environment:   EnvironmentProd,
		ApplicationID: applicationID,
	}
	cfgJSON, _ := json.Marshal(cfg)

	// 审批人：取 role:admin 成员，若无则留空（仅申请人可见，待管理员在列表中指派）
	var approverIDs []string
	_ = s.db.Table("role_members").Where("role_id = ?", "role:admin").Pluck("user_id", &approverIDs)

	now := time.Now()
	title := fmt.Sprintf("生产发布 %s @ %s (%s)", repoURL, branch, commitSHA)
	if len(commitSHA) > 7 {
		title = fmt.Sprintf("生产发布 %s @ %s (%s)", repoURL, branch, commitSHA[:7])
	}
	a := &model.Approval{
		ID:            uuid.New().String(),
		Title:         title,
		Description:   fmt.Sprintf("发布代码记录 %s，环境 prod", releaseRunID),
		Type:          model.ApprovalTypeDeployment,
		Status:        model.ApprovalStatusPending,
		Platform:      model.ApprovalPlatformInternal,
		ApplicantID:   applicantID,
		ApplicantName: applicantName,
		DeployConfig:  string(cfgJSON),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if len(approverIDs) > 0 {
		a.ApproverIDs = model.StringArray(approverIDs)
	}
	if err := s.db.Create(a).Error; err != nil {
		return "", err
	}
	// 若系统设置中配置了发布审批（飞书/钉钉/企微），则创建第三方审批实例并更新本记录
	_ = s.tryCreateReleaseThirdPartyApproval(a)
	return a.ID, nil
}

// tryCreateReleaseThirdPartyApproval 读取 release_approval 设置，若已配置则创建飞书/钉钉/企微审批实例并更新 approval
func (s *Service) tryCreateReleaseThirdPartyApproval(a *model.Approval) error {
	if s.settingRepo == nil || s.db == nil {
		return nil
	}
	settings, err := s.settingRepo.GetByCategory(model.CategoryReleaseApproval)
	if err != nil || len(settings) == 0 {
		return nil
	}
	cfgMap := make(map[string]string)
	prefix := model.CategoryReleaseApproval + "."
	for _, st := range settings {
		k := st.Key
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			k = k[len(prefix):]
		}
		cfgMap[k] = st.Value
	}
	platform := cfgMap["platform"]
	if platform == "" {
		platform = "feishu"
	}
	var appID, appSecret, approvalCode string
	switch platform {
	case "feishu":
		appID = cfgMap["feishu_app_id"]
		appSecret = cfgMap["feishu_app_secret"]
		approvalCode = cfgMap["feishu_approval_code"]
	case "dingtalk":
		appID = cfgMap["dingtalk_app_id"]
		appSecret = cfgMap["dingtalk_app_secret"]
		approvalCode = cfgMap["dingtalk_process_code"]
	case "wechat":
		appID = cfgMap["wechat_app_id"]
		appSecret = cfgMap["wechat_app_secret"]
		approvalCode = cfgMap["wechat_template_id"]
	default:
		return nil
	}
	if appID == "" || appSecret == "" || approvalCode == "" {
		return nil
	}
	config := &model.ApprovalConfig{
		Type:         platform,
		AppID:        appID,
		AppSecret:    appSecret,
		ApprovalCode: approvalCode,
		ProcessCode:  approvalCode,
		TemplateID:   approvalCode,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 目前仅实现飞书/Lark/钉钉：按字段名称构建表单并创建实例
	if platform == "feishu" {
		provider := approval.NewFeishuProvider(config, s.db, model.ApprovalPlatformFeishu)
		formData, err := provider.BuildReleaseFormData(ctx, approvalCode, a, "")
		if err != nil {
			return err
		}
		externalID, err := provider.CreateApprovalWithFormData(ctx, approvalCode, formData, a)
		if err != nil {
			return err
		}
		a.Platform = model.ApprovalPlatformFeishu
		a.ExternalID = externalID
	} else if platform == "lark" {
		provider := approval.NewFeishuProvider(config, s.db, model.ApprovalPlatformLark)
		formData, err := provider.BuildReleaseFormData(ctx, approvalCode, a, "")
		if err != nil {
			return err
		}
		externalID, err := provider.CreateApprovalWithFormData(ctx, approvalCode, formData, a)
		if err != nil {
			return err
		}
		a.Platform = model.ApprovalPlatformLark
		a.ExternalID = externalID
		a.ExternalURL = fmt.Sprintf("https://www.larksuite.com/approval/instance/%s", externalID)
		return s.db.Save(a).Error
	} else if platform == "dingtalk" {
		provider := approval.NewDingTalkProvider(config, s.db)
		externalID, err := provider.CreateApprovalWithFormData(ctx, approvalCode, a.Description, a)
		if err != nil {
			return err
		}
		a.Platform = model.ApprovalPlatformDingTalk
		a.ExternalID = externalID
		a.ExternalURL = fmt.Sprintf("https://oa.dingtalk.com/approval/detail?processInstanceId=%s", externalID)
		return s.db.Save(a).Error
	}
	return nil
}

// ExecuteApprovedDeployment 审批通过后调用：根据 Approval.DeployConfig 执行实际发布（可选走 Temporal 编排）
func (s *Service) ExecuteApprovedDeployment(approval *model.Approval) error {
	if approval.Type != model.ApprovalTypeDeployment || approval.DeployConfig == "" {
		return nil
	}
	var cfg DeployConfigForApproval
	if err := json.Unmarshal([]byte(approval.DeployConfig), &cfg); err != nil {
		return fmt.Errorf("parse deploy_config: %w", err)
	}
	if cfg.ReleaseRunID == "" || cfg.Environment == "" {
		return fmt.Errorf("deploy_config missing release_run_id or environment")
	}
	if s.deployProdStarter != nil {
		return s.deployProdStarter.StartDeployProd(context.Background(), cfg.ReleaseRunID, cfg.ApplicationID, cfg.Environment)
	}
	return s.ExecuteDeployment(cfg.ReleaseRunID, cfg.ApplicationID, cfg.Environment)
}

// ExecuteDeployment 根据 runID + 应用 + 环境执行部署（供 Temporal Activity 或直接调用）
func (s *Service) ExecuteDeployment(runID, applicationID, environment string) error {
	run, err := s.repo.GetByID(runID)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.repo.UpdateStatusAndDeployedEnv(run.ID, model.ReleaseRunStatusRunning, environment, &now, nil)
}

// GetLastSuccessfulProdRun 查询某应用最近一次 prod 部署成功的 run（用于展示当前线上版本、回滚源）
func (s *Service) GetLastSuccessfulProdRun(applicationID string) (*model.ReleaseRun, error) {
	return s.repo.GetLastSuccessfulProdRun(applicationID)
}

// RollbackProd 提交生产回滚：基于上一版 prod 成功 run 创建新 run 并走工单审批
func (s *Service) RollbackProd(applicationID string, applicantID string, applicantName string) (prodApprovalCreated bool, approvalID string, runID string, err error) {
	last, err := s.repo.GetLastSuccessfulProdRun(applicationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", "", fmt.Errorf("no previous prod deployment to rollback, deploy first")
		}
		return false, "", "", err
	}
	run := &model.ReleaseRun{
		ID:                uuid.New().String(),
		ApplicationID:     applicationID,
		RepoURL:           last.RepoURL,
		Branch:            last.Branch,
		CommitSHA:         last.CommitSHA,
		CommitMessage:     last.CommitMessage + " (rollback)",
		Ref:               last.Ref,
		Source:            model.ReleaseRunSourceRollback,
		Status:            model.ReleaseRunStatusPending,
		CreatedBy:         applicantID,
		RollbackFromRunID: last.ID,
	}
	if err := s.repo.Create(run); err != nil {
		return false, "", "", err
	}
	approvalID, err = s.createProdApproval(run.ID, applicationID, run.RepoURL, run.Branch, run.CommitSHA, applicantID, applicantName)
	if err != nil {
		return false, "", run.ID, err
	}
	return true, approvalID, run.ID, nil
}

// GitHubPushPayload GitHub push event 部分字段
type GitHubPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
	} `json:"head_commit"`
}

// ParseGitHubPush 解析 GitHub push webhook，返回 repo_url, branch, commit_sha, message, ref, author
// 删除分支时 head_commit 为 null，此类事件会返回错误
func ParseGitHubPush(body []byte) (repoURL, branch, commitSHA, commitMessage, ref, author string, err error) {
	var p GitHubPushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", "", "", "", "", "", err
	}
	if p.Ref == "" {
		return "", "", "", "", "", "", fmt.Errorf("invalid github push payload: missing ref")
	}
	if p.HeadCommit.ID == "" {
		return "", "", "", "", "", "", fmt.Errorf("invalid github push payload: head_commit empty (e.g. branch delete)")
	}
	// ref 如 refs/heads/main
	ref = p.Ref
	if len(ref) > 11 && ref[:11] == "refs/heads/" {
		branch = ref[11:]
	} else {
		branch = ref
	}
	repoURL = p.Repository.CloneURL
	if repoURL == "" {
		repoURL = p.Repository.SSHURL
	}
	commitSHA = p.HeadCommit.ID
	commitMessage = p.HeadCommit.Message
	author = p.HeadCommit.Author.Username
	if author == "" {
		author = p.HeadCommit.Author.Name
	}
	return repoURL, branch, commitSHA, commitMessage, ref, author, nil
}

// GitLabPushPayload GitLab push event 部分字段
type GitLabPushPayload struct {
	Ref     string `json:"ref"`
	Project struct {
		GitHTTPURL string `json:"git_http_url"`
		GitSSHURL  string `json:"git_ssh_url"`
	} `json:"project"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
}

// ParseGitLabPush 解析 GitLab push webhook
func ParseGitLabPush(body []byte) (repoURL, branch, commitSHA, commitMessage, ref, author string, err error) {
	var p GitLabPushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", "", "", "", "", "", err
	}
	if p.Ref == "" || len(p.Commits) == 0 {
		return "", "", "", "", "", "", fmt.Errorf("invalid gitlab push payload: missing ref or commits")
	}
	ref = p.Ref
	if len(ref) > 11 && ref[:11] == "refs/heads/" {
		branch = ref[11:]
	} else {
		branch = ref
	}
	repoURL = p.Project.GitHTTPURL
	if repoURL == "" {
		repoURL = p.Project.GitSSHURL
	}
	last := p.Commits[len(p.Commits)-1]
	commitSHA = last.ID
	commitMessage = last.Message
	author = last.Author.Name
	return repoURL, branch, commitSHA, commitMessage, ref, author, nil
}

// HelmDeployRequest 一键 Helm 部署请求
type HelmDeployRequest struct {
	AppName     string
	AppID       string
	Environment string
	Version     string
	UserID      string
	UserName    string
}

// DeployHelmRelease 一键 Helm 部署：查找应用→解析参数→创建记录→异步执行
func (s *Service) DeployHelmRelease(req *HelmDeployRequest) (deploymentID string, err error) {
	if s.deploymentSvc == nil {
		return "", fmt.Errorf("deployment service not configured")
	}
	env := req.Environment
	if env == "" {
		env = "prod"
	}

	var app *model.Application
	if req.AppID != "" {
		app, err = s.appRepo.FindByID(req.AppID)
	} else if req.AppName != "" {
		var a model.Application
		if err = s.db.Where("name = ?", req.AppName).First(&a).Error; err != nil {
			return "", fmt.Errorf("application not found by name %q: %w", req.AppName, err)
		}
		app = &a
	} else {
		return "", fmt.Errorf("app_id or app_name required")
	}
	if err != nil {
		return "", fmt.Errorf("find application: %w", err)
	}

	clusterID := resolveParam(s.db, app.ID, env, "cluster")
	namespace := resolveParam(s.db, app.ID, env, "namespace")

	// 构建完整的 Helm values（根据数据库参数动态生成）
	helmValues := buildHelmValues(s.db, app.ID, env, req)

	// 默认让资源名等于应用名：standard-app chart 的 fullname 模板在 release 名不含 chart 名时
	// 会拼成 "<release>-standard-app"（如 xxl-job-admin-standard-app）。设置 fullnameOverride 去掉多余后缀。
	// 仅覆盖 fullnameOverride，不动 nameOverride，避免改变 Deployment 的不可变 selector 导致升级失败。
	if _, ok := helmValues["fullnameOverride"]; !ok {
		helmValues["fullnameOverride"] = app.Name
	}

	// executeHelmDeployment 会把 DeployConfig 解析为 model.HelmDeployConfig，
	// 真正的 Helm values 必须放在 "values" 键下，否则 cfg.Values 为空、参数不生效。
	deployConfig := map[string]interface{}{
		"values": helmValues,
	}

	// 打印 Helm values YAML 用于调试
	valuesYAML, _ := json.MarshalIndent(helmValues, "", "  ")
	fmt.Printf("[HelmDeployRelease] app=%s env=%s values=\n%s\n", app.Name, env, string(valuesYAML))

	deployReq := &svc.CreateDeploymentRequest{
		ProjectName:   app.Name,
		ProjectID:     app.ID,
		EnvName:       env,
		ClusterID:     clusterID,
		Namespace:     namespace,
		DeployType:    "helm",
		DeployConfig:  deployConfig,
		Version:       req.Version,
		CreatedBy:     req.UserID,
		CreatedByName: req.UserName,
		Description:   "Helm release: " + app.Name + "/" + env,
	}

	deployment, err := s.deploymentSvc.CreateDeployment(deployReq)
	if err != nil {
		return "", fmt.Errorf("create deployment record: %w", err)
	}

	go func(id string) {
		_ = s.deploymentSvc.ExecuteK8sDeployment(id)
	}(deployment.ID)

	return deployment.ID, nil
}

// DeployBuildMasterResult 一键发版结果
type DeployBuildMasterResult struct {
	DeploymentIDs []string     `json:"deployment_ids"`
	FailedItems   []FailedItem `json:"failed_items,omitempty"`
}

type FailedItem struct {
	AppName string `json:"app_name"`
	Tag     string `json:"tag"`
	Error   string `json:"error"`
}

// DeployBuildMaster 一键发版 Build Master 发布单，返回创建的 deployment ID 列表
func (s *Service) DeployBuildMaster(listID string, userID string, userName string) (*DeployBuildMasterResult, error) {
	var details []model.BuildMasterItemDetail
	if err := s.db.Where("list_id = ?", listID).Find(&details).Error; err != nil {
		return nil, fmt.Errorf("find details: %w", err)
	}
	if len(details) == 0 {
		return nil, fmt.Errorf("no details to deploy")
	}
	result := &DeployBuildMasterResult{}
	for _, d := range details {
		if d.AppName == "" {
			s.db.Model(&d).Update("status", model.BuildMasterItemStatusDone)
			continue
		}
		id, err := s.DeployHelmRelease(&HelmDeployRequest{
			AppName:  d.AppName,
			Version:  d.Tag,
			UserID:   userID,
			UserName: userName,
		})
		if err != nil {
			s.db.Model(&d).Updates(map[string]interface{}{
				"status": model.BuildMasterItemStatusUndone,
				"record": fmt.Sprintf("部署失败: %v", err),
			})
			result.FailedItems = append(result.FailedItems, FailedItem{
				AppName: d.AppName,
				Tag:     d.Tag,
				Error:   err.Error(),
			})
			continue
		}
		result.DeploymentIDs = append(result.DeploymentIDs, id)
		s.db.Model(&d).Updates(map[string]interface{}{
			"status": model.BuildMasterItemStatusDone,
			"record": fmt.Sprintf("部署已触发，部署ID: %s", id),
		})
	}
	var remaining []model.BuildMasterItemDetail
	if err := s.db.Where("list_id = ? AND status = ?", listID, model.BuildMasterItemStatusUndone).Find(&remaining).Error; err == nil && len(remaining) == 0 {
		s.db.Model(&model.BuildMasterList{}).Where("id = ? AND status = ?", listID, model.BuildMasterStatusReleasing).
			Update("status", model.BuildMasterStatusCompleted)
	}
	return result, nil
}

func resolveParam(db *gorm.DB, appID, env, paramName string) string {
	var cfg model.AppDeployParamConfig
	if err := db.Where("app_id = ? AND env = ? AND param_name = ?", appID, env, paramName).
		First(&cfg).Error; err == nil {
		return cfg.ParamValue
	}
	var def model.AppDeployParamDefault
	if err := db.Where("param_name = ?", paramName).First(&def).Error; err == nil {
		return def.DefaultValue
	}
	return ""
}

// buildHelmValues 从数据库读取所有 k8s.* 参数，剥掉前缀后按 Helm --set 路径语义展开为嵌套 map。
// 例如 k8s.image.repository → values["image"]["repository"]；
// k8s.env[0].name → values["env"][0]["name"]。
// helm.* 参数（chart 元信息）不会被放入 values 中。
func buildHelmValues(db *gorm.DB, appID, env string, req *HelmDeployRequest) map[string]interface{} {
	values := make(map[string]interface{})

	// 收集 app+env 级别的 k8s.* 参数
	paramMap := make(map[string]string)
	var configs []model.AppDeployParamConfig
	db.Where("app_id = ? AND env = ? AND param_name LIKE ?", appID, env, "k8s.%").Find(&configs)
	for _, cfg := range configs {
		paramMap[cfg.ParamName[4:]] = cfg.ParamValue
	}

	// 补充全局默认值（仅在 app 级别未设置时）
	var defaults []model.AppDeployParamDefault
	db.Where("param_name LIKE ?", "k8s.%").Find(&defaults)
	for _, d := range defaults {
		name := d.ParamName[4:]
		if _, exists := paramMap[name]; !exists {
			paramMap[name] = d.DefaultValue
		}
	}

	// 展开为嵌套结构
	for key, val := range paramMap {
		setNestedValue(values, key, val)
	}

	return values
}

// setNestedValue 将 "a.b[0].c" = val 展开为 map[a][b][0][c] = val
func setNestedValue(m map[string]interface{}, key string, val string) {
	parts := strings.Split(key, ".")
	insertValue(m, parts, inferValue(val))
}

// insertValue 递归创建嵌套 map/slice 并将 val 设置在叶子节点
func insertValue(m map[string]interface{}, parts []string, val interface{}) {
	if len(parts) == 0 {
		return
	}
	part := parts[0]
	name, idx, hasIndex := parseArrayIndex(part)

	if len(parts) == 1 {
		if hasIndex {
			arr := growSlice(m, name, idx+1)
			arr[idx] = val
		} else {
			m[name] = val
		}
		return
	}

	if hasIndex {
		arr := growSlice(m, name, idx+1)
		if arr[idx] == nil {
			arr[idx] = make(map[string]interface{})
		}
		if next, ok := arr[idx].(map[string]interface{}); ok {
			insertValue(next, parts[1:], val)
		} else {
			n := make(map[string]interface{})
			arr[idx] = n
			insertValue(n, parts[1:], val)
		}
	} else {
		if _, ok := m[name]; !ok {
			m[name] = make(map[string]interface{})
		}
		if next, ok := m[name].(map[string]interface{}); ok {
			insertValue(next, parts[1:], val)
		} else {
			n := make(map[string]interface{})
			m[name] = n
			insertValue(n, parts[1:], val)
		}
	}
}

// parseArrayIndex 解析 "env[0]" → ("env", 0, true)，无索引返回 (part, 0, false)
func parseArrayIndex(part string) (string, int, bool) {
	start := strings.Index(part, "[")
	if start == -1 {
		return part, 0, false
	}
	end := strings.Index(part, "]")
	if end == -1 || end <= start {
		return part, 0, false
	}
	idx, err := strconv.Atoi(part[start+1 : end])
	if err != nil {
		return part, 0, false
	}
	return part[:start], idx, true
}

// growSlice 确保 m[name] 为长度 >= minLen 的 []interface{}，不足时扩容
func growSlice(m map[string]interface{}, name string, minLen int) []interface{} {
	existing, ok := m[name]
	if ok {
		if arr, ok := existing.([]interface{}); ok {
			if len(arr) < minLen {
				newArr := make([]interface{}, minLen)
				copy(newArr, arr)
				m[name] = newArr
				return newArr
			}
			return arr
		}
	}
	arr := make([]interface{}, minLen)
	m[name] = arr
	return arr
}

// inferValue 将字符串按内容推测为 bool / int64 / float64 / string
func inferValue(s string) interface{} {
	if s == "" {
		return s
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
