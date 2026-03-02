package aiassistant

import (
	"strings"

	"github.com/fisker086/keyops/internal/aiassistant/tools"
	"github.com/fisker086/keyops/internal/aiassistant/tools/builtin"
	"github.com/fisker086/keyops/internal/aiassistant/tools/k8s"
)

// SkillID 技能标识，与目标环境配置对应
const (
	SkillPrometheus = "prometheus"
	SkillGrafana    = "grafana"
	SkillK8s        = "k8s"
)

// GetEnabledSkills 根据环境配置返回已启用的技能列表，供 UI 展示
func GetEnabledSkills(env *Environment, runners map[string]tools.Runner) []string {
	if env == nil {
		return nil
	}
	var skills []string
	if env.PromURL != "" {
		skills = append(skills, SkillPrometheus)
	}
	if env.GrafURL != "" {
		skills = append(skills, SkillGrafana)
	}
	if env.K8sClusterID != "" {
		if r := runners[k8s.ID]; r != nil {
			skills = append(skills, SkillK8s)
		}
	}
	return skills
}

// GetAvailableToolsPrompt 根据环境配置返回「当前可用技能」的提示词片段，仅包含已配置的工具
func GetAvailableToolsPrompt(env *Environment, runners map[string]tools.Runner) string {
	if env == nil {
		return ""
	}
	var parts []string
	if env.PromURL != "" {
		parts = append(parts, strings.TrimSpace(builtin.PromptFragmentPrometheus))
	}
	if env.GrafURL != "" {
		parts = append(parts, strings.TrimSpace(builtin.PromptFragmentGrafana))
	}
	if env.K8sClusterID != "" {
		if r := runners[k8s.ID]; r != nil {
			parts = append(parts, strings.TrimSpace(tools.GetPromptFragment(k8s.ID)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n## 当前环境可用技能（单 Agent 多 Skills，仅使用以下工具）：\n" +
		strings.Join(parts, "\n\n") +
		"\n\n**重要**：仅可使用上述列出的工具，未列出的工具表示当前环境未配置，请勿调用。"
}
