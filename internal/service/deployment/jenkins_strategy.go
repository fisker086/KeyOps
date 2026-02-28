package deployment

import (
	"context"
	"fmt"

	"github.com/fisker086/keyops/internal/model"
	jenkinsService "github.com/fisker086/keyops/internal/service/jenkins"
)

// JenkinsDeployStrategy Jenkins发布策略实现
type JenkinsDeployStrategy struct {
	jenkinsService *jenkinsService.JenkinsService
}

// NewJenkinsDeployStrategy 创建Jenkins发布策略
func NewJenkinsDeployStrategy(jenkinsService *jenkinsService.JenkinsService) *JenkinsDeployStrategy {
	return &JenkinsDeployStrategy{
		jenkinsService: jenkinsService,
	}
}

func (s *JenkinsDeployStrategy) GetDeployType() string {
	return model.DeployTypeJenkins
}

func (s *JenkinsDeployStrategy) ValidateConfig(config model.DeployTypeConfig) error {
	jenkinsConfig, ok := config.(*model.JenkinsDeployConfig)
	if !ok {
		return fmt.Errorf("invalid config type for Jenkins deployment")
	}
	return jenkinsConfig.Validate()
}

func (s *JenkinsDeployStrategy) Execute(ctx context.Context, deployment *model.Deployment, config model.DeployTypeConfig) error {
	jenkinsConfig, ok := config.(*model.JenkinsDeployConfig)
	if !ok {
		return fmt.Errorf("invalid config type for Jenkins deployment")
	}

	// 构建StartJobRequest
	req := &model.StartJobRequest{
		Parameters: jenkinsConfig.BuildParameters,
	}

	// 调用Jenkins服务执行构建
	_, err := s.jenkinsService.StartJob(
		uint(jenkinsConfig.JenkinsServerID),
		jenkinsConfig.JenkinsJob,
		req,
	)
	if err != nil {
		return fmt.Errorf("failed to start Jenkins job: %w", err)
	}

	// StartJobResponse 不包含 buildNumber，需要从队列或后续查询获取
	// 这里先设置job名称，buildNumber会在后续状态查询时更新
	deployment.JenkinsJob = jenkinsConfig.JenkinsJob

	// 注意：Jenkins的StartJob不会立即返回buildNumber，需要通过队列查询获取
	// 这里暂时不设置buildNumber，等待后续状态轮询时更新

	return nil
}

func (s *JenkinsDeployStrategy) GetStatus(ctx context.Context, deployment *model.Deployment) (string, error) {
	if deployment.JenkinsBuildNumber == 0 {
		// 如果没有buildNumber，尝试从队列中获取
		// 这里暂时返回pending，实际应该查询队列
		return model.DeploymentStatusPending, nil
	}

	// 从deployment配置中获取Jenkins服务器ID
	serverID, err := GetJenkinsServerIDFromDeployment(deployment)
	if err != nil {
		return "", fmt.Errorf("failed to get Jenkins server ID: %w", err)
	}

	// 从Jenkins获取构建详情
	buildDetail, err := s.jenkinsService.GetBuildDetail(
		serverID,
		deployment.JenkinsJob,
		deployment.JenkinsBuildNumber,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get Jenkins build status: %w", err)
	}

	build := buildDetail.Build

	// 转换Jenkins状态到部署状态
	if build.Building {
		return model.DeploymentStatusRunning, nil
	}

	switch build.Result {
	case "SUCCESS":
		return model.DeploymentStatusSuccess, nil
	case "FAILURE", "UNSTABLE":
		return model.DeploymentStatusFailed, nil
	case "ABORTED":
		return model.DeploymentStatusCancelled, nil
	default:
		return model.DeploymentStatusPending, nil
	}
}

func (s *JenkinsDeployStrategy) GetLogs(ctx context.Context, deployment *model.Deployment) (string, error) {
	if deployment.JenkinsBuildNumber == 0 {
		return "", fmt.Errorf("no build number available")
	}

	// 从deployment配置中获取Jenkins服务器ID
	serverID, err := GetJenkinsServerIDFromDeployment(deployment)
	if err != nil {
		return "", fmt.Errorf("failed to get Jenkins server ID: %w", err)
	}

	// 从Jenkins获取构建日志（start=0表示从头开始）
	logResponse, err := s.jenkinsService.GetBuildLog(
		serverID,
		deployment.JenkinsJob,
		deployment.JenkinsBuildNumber,
		0, // start from beginning
	)
	if err != nil {
		return "", fmt.Errorf("failed to get Jenkins build logs: %w", err)
	}

	return logResponse.Log, nil
}

func (s *JenkinsDeployStrategy) Cancel(ctx context.Context, deployment *model.Deployment) error {
	if deployment.JenkinsBuildNumber == 0 {
		return fmt.Errorf("no build number available")
	}

	// 从deployment配置中获取Jenkins服务器ID
	serverID, err := GetJenkinsServerIDFromDeployment(deployment)
	if err != nil {
		return fmt.Errorf("failed to get Jenkins server ID: %w", err)
	}

	// 停止Jenkins构建
	_, err = s.jenkinsService.StopBuild(
		serverID,
		deployment.JenkinsJob,
		deployment.JenkinsBuildNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to stop Jenkins build: %w", err)
	}

	return nil
}
