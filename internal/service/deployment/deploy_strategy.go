package deployment

import (
	"context"
	"github.com/fisker086/keyops/internal/model"
)

// DeployStrategy 发布策略接口
// 每种发布方式（Jenkins, GitOps, ArgoCD, Helm等）都需要实现此接口
type DeployStrategy interface {
	// GetDeployType 返回部署类型
	GetDeployType() string
	
	// ValidateConfig 验证配置
	ValidateConfig(config model.DeployTypeConfig) error
	
	// Execute 执行部署
	Execute(ctx context.Context, deployment *model.Deployment, config model.DeployTypeConfig) error
	
	// GetStatus 获取部署状态
	GetStatus(ctx context.Context, deployment *model.Deployment) (string, error)
	
	// GetLogs 获取部署日志
	GetLogs(ctx context.Context, deployment *model.Deployment) (string, error)
	
	// Cancel 取消部署
	Cancel(ctx context.Context, deployment *model.Deployment) error
}

// DeployStrategyRegistry 发布策略注册表
type DeployStrategyRegistry struct {
	strategies map[string]DeployStrategy
}

// NewDeployStrategyRegistry 创建策略注册表
func NewDeployStrategyRegistry() *DeployStrategyRegistry {
	return &DeployStrategyRegistry{
		strategies: make(map[string]DeployStrategy),
	}
}

// Register 注册发布策略
func (r *DeployStrategyRegistry) Register(strategy DeployStrategy) {
	r.strategies[strategy.GetDeployType()] = strategy
}

// GetStrategy 获取发布策略
func (r *DeployStrategyRegistry) GetStrategy(deployType string) (DeployStrategy, bool) {
	strategy, ok := r.strategies[deployType]
	return strategy, ok
}

// GetAllStrategies 获取所有已注册的策略
func (r *DeployStrategyRegistry) GetAllStrategies() map[string]DeployStrategy {
	return r.strategies
}

