package aiassistant

import (
	"os"
	"strings"
)

// EnvManager 环境配置：环境来自数据库 ai_assistant_environments，此处仅兜底 ENV_* 多环境
type EnvManager struct{}

// NewEnvManager 创建
func NewEnvManager() *EnvManager {
	return &EnvManager{}
}

// ListEnvironments 列出所有环境（环境已通过数据库 ai_assistant_environments 配置，此处仅兜底 ENV_*）
func (m *EnvManager) ListEnvironments() []Environment {
	return listEnvironmentsFromEnv()
}

// GetEnvironment 按 id 获取
func (m *EnvManager) GetEnvironment(envID string) *Environment {
	for _, e := range m.ListEnvironments() {
		if e.ID == envID {
			return &e
		}
	}
	return nil
}

func listEnvironmentsFromEnv() []Environment {
	var list []Environment
	for _, kv := range os.Environ() {
		key, val, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(key, "ENV_") || !strings.HasSuffix(key, "_PROM_URL") {
			continue
		}
		envID := strings.TrimPrefix(strings.TrimSuffix(key, "_PROM_URL"), "ENV_")
		if envID == "" || val == "" {
			continue
		}
		name := os.Getenv("ENV_" + envID + "_NAME")
		if name == "" {
			name = envID
		}
		grafURL := os.Getenv("ENV_" + envID + "_GRAF_URL")
		if grafURL == "" {
			continue
		}
		list = append(list, Environment{
			ID:        envID,
			Name:      name,
			PromURL:   val,
			GrafURL:   grafURL,
			GrafToken: os.Getenv("ENV_" + envID + "_GRAF_TOKEN"),
			Cluster:   os.Getenv("ENV_" + envID + "_CLUSTER"),
		})
	}
	return list
}
