package deployment

import (
	"encoding/json"
	"fmt"
	"github.com/fisker086/keyops/internal/model"
)

// ParseDeployConfig 解析部署配置JSON字符串为对应的配置结构
func ParseDeployConfig(deployType string, configJSON string) (model.DeployTypeConfig, error) {
	switch deployType {
	case model.DeployTypeK8s:
		var config model.K8sDeployConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return nil, fmt.Errorf("failed to parse K8s config: %w", err)
		}
		return &config, nil
	case model.DeployTypeGitOps:
		var config model.GitOpsDeployConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return nil, fmt.Errorf("failed to parse GitOps config: %w", err)
		}
		return &config, nil
	case model.DeployTypeHelm:
		var config model.HelmDeployConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return nil, fmt.Errorf("failed to parse Helm config: %w", err)
		}
		return &config, nil
	default:
		return nil, fmt.Errorf("unsupported deploy type: %s", deployType)
	}
}
