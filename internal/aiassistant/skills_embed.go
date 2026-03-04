package aiassistant

import (
	_ "embed"
)

//go:embed skills/k8s-install/SKILL.md
var k8sInstallSkillContent string

// GetK8sInstallSkillContent 返回 K8s 安装技能知识库内容
func GetK8sInstallSkillContent() string {
	return k8sInstallSkillContent
}

// GetSkillContent 按 skill_id 返回技能知识库内容，供专家 system_prompt 前置注入。数据库 skill_id 为空则无技能。
func GetSkillContent(skillID string) string {
	switch skillID {
	case SkillK8sInstall:
		return GetK8sInstallSkillContent()
	default:
		return ""
	}
}
