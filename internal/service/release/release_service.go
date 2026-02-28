package release

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	jenkinsService "github.com/fisker086/keyops/internal/service/jenkins"
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
	repo              *repository.ReleaseRunRepository
	db                *gorm.DB
	appRepo           *repository.ApplicationRepository
	bindingRepo       *repository.ApplicationDeployBindingRepository
	settingRepo       *repository.SettingRepository
	jenkinsSvc        *jenkinsService.JenkinsService
	deployProdStarter DeployProdStarter // 若设置则审批通过后走编排（如 Temporal），否则直接执行
}

func NewService(repo *repository.ReleaseRunRepository) *Service {
	return &Service{repo: repo}
}

// SetDependencies 注入 DB、应用、绑定、设置与 Jenkins 依赖（用于执行发布与创建 prod 审批）
func (s *Service) SetDependencies(db *gorm.DB, appRepo *repository.ApplicationRepository, bindingRepo *repository.ApplicationDeployBindingRepository, jenkinsSvc *jenkinsService.JenkinsService) {
	s.db = db
	s.appRepo = appRepo
	s.bindingRepo = bindingRepo
	s.jenkinsSvc = jenkinsSvc
}

// SetSettingRepository 注入设置仓库（用于读取 release_approval 创建第三方审批）
func (s *Service) SetSettingRepository(repo *repository.SettingRepository) {
	s.settingRepo = repo
}

// SetDeployProdStarter 注入生产发布编排器（如 Temporal）；为 nil 时审批通过后直接执行
func (s *Service) SetDeployProdStarter(starter DeployProdStarter) {
	s.deployProdStarter = starter
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

// List 分页列表
func (s *Service) List(repoURL, branch, status string, page, pageSize int) ([]model.ReleaseRun, int64, error) {
	return s.repo.List(repoURL, branch, status, page, pageSize)
}

// GetByID 根据 ID 获取
func (s *Service) GetByID(id string) (*model.ReleaseRun, error) {
	return s.repo.GetByID(id)
}

// UpdateRunStatus 更新 run 状态（Jenkins 回调或人工标记成功/失败后调用，用于回滚源）
func (s *Service) UpdateRunStatus(id string, status string, completedAt *time.Time) error {
	return s.repo.UpdateStatus(id, status, nil, completedAt)
}

// DeployConfigForApproval prod 审批单中 DeployConfig 的 JSON 结构
type DeployConfigForApproval struct {
	ReleaseRunID  string `json:"release_run_id"`
	Environment   string `json:"environment"`
	ApplicationID string `json:"application_id"`
}

// ExecuteRun 执行一条发布记录：按环境选择绑定并触发 Jenkins，或提交 prod 工单
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

	// 非 prod：直接执行 Jenkins
	return false, "", s.doExecuteRun(run, applicationID, environment)
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
		ID:           uuid.New().String(),
		Title:        title,
		Description:  fmt.Sprintf("发布代码记录 %s，环境 prod", releaseRunID),
		Type:         model.ApprovalTypeDeployment,
		Status:       model.ApprovalStatusPending,
		Platform:     model.ApprovalPlatformInternal,
		ApplicantID:  applicantID,
		ApplicantName: applicantName,
		DeployConfig: string(cfgJSON),
		CreatedAt:    now,
		UpdatedAt:    now,
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

	// 目前仅实现飞书：按字段名称构建表单并创建实例
	if platform == "feishu" {
		provider := approval.NewFeishuProvider(config, s.db)
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
		a.ExternalURL = fmt.Sprintf("https://www.feishu.cn/approval/instance/%s", externalID)
		return s.db.Save(a).Error
	}
	// 钉钉/企微后续可按同样方式接入
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
	return s.doExecuteRun(run, applicationID, environment)
}

// doExecuteRun 根据应用+环境触发 Jenkins（带发布策略参数），并更新 run 状态与部署环境
func (s *Service) doExecuteRun(run *model.ReleaseRun, applicationID string, environment string) error {
	if s.jenkinsSvc == nil || s.bindingRepo == nil {
		return fmt.Errorf("release execute not configured: missing jenkins or binding")
	}
	binding, err := s.bindingRepo.FindByApplicationDeployTypeAndEnvironment(applicationID, "jenkins", environment)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("no Jenkins binding for environment %q, configure in 应用-发布", environment)
		}
		return err
	}
	serverID, err := strconv.ParseUint(binding.DeployConfigID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid jenkins server id in binding: %s", binding.DeployConfigID)
	}
	jobName := binding.JenkinsJob
	if jobName == "" {
		return fmt.Errorf("jenkins job name is empty in binding")
	}
	params := map[string]string{
		"BRANCH":     run.Branch,
		"COMMIT":     run.CommitSHA,
		"GIT_COMMIT": run.CommitSHA,
	}
	// 发布策略：run 单次覆盖 > binding 默认
	strategy := run.DeployStrategy
	if strategy == "" {
		strategy = binding.DeployStrategy
	}
	if strategy == "" {
		strategy = model.DeployStrategyRolling
	}
	params["DEPLOY_STRATEGY"] = strategy
	if binding.StrategyOptions != "" {
		params["STRATEGY_OPTIONS"] = binding.StrategyOptions
	}
	_, err = s.jenkinsSvc.StartJob(uint(serverID), jobName, &model.StartJobRequest{Parameters: params})
	if err != nil {
		return fmt.Errorf("start jenkins job: %w", err)
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
	Ref          string `json:"ref"`
	Project      struct {
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
