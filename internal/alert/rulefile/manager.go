package rulefile

import (
	"encoding/json"
	"fmt"

	"github.com/fisker086/keyops/internal/model"
)

// RuleFileManager 规则文件管理器
type RuleFileManager struct {
	ruleDir string // 规则文件目录
}

// NewRuleFileManager 创建规则文件管理器
func NewRuleFileManager(ruleDir string) *RuleFileManager {
	return &RuleFileManager{
		ruleDir: ruleDir,
	}
}

// SyncRuleToFile 同步规则到文件（规则目录为空时仅做 DB 记录，不写文件、不依赖挂载）
func (m *RuleFileManager) SyncRuleToFile(source *model.AlertRuleSource, group *model.AlertRuleGroup, rule *model.AlertRule, operation string) error {
	if m.ruleDir == "" {
		return nil
	}
	// 根据数据源类型选择不同的处理方式
	switch source.SourceType {
	case "prometheus", "thanos", "victoriametrics":
		return m.syncPrometheusRule(source, group, rule, operation)
	default:
		return fmt.Errorf("不支持的数据源类型: %s", source.SourceType)
	}
}

// syncPrometheusRule 同步 Prometheus 规则
func (m *RuleFileManager) syncPrometheusRule(source *model.AlertRuleSource, group *model.AlertRuleGroup, rule *model.AlertRule, operation string) error {
	// 确保规则文件路径存在
	if group.File == "" {
		// 如果没有指定文件，使用数据源名称生成文件名
		group.File = fmt.Sprintf("%s_%d.rules", source.SourceName, group.ID)
	}

	ruleFile := NewPrometheusRuleFile(m.ruleDir)

	// 解析 Labels 和 Annotations
	labels := make(map[string]string)
	annotations := make(map[string]string)

	if rule.Labels != nil {
		if err := json.Unmarshal(rule.Labels, &labels); err != nil {
			return fmt.Errorf("解析 Labels 失败: %w", err)
		}
	}

	if rule.Annotations != nil {
		if err := json.Unmarshal(rule.Annotations, &annotations); err != nil {
			return fmt.Errorf("解析 Annotations 失败: %w", err)
		}
	}

	alertRule := AlertRule{
		Name:        rule.Name,
		Expr:        rule.Expr,
		Duration:    rule.Duration,
		Labels:      labels,
		Annotations: annotations,
	}

	switch operation {
	case "create":
		// 如果文件不存在，先创建
		fullPath := fmt.Sprintf("%s/%s", m.ruleDir, group.File)
		if err := ruleFile.CreateRuleFile(group.File); err != nil {
			// 如果文件已存在，忽略错误，继续添加规则
			if err.Error() != fmt.Sprintf("规则文件已存在: %s", fullPath) {
				return err
			}
		}
		return ruleFile.AddRule(group.File, group.GroupName, alertRule)
	case "update":
		// update 操作应该通过 SyncRuleToFileWithOldName 调用
		return fmt.Errorf("update 操作应该使用 SyncRuleToFileWithOldName")
	case "delete":
		return ruleFile.DeleteRule(group.File, group.GroupName, rule.Name)
	default:
		return fmt.Errorf("不支持的操作: %s", operation)
	}
}

// SyncRuleGroupToFile 同步规则组到文件（规则目录为空时跳过，仅做 DB 记录）
func (m *RuleFileManager) SyncRuleGroupToFile(source *model.AlertRuleSource, group *model.AlertRuleGroup, operation string) error {
	if m.ruleDir == "" {
		return nil
	}
	switch source.SourceType {
	case "prometheus", "thanos", "victoriametrics":
		return m.syncPrometheusRuleGroup(source, group, operation)
	default:
		return fmt.Errorf("不支持的数据源类型: %s", source.SourceType)
	}
}

// syncPrometheusRuleGroup 同步 Prometheus 规则组
func (m *RuleFileManager) syncPrometheusRuleGroup(source *model.AlertRuleSource, group *model.AlertRuleGroup, operation string) error {
	ruleFile := NewPrometheusRuleFile(m.ruleDir)

	// 确保规则文件路径存在
	if group.File == "" {
		group.File = fmt.Sprintf("%s_%d.rules", source.SourceName, group.ID)
	}

	switch operation {
	case "create":
		return ruleFile.CreateRuleFile(group.File)
	case "delete":
		return ruleFile.DeleteRuleFile(group.File)
	default:
		return fmt.Errorf("不支持的操作: %s", operation)
	}
}

// SyncRuleToFileWithOldName 同步规则到文件（规则目录为空时跳过）
func (m *RuleFileManager) SyncRuleToFileWithOldName(source *model.AlertRuleSource, group *model.AlertRuleGroup, rule *model.AlertRule, oldRuleName string, operation string) error {
	if m.ruleDir == "" {
		return nil
	}
	// 根据数据源类型选择不同的处理方式
	switch source.SourceType {
	case "prometheus", "thanos", "victoriametrics":
		return m.syncPrometheusRuleWithOldName(source, group, rule, oldRuleName, operation)
	default:
		return fmt.Errorf("不支持的数据源类型: %s", source.SourceType)
	}
}

// syncPrometheusRuleWithOldName 同步 Prometheus 规则（带旧名称）
func (m *RuleFileManager) syncPrometheusRuleWithOldName(source *model.AlertRuleSource, group *model.AlertRuleGroup, rule *model.AlertRule, oldRuleName string, operation string) error {
	// 确保规则文件路径存在
	if group.File == "" {
		group.File = fmt.Sprintf("%s_%d.rules", source.SourceName, group.ID)
	}

	ruleFile := NewPrometheusRuleFile(m.ruleDir)

	// 解析 Labels 和 Annotations
	labels := make(map[string]string)
	annotations := make(map[string]string)

	if rule.Labels != nil {
		if err := json.Unmarshal(rule.Labels, &labels); err != nil {
			return fmt.Errorf("解析 Labels 失败: %w", err)
		}
	}

	if rule.Annotations != nil {
		if err := json.Unmarshal(rule.Annotations, &annotations); err != nil {
			return fmt.Errorf("解析 Annotations 失败: %w", err)
		}
	}

	alertRule := AlertRule{
		Name:        rule.Name,
		Expr:        rule.Expr,
		Duration:    rule.Duration,
		Labels:      labels,
		Annotations: annotations,
	}

	if operation == "update" {
		return ruleFile.UpdateRule(group.File, group.GroupName, oldRuleName, alertRule)
	}

	return fmt.Errorf("不支持的操作: %s", operation)
}

// ReloadDatasource 重新加载数据源配置（规则目录为空时不调用 Prometheus reload）
func (m *RuleFileManager) ReloadDatasource(source *model.AlertRuleSource) error {
	if m.ruleDir == "" {
		return nil
	}
	client, err := NewDatasourceClient(source.SourceType, source.Address)
	if err != nil {
		return err
	}
	return client.Reload()
}

