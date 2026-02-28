package aiassistant

import (
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// LLMConfig LLM 配置（API Key、Base URL、Model 等）
type LLMConfig struct {
	APIKey   string
	BaseURL  string
	Model    string
	MaxSteps int
	ProxyURL string
}

// ResolveLLMConfig 解析 LLM 配置：从数据库 settings（ai_assistant 分类）读取；无则返回 nil。
func ResolveLLMConfig(repo *repository.SettingRepository) (*LLMConfig, error) {
	cfg := &LLMConfig{}
	if repo != nil {
		settings, err := repo.GetByCategory(model.CategoryAiAssistant)
		if err == nil && len(settings) > 0 {
			m := make(map[string]string)
			for _, s := range settings {
				key := s.Key
				if len(s.Category) > 0 && len(key) > len(s.Category)+1 && key[:len(s.Category)+1] == s.Category+"." {
					key = key[len(s.Category)+1:]
				}
				m[key] = s.Value
			}
			apiKey := m["api_key"]
			if apiKey != "" {
				cfg.APIKey = apiKey
				cfg.BaseURL = m["base_url"]
				cfg.Model = m["model"]
				if cfg.Model == "" {
					cfg.Model = "qwen-max"
				}
				if v := m["max_steps"]; v != "" {
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						cfg.MaxSteps = n
					}
				}
				if cfg.MaxSteps <= 0 {
					cfg.MaxSteps = 30
				}
				cfg.ProxyURL = m["proxy_url"]
				return cfg, nil
			}
		}
	}
	return nil, nil
}

// GetAvailableModels 获取可选模型列表，供前端下拉选择。从 settings 的 available_models 读取，逗号分隔；空则用 model 作为唯一选项。
func GetAvailableModels(repo *repository.SettingRepository) (models []string, defaultModel string) {
	llm, _ := ResolveLLMConfig(repo)
	if llm == nil {
		return nil, ""
	}
	defaultModel = llm.Model
	if defaultModel == "" {
		defaultModel = "qwen-max"
	}
	if repo != nil {
		settings, err := repo.GetByCategory(model.CategoryAiAssistant)
		if err == nil {
			var availableModels string
			for _, s := range settings {
				key := s.Key
				if len(s.Category) > 0 && len(key) > len(s.Category)+1 && key[:len(s.Category)+1] == s.Category+"." {
					key = key[len(s.Category)+1:]
				}
				if key == "available_models" && s.Value != "" {
					availableModels = s.Value
					break
				}
			}
			if availableModels != "" {
				parts := splitAndTrim(availableModels, ",")
				if len(parts) > 0 {
					models = parts
					// 确保 default 在列表中
					hasDefault := false
					for _, p := range parts {
						if p == defaultModel {
							hasDefault = true
							break
						}
					}
					if !hasDefault {
						defaultModel = parts[0]
					}
					return models, defaultModel
				}
			}
		}
	}
	// 无 available_models 时，用 model 作为唯一选项
	return []string{defaultModel}, defaultModel
}

func splitAndTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
