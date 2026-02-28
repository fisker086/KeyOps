package rulefile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// PrometheusRuleFile 管理 Prometheus 规则文件
type PrometheusRuleFile struct {
	RuleDir string // 规则文件目录
}

// RuleGroupYaml Prometheus 规则组 YAML 结构
type RuleGroupYaml struct {
	Groups []RuleGroup `yaml:"groups"`
}

// RuleGroup 规则组
type RuleGroup struct {
	Name  string `yaml:"name"`
	Rules []Rule  `yaml:"rules"`
}

// Rule 告警规则
type Rule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr,omitempty"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// AlertRule 告警规则数据（从数据库模型转换）
type AlertRule struct {
	Name        string
	Expr        string
	Duration    int // 持续时间（秒）
	Labels      map[string]string
	Annotations map[string]string
}

// NewPrometheusRuleFile 创建 Prometheus 规则文件管理器
func NewPrometheusRuleFile(ruleDir string) *PrometheusRuleFile {
	return &PrometheusRuleFile{
		RuleDir: ruleDir,
	}
}

// formatDuration 格式化持续时间
// 如果持续时间能被60整除，使用分钟，否则使用秒
func formatDuration(duration int) string {
	if duration%60 == 0 {
		return fmt.Sprintf("%dm", duration/60)
	}
	return fmt.Sprintf("%ds", duration)
}

// CreateRuleFile 创建规则文件
func (p *PrometheusRuleFile) CreateRuleFile(filename string) error {
	// 确保文件名以 .rules 结尾
	if filepath.Ext(filename) != ".rules" {
		filename = filename + ".rules"
	}

	fullPath := filepath.Join(p.RuleDir, filename)

	// 检查文件是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("规则文件已存在: %s", fullPath)
	}

	// 确保目录存在
	if err := os.MkdirAll(p.RuleDir, 0755); err != nil {
		return fmt.Errorf("创建规则目录失败: %w", err)
	}

	// 创建空文件
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("创建规则文件失败: %w", err)
	}
	defer file.Close()

	// 写入空的规则组结构
	groupYaml := RuleGroupYaml{
		Groups: []RuleGroup{},
	}
	yamlData, err := yaml.Marshal(&groupYaml)
	if err != nil {
		return fmt.Errorf("序列化 YAML 失败: %w", err)
	}

	_, err = file.Write(yamlData)
	return err
}

// DeleteRuleFile 删除规则文件
func (p *PrometheusRuleFile) DeleteRuleFile(filename string) error {
	fullPath := filepath.Join(p.RuleDir, filename)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// 文件不存在，直接返回成功（幂等操作）
		return nil
	}

	// 备份文件
	backupPath := fullPath + ".bak." + fmt.Sprintf("%d", time.Now().Unix())
	if err := copyFile(fullPath, backupPath); err != nil {
		return fmt.Errorf("备份文件失败: %w", err)
	}

	// 删除文件
	return os.Remove(fullPath)
}

// AddRule 添加规则到规则文件
func (p *PrometheusRuleFile) AddRule(filename string, groupName string, rule AlertRule) error {
	fullPath := filepath.Join(p.RuleDir, filename)

	// 读取现有文件
	groupYaml, err := p.readRuleFile(fullPath)
	fileExists := true
	if err != nil {
		// 如果文件不存在，创建新的规则组结构
		if os.IsNotExist(err) {
			groupYaml = &RuleGroupYaml{
				Groups: []RuleGroup{},
			}
			fileExists = false
		} else {
			return err
		}
	}

	// 如果文件存在，备份文件
	if fileExists {
		backupPath := fullPath + ".bak." + fmt.Sprintf("%d", time.Now().Unix())
		if err := copyFile(fullPath, backupPath); err != nil {
			return fmt.Errorf("备份文件失败: %w", err)
		}
	}

	// 转换为 Prometheus Rule
	promRule := Rule{
		Alert:       rule.Name,
		Expr:        rule.Expr,
		For:         formatDuration(rule.Duration),
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
	}

	// 查找或创建规则组
	groupIndex := -1
	for i, group := range groupYaml.Groups {
		if group.Name == groupName {
			groupIndex = i
			break
		}
	}

	if groupIndex >= 0 {
		// 检查规则是否已存在（避免重复添加）
		for _, existingRule := range groupYaml.Groups[groupIndex].Rules {
			if existingRule.Alert == promRule.Alert {
				return fmt.Errorf("规则 %s 已存在于组 %s 中", promRule.Alert, groupName)
			}
		}
		// 添加到现有组
		groupYaml.Groups[groupIndex].Rules = append(groupYaml.Groups[groupIndex].Rules, promRule)
	} else {
		// 创建新组
		groupYaml.Groups = append(groupYaml.Groups, RuleGroup{
			Name:  groupName,
			Rules: []Rule{promRule},
		})
	}

	// 写回文件
	return p.writeRuleFile(fullPath, groupYaml)
}

// UpdateRule 更新规则文件中的规则
func (p *PrometheusRuleFile) UpdateRule(filename string, groupName string, oldRuleName string, rule AlertRule) error {
	fullPath := filepath.Join(p.RuleDir, filename)

	// 读取现有文件
	groupYaml, err := p.readRuleFile(fullPath)
	if err != nil {
		return err
	}

	// 备份文件
	backupPath := fullPath + ".bak." + fmt.Sprintf("%d", time.Now().Unix())
	if err := copyFile(fullPath, backupPath); err != nil {
		return fmt.Errorf("备份文件失败: %w", err)
	}

	// 转换为 Prometheus Rule
	promRule := Rule{
		Alert:       rule.Name,
		Expr:        rule.Expr,
		For:         formatDuration(rule.Duration),
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
	}

	// 查找并更新规则
	found := false
	for i, group := range groupYaml.Groups {
		if group.Name == groupName {
			for j, r := range group.Rules {
				if r.Alert == oldRuleName {
					groupYaml.Groups[i].Rules[j] = promRule
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	if !found {
		// 如果规则不存在，尝试添加新规则（可能是规则名称改变了，但旧规则不存在）
		// 这种情况下，直接添加新规则
		groupIndex := -1
		for i, group := range groupYaml.Groups {
			if group.Name == groupName {
				groupIndex = i
				break
			}
		}
		if groupIndex >= 0 {
			groupYaml.Groups[groupIndex].Rules = append(groupYaml.Groups[groupIndex].Rules, promRule)
		} else {
			groupYaml.Groups = append(groupYaml.Groups, RuleGroup{
				Name:  groupName,
				Rules: []Rule{promRule},
			})
		}
	}

	// 写回文件
	return p.writeRuleFile(fullPath, groupYaml)
}

// DeleteRule 从规则文件中删除规则
func (p *PrometheusRuleFile) DeleteRule(filename string, groupName string, ruleName string) error {
	fullPath := filepath.Join(p.RuleDir, filename)

	// 读取现有文件
	groupYaml, err := p.readRuleFile(fullPath)
	if err != nil {
		// 如果文件不存在，返回成功（幂等操作）
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// 备份文件
	backupPath := fullPath + ".bak." + fmt.Sprintf("%d", time.Now().Unix())
	if err := copyFile(fullPath, backupPath); err != nil {
		return fmt.Errorf("备份文件失败: %w", err)
	}

	// 查找并删除规则
	found := false
	for i, group := range groupYaml.Groups {
		if group.Name == groupName {
			newRules := []Rule{}
			for _, r := range group.Rules {
				if r.Alert != ruleName {
					newRules = append(newRules, r)
				} else {
					found = true
				}
			}
			groupYaml.Groups[i].Rules = newRules
			if found {
				break
			}
		}
	}

	if !found {
		// 规则不存在，返回成功（幂等操作）
		return nil
	}

	// 写回文件
	return p.writeRuleFile(fullPath, groupYaml)
}

// readRuleFile 读取规则文件
func (p *PrometheusRuleFile) readRuleFile(filePath string) (*RuleGroupYaml, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err // 返回原始错误，让调用者判断是否是文件不存在
	}

	var groupYaml RuleGroupYaml
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &groupYaml); err != nil {
			return nil, fmt.Errorf("解析 YAML 失败: %w", err)
		}
	}

	return &groupYaml, nil
}

// writeRuleFile 写入规则文件
func (p *PrometheusRuleFile) writeRuleFile(filePath string, groupYaml *RuleGroupYaml) error {
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	yamlData, err := yaml.Marshal(groupYaml)
	if err != nil {
		return fmt.Errorf("序列化 YAML 失败: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.Write(yamlData)
	return err
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0644)
}

// SyncRulesFromDB 从数据库同步所有规则到规则文件
// 这个方法可以根据数据源类型和规则组，将数据库中的规则同步到对应的规则文件
func (p *PrometheusRuleFile) SyncRulesFromDB(rules []RuleGroupData) error {
	// 按规则文件分组
	fileGroups := make(map[string][]RuleGroupData)
	for _, rule := range rules {
		key := rule.Filename
		fileGroups[key] = append(fileGroups[key], rule)
	}

	// 为每个规则文件生成完整的规则组结构
	for filename, rules := range fileGroups {
		groupYaml := RuleGroupYaml{
			Groups: []RuleGroup{},
		}

		// 按规则组分组
		groupMap := make(map[string][]Rule)
		for _, rule := range rules {
			promRule := Rule{
				Alert:       rule.Rule.Name,
				Expr:        rule.Rule.Expr,
				For:         formatDuration(rule.Rule.Duration),
				Labels:      rule.Rule.Labels,
				Annotations: rule.Rule.Annotations,
			}
			groupMap[rule.GroupName] = append(groupMap[rule.GroupName], promRule)
		}

		// 构建规则组
		for groupName, rules := range groupMap {
			groupYaml.Groups = append(groupYaml.Groups, RuleGroup{
				Name:  groupName,
				Rules: rules,
			})
		}

		// 写入文件
		fullPath := filepath.Join(p.RuleDir, filename)
		if err := p.writeRuleFile(fullPath, &groupYaml); err != nil {
			return fmt.Errorf("同步规则文件 %s 失败: %w", filename, err)
		}
	}

	return nil
}

// RuleGroupData 规则组数据（用于同步）
type RuleGroupData struct {
	Filename  string
	GroupName string
	Rule      AlertRule
}

