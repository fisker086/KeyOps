package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/crypto"
	"github.com/fisker086/keyops/pkg/logger"
)

type JenkinsService struct {
	jenkinsRepo    *repository.JenkinsRepository
	deploymentRepo *repository.DeploymentRepository
	crypto         *crypto.Crypto
}

func NewJenkinsService(jenkinsRepo *repository.JenkinsRepository, deploymentRepo *repository.DeploymentRepository, cryptoService *crypto.Crypto) *JenkinsService {
	return &JenkinsService{
		jenkinsRepo:    jenkinsRepo,
		deploymentRepo: deploymentRepo,
		crypto:         cryptoService,
	}
}

// JenkinsClient Jenkins客户端
type JenkinsClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewJenkinsClient 创建Jenkins客户端
func NewJenkinsClient(baseURL string, username, password string) *JenkinsClient {
	return &JenkinsClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest 发送HTTP请求
func (jc *JenkinsClient) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	reqURL := jc.BaseURL + endpoint
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(jc.Username, jc.Password)
	req.Header.Set("Content-Type", "application/json")

	return jc.HTTPClient.Do(req)
}

// getJenkinsClient 获取Jenkins客户端
func (s *JenkinsService) getJenkinsClient(serverID uint) (*JenkinsClient, error) {
	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("获取Jenkins服务器失败: %v", err)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("Jenkins服务器已禁用")
	}

	// 获取密码或Token（统一存储在Password字段）
	credential := server.Password
	if s.crypto.IsEncrypted(credential) {
		decryptedPassword, err := s.crypto.Decrypt(credential)
		if err != nil {
			logger.Errorf("解密Jenkins密码/Token失败 (serverID: %d): %v", serverID, err)
			return nil, fmt.Errorf("解密Jenkins密码/Token失败: %v", err)
		}
		credential = decryptedPassword
		logger.Debugf("成功解密Jenkins密码/Token (serverID: %d)", serverID)
	} else if credential != "" {
		// 密码未加密，可能是旧数据，记录警告
		logger.Warnf("检测到未加密的Jenkins密码/Token (serverID: %d)，建议更新密码以启用加密", serverID)
	}

	return NewJenkinsClient(server.URL, server.Username, credential), nil
}

// GetJenkinsServers 获取Jenkins服务器列表
func (s *JenkinsService) GetJenkinsServers(page, pageSize int) ([]model.JenkinsServerInfo, int64, error) {
	servers, total, err := s.jenkinsRepo.List(page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var serverList []model.JenkinsServerInfo
	for _, server := range servers {
		serverInfo := model.JenkinsServerInfo{
			ID:          server.ID,
			Alias:       server.Alias,
			URL:         server.URL,
			Username:    server.Username,
			Description: server.Description,
			Enabled:     server.Enabled,
			CreatedAt:   server.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   server.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		serverList = append(serverList, serverInfo)
	}

	return serverList, total, nil
}

// GetJenkinsServerDetail 获取Jenkins服务器详情
func (s *JenkinsService) GetJenkinsServerDetail(serverID uint) (*model.JenkinsServerInfo, error) {
	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("获取Jenkins服务器详情失败: %v", err)
	}

	serverInfo := &model.JenkinsServerInfo{
		ID:          server.ID,
		Alias:       server.Alias,
		URL:         server.URL,
		Username:    server.Username,
		Description: server.Description,
		Enabled:     server.Enabled,
		CreatedAt:   server.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   server.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	return serverInfo, nil
}

// CreateJenkinsServer 创建Jenkins服务器
func (s *JenkinsService) CreateJenkinsServer(req *model.CreateJenkinsServerRequest) (*model.JenkinsServer, error) {
	// 验证必填字段
	if req.Username == "" {
		return nil, fmt.Errorf("用户名不能为空")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("密码或API Token不能为空")
	}

	server := &model.JenkinsServer{
		Alias:       req.Alias,
		URL:         req.URL,
		Username:    req.Username,
		Description: req.Description,
		Enabled:     req.Enabled,
	}

	// 加密密码或Token
	encryptedPassword, err := s.crypto.Encrypt(req.Password)
	if err != nil {
		logger.Errorf("加密Jenkins密码/Token失败: %v", err)
		return nil, fmt.Errorf("加密Jenkins密码/Token失败: %v", err)
	}
	server.Password = encryptedPassword

	if err := s.jenkinsRepo.Create(server); err != nil {
		return nil, fmt.Errorf("创建Jenkins服务器失败: %v", err)
	}

	logger.Infof("创建Jenkins服务器成功: %s (%s)", server.Alias, server.URL)

	return server, nil
}

// UpdateJenkinsServer 更新Jenkins服务器
func (s *JenkinsService) UpdateJenkinsServer(serverID uint, req *model.UpdateJenkinsServerRequest) error {
	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		return fmt.Errorf("获取Jenkins服务器失败: %v", err)
	}

	if req.Alias != "" {
		server.Alias = req.Alias
	}
	if req.URL != "" {
		server.URL = req.URL
	}
	if req.Username != "" {
		server.Username = req.Username
	}
	if req.Password != "" {
		encryptedPassword, err := s.crypto.Encrypt(req.Password)
		if err != nil {
			logger.Errorf("加密Jenkins密码/Token失败: %v", err)
			return fmt.Errorf("加密Jenkins密码/Token失败: %v", err)
		}
		server.Password = encryptedPassword
	}
	if req.Description != "" {
		server.Description = req.Description
	}
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}

	if err := s.jenkinsRepo.Update(server); err != nil {
		return fmt.Errorf("更新Jenkins服务器失败: %v", err)
	}

	logger.Infof("更新Jenkins服务器成功: %s (ID: %d)", server.Alias, serverID)
	return nil
}

// DeleteJenkinsServer 删除Jenkins服务器
func (s *JenkinsService) DeleteJenkinsServer(serverID uint) error {
	// 先检查服务器是否存在
	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		return fmt.Errorf("获取Jenkins服务器失败: %v", err)
	}

	if err := s.jenkinsRepo.Delete(serverID); err != nil {
		logger.Errorf("删除Jenkins服务器失败 (serverID: %d): %v", serverID, err)
		return fmt.Errorf("删除Jenkins服务器失败: %v", err)
	}

	logger.Infof("删除Jenkins服务器成功: %s (ID: %d)", server.Alias, serverID)
	return nil
}

// TestJenkinsConnection 测试Jenkins连接
func (s *JenkinsService) TestJenkinsConnection(req *model.TestJenkinsConnectionRequest) (*model.TestJenkinsConnectionResponse, error) {
	logger.Debugf("开始测试Jenkins连接: %s", req.URL)

	// 验证 URL 格式
	u, err := url.Parse(req.URL)
	if err != nil {
		logger.Errorf("解析Jenkins URL失败: %v", err)
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "无效的Jenkins URL",
			Error:   err.Error(),
		}, nil
	}

	if u.Hostname() == "" {
		logger.Errorf("无效的Jenkins主机地址: %s", req.URL)
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "无效的Jenkins主机地址，请检查服务器地址格式",
			Error:   "主机地址为空",
		}, nil
	}

	// 验证必填字段
	if req.Username == "" {
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "用户名不能为空",
		}, nil
	}
	if req.Password == "" {
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "密码或API Token不能为空",
		}, nil
	}

	client := NewJenkinsClient(req.URL, req.Username, req.Password)

	// 获取系统信息来测试连接
	resp, err := client.makeRequest("GET", "/api/json", nil)
	if err != nil {
		logger.Errorf("连接Jenkins失败 (%s): %v", req.URL, err)
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "连接Jenkins失败",
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 读取响应体以获取更详细的错误信息
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		var errorMsg string
		if resp.StatusCode == http.StatusUnauthorized {
			errorMsg = "认证失败 (401)：请使用用户名 + API Token（Jenkins 用户配置中生成），勿用登录密码。"
			logger.Errorf("Jenkins认证失败 (%s): 用户名=%s, 响应体=%s", req.URL, req.Username, bodyStr)
		} else {
			errorMsg = fmt.Sprintf("Jenkins响应错误: %d", resp.StatusCode)
			logger.Errorf("Jenkins响应错误 (%s, status: %d, body: %s)", req.URL, resp.StatusCode, bodyStr)
		}

		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: errorMsg,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyStr),
		}, nil
	}

	var systemInfo model.JenkinsSystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&systemInfo); err != nil {
		logger.Errorf("解析Jenkins响应失败: %v", err)
		return &model.TestJenkinsConnectionResponse{
			Success: false,
			Message: "解析Jenkins响应失败",
			Error:   err.Error(),
		}, nil
	}

	logger.Infof("Jenkins连接测试成功 (%s, version: %s)", req.URL, systemInfo.Version)
	return &model.TestJenkinsConnectionResponse{
		Success:    true,
		Message:    "连接成功",
		SystemInfo: &systemInfo,
	}, nil
}

// GetJobs 获取任务列表
func (s *JenkinsService) GetJobs(serverID uint) (*model.JenkinsJobListResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins服务器信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取Jenkins服务器信息失败: %v", err)
	}
	serverName := server.Alias

	logger.Debugf("开始获取Jenkins任务列表 (serverID: %d, server: %s)", serverID, serverName)
	resp, err := client.makeRequest("GET", "/api/json?tree=jobs[name,displayName,description,url,buildable,color,_class,lastBuild[number,url,displayName,result,building,duration,timestamp],lastStableBuild[number,url,displayName,result],lastSuccessfulBuild[number,url,displayName,result],lastFailedBuild[number,url,displayName,result]]", nil)
	if err != nil {
		logger.Errorf("请求Jenkins API失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取任务列表失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, status: %d)", serverID, resp.StatusCode)
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var jenkinsData struct {
		Jobs []model.JenkinsJob `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jenkinsData); err != nil {
		logger.Errorf("解析Jenkins响应失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	logger.Infof("成功获取Jenkins任务列表 (serverID: %d, 任务数: %d)", serverID, len(jenkinsData.Jobs))
	return &model.JenkinsJobListResponse{
		Jobs:   jenkinsData.Jobs,
		Total:  len(jenkinsData.Jobs),
		Server: serverName,
	}, nil
}

// SearchJobs 搜索Jenkins任务
func (s *JenkinsService) SearchJobs(serverID uint, keyword string) (*model.JenkinsJobListResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		return nil, err
	}

	server, _ := s.jenkinsRepo.GetByID(serverID)
	serverName := server.Alias

	// 获取所有任务
	resp, err := client.makeRequest("GET", "/api/json?tree=jobs[name,displayName,description,url,buildable,color,_class,lastBuild[number,url,displayName,result,building,duration,timestamp],lastStableBuild[number,url,displayName,result],lastSuccessfulBuild[number,url,displayName,result],lastFailedBuild[number,url,displayName,result]]", nil)
	if err != nil {
		return nil, fmt.Errorf("获取任务列表失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var jenkinsData struct {
		Jobs []model.JenkinsJob `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jenkinsData); err != nil {
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	// 过滤任务 - 支持名称、显示名称、描述的模糊匹配
	var filteredJobs []model.JenkinsJob
	keywordLower := strings.ToLower(keyword)

	for _, job := range jenkinsData.Jobs {
		// 检查任务名称
		if strings.Contains(strings.ToLower(job.Name), keywordLower) {
			filteredJobs = append(filteredJobs, job)
			continue
		}
		// 检查显示名称
		if strings.Contains(strings.ToLower(job.DisplayName), keywordLower) {
			filteredJobs = append(filteredJobs, job)
			continue
		}
		// 检查描述
		if strings.Contains(strings.ToLower(job.Description), keywordLower) {
			filteredJobs = append(filteredJobs, job)
			continue
		}
	}

	return &model.JenkinsJobListResponse{
		Jobs:   filteredJobs,
		Total:  len(filteredJobs),
		Server: serverName,
	}, nil
}

// GetJobDetail 获取任务详情
func (s *JenkinsService) GetJobDetail(serverID uint, jobName string) (*model.JenkinsJobDetailResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins服务器信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取Jenkins服务器信息失败: %v", err)
	}
	serverName := server.Alias

	logger.Debugf("开始获取Jenkins任务详情 (serverID: %d, job: %s)", serverID, jobName)
	// 获取任务详情
	jobURL := fmt.Sprintf("/job/%s/api/json", url.PathEscape(jobName))
	resp, err := client.makeRequest("GET", jobURL, nil)
	if err != nil {
		logger.Errorf("获取Jenkins任务详情失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
		return nil, fmt.Errorf("获取任务详情失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, job: %s, status: %d)", serverID, jobName, resp.StatusCode)
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var job model.JenkinsJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		logger.Errorf("解析Jenkins任务详情失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	// 获取构建历史
	buildsURL := fmt.Sprintf("/job/%s/api/json?tree=builds[number,url,displayName,result,building,duration,timestamp,keepLog,queueId]", url.PathEscape(jobName))
	resp, err = client.makeRequest("GET", buildsURL, nil)
	if err != nil {
		logger.Warnf("获取构建历史失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
		// 构建历史获取失败不影响任务详情返回
		return &model.JenkinsJobDetailResponse{
			Job:    job,
			Builds: []model.JenkinsBuild{},
			Server: serverName,
		}, nil
	}
	defer resp.Body.Close()

	var buildsData struct {
		Builds []model.JenkinsBuild `json:"builds"`
	}

	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&buildsData); err != nil {
			logger.Warnf("解析构建历史失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
		}
	}

	logger.Infof("成功获取Jenkins任务详情 (serverID: %d, job: %s, 构建数: %d)", serverID, jobName, len(buildsData.Builds))
	return &model.JenkinsJobDetailResponse{
		Job:    job,
		Builds: buildsData.Builds,
		Server: serverName,
	}, nil
}

// StartJob 启动任务
func (s *JenkinsService) StartJob(serverID uint, jobName string, req *model.StartJobRequest) (*model.StartJobResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins服务器信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取Jenkins服务器信息失败: %v", err)
	}
	serverName := server.Alias

	var buildURL string
	if len(req.Parameters) > 0 {
		// 带参数构建
		buildURL = fmt.Sprintf("/job/%s/buildWithParameters", url.PathEscape(jobName))

		// 构建表单数据
		formData := url.Values{}
		for key, value := range req.Parameters {
			formData.Set(key, value)
		}

		logger.Infof("启动Jenkins任务 (serverID: %d, job: %s, 带参数: %d个)", serverID, jobName, len(req.Parameters))
		resp, err := client.makeRequest("POST", buildURL+"?"+formData.Encode(), nil)
		if err != nil {
			logger.Errorf("启动Jenkins任务失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
			return nil, fmt.Errorf("启动任务失败: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			logger.Errorf("Jenkins API响应错误 (serverID: %d, job: %s, status: %d)", serverID, jobName, resp.StatusCode)
			return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
		}
	} else {
		// 无参数构建
		buildURL = fmt.Sprintf("/job/%s/build", url.PathEscape(jobName))

		logger.Infof("启动Jenkins任务 (serverID: %d, job: %s, 无参数)", serverID, jobName)
		resp, err := client.makeRequest("POST", buildURL, nil)
		if err != nil {
			logger.Errorf("启动Jenkins任务失败 (serverID: %d, job: %s): %v", serverID, jobName, err)
			return nil, fmt.Errorf("启动任务失败: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			logger.Errorf("Jenkins API响应错误 (serverID: %d, job: %s, status: %d)", serverID, jobName, resp.StatusCode)
			return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
		}
	}

	logger.Infof("Jenkins任务启动成功 (serverID: %d, job: %s)", serverID, jobName)
	return &model.StartJobResponse{
		Success: true,
		Message: "任务启动成功",
		JobName: jobName,
		Server:  serverName,
	}, nil
}

// StopBuild 停止构建
func (s *JenkinsService) StopBuild(serverID uint, jobName string, buildNumber int) (*model.StopBuildResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins服务器信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取Jenkins服务器信息失败: %v", err)
	}
	serverName := server.Alias

	logger.Infof("停止Jenkins构建 (serverID: %d, job: %s, build: #%d)", serverID, jobName, buildNumber)
	stopURL := fmt.Sprintf("/job/%s/%d/stop", url.PathEscape(jobName), buildNumber)
	resp, err := client.makeRequest("POST", stopURL, nil)
	if err != nil {
		logger.Errorf("停止Jenkins构建失败 (serverID: %d, job: %s, build: #%d): %v", serverID, jobName, buildNumber, err)
		return nil, fmt.Errorf("停止构建失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, job: %s, build: #%d, status: %d)", serverID, jobName, buildNumber, resp.StatusCode)
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	logger.Infof("Jenkins构建停止成功 (serverID: %d, job: %s, build: #%d)", serverID, jobName, buildNumber)
	return &model.StopBuildResponse{
		Success:     true,
		Message:     "构建停止成功",
		JobName:     jobName,
		BuildNumber: buildNumber,
		Server:      serverName,
	}, nil
}

// GetBuildDetail 获取构建详情
func (s *JenkinsService) GetBuildDetail(serverID uint, jobName string, buildNumber int) (*model.JenkinsBuildDetailResponse, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		return nil, err
	}

	server, _ := s.jenkinsRepo.GetByID(serverID)
	serverName := server.Alias

	buildURL := fmt.Sprintf("/job/%s/%d/api/json", url.PathEscape(jobName), buildNumber)
	resp, err := client.makeRequest("GET", buildURL, nil)
	if err != nil {
		return nil, fmt.Errorf("获取构建详情失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var build model.JenkinsBuild
	if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	return &model.JenkinsBuildDetailResponse{
		Build:  build,
		Server: serverName,
	}, nil
}

// GetBuildLog 获取构建日志
// 优先从数据库读取已保存的日志，如果不存在或start>0，则从Jenkins API获取
func (s *JenkinsService) GetBuildLog(serverID uint, jobName string, buildNumber int, start int) (*model.GetBuildLogResponse, error) {
	server, err := s.jenkinsRepo.GetByID(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins服务器信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取Jenkins服务器信息失败: %v", err)
	}
	serverName := server.Alias

	// 尝试从数据库查找对应的部署记录
	deployment, err := s.deploymentRepo.FindByJenkinsBuild(jobName, buildNumber)
	if err == nil && deployment != nil && deployment.BuildLog != "" && start == 0 {
		// 如果数据库中有完整日志，且start=0（从头开始），直接返回数据库中的日志
		logger.Debugf("从数据库读取Jenkins构建日志 (job: %s, build: #%d)", jobName, buildNumber)
		return &model.GetBuildLogResponse{
			Log:         deployment.BuildLog,
			HasMore:     false,
			TextSize:    len(deployment.BuildLog),
			MoreData:    false,
			JobName:     jobName,
			BuildNumber: buildNumber,
			Server:      serverName,
		}, nil
	}

	// 从Jenkins API获取日志
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		// 如果Jenkins API失败，但数据库有日志，返回数据库日志
		if deployment != nil && deployment.BuildLog != "" {
			logger.Warnf("Jenkins API失败，使用数据库中的日志 (job: %s, build: #%d)", jobName, buildNumber)
			return &model.GetBuildLogResponse{
				Log:         deployment.BuildLog,
				HasMore:     false,
				TextSize:    len(deployment.BuildLog),
				MoreData:    false,
				JobName:     jobName,
				BuildNumber: buildNumber,
				Server:      serverName,
			}, nil
		}
		return nil, err
	}

	logger.Debugf("从Jenkins API获取构建日志 (serverID: %d, job: %s, build: #%d, start: %d)", serverID, jobName, buildNumber, start)
	logURL := fmt.Sprintf("/job/%s/%d/logText/progressiveText?start=%d", url.PathEscape(jobName), buildNumber, start)
	resp, err := client.makeRequest("GET", logURL, nil)
	if err != nil {
		logger.Errorf("获取Jenkins构建日志失败 (serverID: %d, job: %s, build: #%d): %v", serverID, jobName, buildNumber, err)
		// 如果Jenkins API失败，但数据库有日志，返回数据库日志
		if deployment != nil && deployment.BuildLog != "" {
			logger.Warnf("Jenkins API失败，使用数据库中的日志 (job: %s, build: #%d)", jobName, buildNumber)
			return &model.GetBuildLogResponse{
				Log:         deployment.BuildLog,
				HasMore:     false,
				TextSize:    len(deployment.BuildLog),
				MoreData:    false,
				JobName:     jobName,
				BuildNumber: buildNumber,
				Server:      serverName,
			}, nil
		}
		return nil, fmt.Errorf("获取构建日志失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, job: %s, build: #%d, status: %d)", serverID, jobName, buildNumber, resp.StatusCode)
		// 如果Jenkins API返回错误，但数据库有日志，返回数据库日志
		if deployment != nil && deployment.BuildLog != "" {
			logger.Warnf("Jenkins API返回错误，使用数据库中的日志 (job: %s, build: #%d)", jobName, buildNumber)
			return &model.GetBuildLogResponse{
				Log:         deployment.BuildLog,
				HasMore:     false,
				TextSize:    len(deployment.BuildLog),
				MoreData:    false,
				JobName:     jobName,
				BuildNumber: buildNumber,
				Server:      serverName,
			}, nil
		}
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	logBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("读取Jenkins构建日志内容失败 (serverID: %d, job: %s, build: #%d): %v", serverID, jobName, buildNumber, err)
		return nil, fmt.Errorf("读取日志内容失败: %v", err)
	}

	// 检查是否有更多日志
	moreData := resp.Header.Get("X-More-Data") == "true"
	textSize, _ := strconv.Atoi(resp.Header.Get("X-Text-Size"))

	logContent := string(logBytes)

	// 如果构建已完成且没有更多数据，且start=0（完整日志），保存到数据库
	if !moreData && start == 0 && deployment != nil {
		// 检查构建是否已完成（通过检查构建详情）
		buildDetail, err := s.GetBuildDetail(serverID, jobName, buildNumber)
		if err == nil && buildDetail != nil && !buildDetail.Build.Building {
			// 构建已完成，保存完整日志到数据库
			if err := s.deploymentRepo.SaveBuildLog(deployment.ID, logContent); err != nil {
				logger.Warnf("保存Jenkins构建日志到数据库失败 (deploymentID: %s, job: %s, build: #%d): %v", deployment.ID, jobName, buildNumber, err)
			} else {
				logger.Infof("已保存Jenkins构建日志到数据库 (deploymentID: %s, job: %s, build: #%d)", deployment.ID, jobName, buildNumber)
			}
		}
	}

	return &model.GetBuildLogResponse{
		Log:         logContent,
		HasMore:     moreData,
		TextSize:    textSize,
		MoreData:    moreData,
		JobName:     jobName,
		BuildNumber: buildNumber,
		Server:      serverName,
	}, nil
}

// GetSystemInfo 获取系统信息
func (s *JenkinsService) GetSystemInfo(serverID uint) (*model.JenkinsSystemInfo, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	logger.Debugf("获取Jenkins系统信息 (serverID: %d)", serverID)
	resp, err := client.makeRequest("GET", "/api/json", nil)
	if err != nil {
		logger.Errorf("请求Jenkins系统信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取系统信息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, status: %d)", serverID, resp.StatusCode)
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var systemInfo model.JenkinsSystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&systemInfo); err != nil {
		logger.Errorf("解析Jenkins系统信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	logger.Debugf("成功获取Jenkins系统信息 (serverID: %d, version: %s)", serverID, systemInfo.Version)
	return &systemInfo, nil
}

// GetQueueInfo 获取队列信息
func (s *JenkinsService) GetQueueInfo(serverID uint) (*model.JenkinsQueue, error) {
	client, err := s.getJenkinsClient(serverID)
	if err != nil {
		logger.Errorf("获取Jenkins客户端失败 (serverID: %d): %v", serverID, err)
		return nil, err
	}

	logger.Debugf("获取Jenkins队列信息 (serverID: %d)", serverID)
	resp, err := client.makeRequest("GET", "/queue/api/json", nil)
	if err != nil {
		logger.Errorf("请求Jenkins队列信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("获取队列信息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Jenkins API响应错误 (serverID: %d, status: %d)", serverID, resp.StatusCode)
		return nil, fmt.Errorf("Jenkins响应错误: %d", resp.StatusCode)
	}

	var queueInfo model.JenkinsQueue
	if err := json.NewDecoder(resp.Body).Decode(&queueInfo); err != nil {
		logger.Errorf("解析Jenkins队列信息失败 (serverID: %d): %v", serverID, err)
		return nil, fmt.Errorf("解析Jenkins响应失败: %v", err)
	}

	logger.Debugf("成功获取Jenkins队列信息 (serverID: %d, 队列项数: %d)", serverID, len(queueInfo.Items))
	return &queueInfo, nil
}
