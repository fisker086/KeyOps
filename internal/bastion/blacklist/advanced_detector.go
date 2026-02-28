package blacklist

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strings"
)

// AdvancedDetector 高级命令检测器 - 防绕过
type AdvancedDetector struct {
	dangerousPatterns []*regexp.Regexp
	dangerousKeywords []string
}

// NewAdvancedDetector 创建高级检测器
func NewAdvancedDetector() *AdvancedDetector {
	return &AdvancedDetector{
		dangerousPatterns: compileDangerousPatterns(),
		dangerousKeywords: []string{
			"rm", "dd", "mkfs", "fdisk", "parted",
			"reboot", "shutdown", "halt", "poweroff",
			"iptables", "firewall", "ufw",
			"userdel", "usermod", "passwd",
			"chmod", "chown", "chgrp",
			"kill", "pkill", "killall",
		},
	}
}

// IsCommandDangerous 综合检测命令是否危险
func (d *AdvancedDetector) IsCommandDangerous(command string) (bool, string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return false, ""
	}

	// 1. 检测 Base64 编码绕过
	if dangerous, reason := d.detectBase64Bypass(command); dangerous {
		return true, reason
	}

	// 2. 检测十六进制编码绕过
	if dangerous, reason := d.detectHexBypass(command); dangerous {
		return true, reason
	}

	// 3. 检测转义字符绕过
	if dangerous, reason := d.detectEscapeBypass(command); dangerous {
		return true, reason
	}

	// 4. 检测变量替换绕过
	if dangerous, reason := d.detectVariableBypass(command); dangerous {
		return true, reason
	}

	// 5. 检测命令替换绕过
	if dangerous, reason := d.detectCommandSubstitution(command); dangerous {
		return true, reason
	}

	// 6. 检测通配符绕过
	if dangerous, reason := d.detectWildcardBypass(command); dangerous {
		return true, reason
	}

	// 7. 检测脚本执行绕过
	if dangerous, reason := d.detectScriptBypass(command); dangerous {
		return true, reason
	}

	// 8. 检测管道和重定向中的危险命令
	if dangerous, reason := d.detectPipelineBypass(command); dangerous {
		return true, reason
	}

	// 9. 标准模式匹配
	if dangerous, reason := d.detectStandardPatterns(command); dangerous {
		return true, reason
	}

	return false, ""
}

// detectBase64Bypass 检测 Base64 编码绕过
// 例如: echo "cm0gLXJmIC8=" | base64 -d | bash
func (d *AdvancedDetector) detectBase64Bypass(command string) (bool, string) {
	lowerCmd := strings.ToLower(command)

	// 检测 base64 解码命令
	if strings.Contains(lowerCmd, "base64") && strings.Contains(lowerCmd, "-d") {
		// 尝试提取并解码 base64 内容
		re := regexp.MustCompile(`["']([A-Za-z0-9+/=]{10,})["']`)
		matches := re.FindAllStringSubmatch(command, -1)
		for _, match := range matches {
			if len(match) > 1 {
				decoded, err := base64.StdEncoding.DecodeString(match[1])
				if err == nil {
					decodedCmd := string(decoded)
					// 递归检查解码后的命令
					if d.containsDangerousKeyword(decodedCmd) {
						return true, "检测到Base64编码的危险命令: " + decodedCmd
					}
				}
			}
		}

		// 即使无法解码，也标记为可疑
		if strings.Contains(lowerCmd, "bash") || strings.Contains(lowerCmd, "sh") {
			return true, "检测到可疑的Base64解码+执行组合"
		}
	}

	return false, ""
}

// detectHexBypass 检测十六进制编码绕过
// 例如: $(printf "\x72\x6d")
func (d *AdvancedDetector) detectHexBypass(command string) (bool, string) {
	// 检测十六进制字符串
	hexPattern := regexp.MustCompile(`\\x[0-9a-fA-F]{2}`)
	if hexPattern.MatchString(command) {
		// 尝试解码
		hexMatches := hexPattern.FindAllString(command, -1)
		var decoded strings.Builder
		for _, hexStr := range hexMatches {
			hexStr = strings.TrimPrefix(hexStr, "\\x")
			if b, err := hex.DecodeString(hexStr); err == nil && len(b) > 0 {
				decoded.WriteByte(b[0])
			}
		}

		if decoded.Len() > 0 {
			decodedCmd := decoded.String()
			if d.containsDangerousKeyword(decodedCmd) {
				return true, "检测到十六进制编码的危险命令: " + decodedCmd
			}
		}

		// 标记可疑的十六进制使用
		lowerCmd := strings.ToLower(command)
		if strings.Contains(lowerCmd, "printf") || strings.Contains(lowerCmd, "echo") {
			return true, "检测到可疑的十六进制编码命令"
		}
	}

	return false, ""
}

// detectEscapeBypass 检测转义字符绕过
// 例如: r\m -rf / 或 rm\ -rf\ /
func (d *AdvancedDetector) detectEscapeBypass(command string) (bool, string) {
	// 移除所有反斜杠和引号，然后检查
	cleaned := strings.ReplaceAll(command, "\\", "")
	cleaned = strings.ReplaceAll(cleaned, "'", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ToLower(cleaned)

	if d.containsDangerousKeyword(cleaned) {
		return true, "检测到使用转义字符绕过的危险命令"
	}

	return false, ""
}

// detectVariableBypass 检测变量替换绕过
// 例如: RM='rm'; $RM -rf /
func (d *AdvancedDetector) detectVariableBypass(command string) (bool, string) {
	lowerCmd := strings.ToLower(command)

	// 检测变量赋值和使用模式
	varPattern := regexp.MustCompile(`\$\{?[A-Za-z_][A-Za-z0-9_]*\}?`)
	if varPattern.MatchString(command) {
		// 检查是否包含危险命令的变量赋值
		for _, keyword := range d.dangerousKeywords {
			if strings.Contains(lowerCmd, "'"+keyword+"'") ||
				strings.Contains(lowerCmd, "\""+keyword+"\"") ||
				strings.Contains(lowerCmd, "="+keyword) {
				return true, "检测到使用变量赋值绕过的危险命令: " + keyword
			}
		}
	}

	return false, ""
}

// detectCommandSubstitution 检测命令替换绕过
// 例如: $(echo rm) -rf / 或 `echo rm` -rf /
func (d *AdvancedDetector) detectCommandSubstitution(command string) (bool, string) {
	lowerCmd := strings.ToLower(command)

	// 检测 $() 或 `` 命令替换
	hasSubstitution := strings.Contains(command, "$(") || strings.Contains(command, "`")

	if hasSubstitution {
		// 提取命令替换内容
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`\$\(([^)]+)\)`),
			regexp.MustCompile("`([^`]+)`"),
		}

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(command, -1)
			for _, match := range matches {
				if len(match) > 1 {
					subCmd := strings.ToLower(match[1])
					if d.containsDangerousKeyword(subCmd) {
						return true, "检测到命令替换中的危险命令: " + match[1]
					}
				}
			}
		}

		// 标记可疑的命令替换 + echo 组合
		if strings.Contains(lowerCmd, "echo") {
			for _, keyword := range d.dangerousKeywords {
				if strings.Contains(lowerCmd, keyword) {
					return true, "检测到可疑的命令替换绕过"
				}
			}
		}
	}

	return false, ""
}

// detectWildcardBypass 检测通配符绕过
// 例如: r?m -rf / 或 r[m] -rf /
func (d *AdvancedDetector) detectWildcardBypass(command string) (bool, string) {
	// 移除通配符，检查是否匹配危险命令
	cleaned := command
	cleaned = regexp.MustCompile(`\?`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`\[[^\]]+\]`).ReplaceAllString(cleaned, "m") // 替换为可能的字符
	cleaned = regexp.MustCompile(`\*`).ReplaceAllString(cleaned, "")
	cleaned = strings.ToLower(cleaned)

	if d.containsDangerousKeyword(cleaned) {
		return true, "检测到使用通配符绕过的危险命令"
	}

	return false, ""
}

// detectScriptBypass 检测脚本执行绕过
// 例如: echo "rm -rf /" > /tmp/x.sh && bash /tmp/x.sh
func (d *AdvancedDetector) detectScriptBypass(command string) (bool, string) {
	lowerCmd := strings.ToLower(command)

	// 检测写入脚本并执行的模式
	scriptPatterns := []string{
		"echo.*>.*\\.sh",
		"cat.*>.*\\.sh",
		"printf.*>.*\\.sh",
		"tee.*\\.sh",
	}

	for _, pattern := range scriptPatterns {
		if matched, _ := regexp.MatchString(pattern, lowerCmd); matched {
			// 检查是否后续执行
			if strings.Contains(lowerCmd, "bash") ||
				strings.Contains(lowerCmd, "sh ") ||
				strings.Contains(lowerCmd, "./") {
				// 检查脚本内容是否包含危险命令
				if d.containsDangerousKeyword(command) {
					return true, "检测到将危险命令写入脚本并执行"
				}
			}
		}
	}

	return false, ""
}

// detectPipelineBypass 检测管道和重定向中的危险命令
func (d *AdvancedDetector) detectPipelineBypass(command string) (bool, string) {
	// 分割管道命令
	parts := regexp.MustCompile(`[|;&&]+`).Split(command, -1)

	for _, part := range parts {
		cleaned := strings.TrimSpace(strings.ToLower(part))
		if d.containsDangerousKeyword(cleaned) {
			return true, "检测到管道或命令链中的危险命令"
		}
	}

	return false, ""
}

// detectStandardPatterns 标准危险模式检测
func (d *AdvancedDetector) detectStandardPatterns(command string) (bool, string) {
	lowerCmd := strings.ToLower(command)

	for _, pattern := range d.dangerousPatterns {
		if pattern.MatchString(lowerCmd) {
			return true, "命令匹配危险模式"
		}
	}

	return false, ""
}

// containsDangerousKeyword 检查是否包含危险关键词
func (d *AdvancedDetector) containsDangerousKeyword(command string) bool {
	lowerCmd := strings.ToLower(command)
	words := strings.Fields(lowerCmd)

	for _, word := range words {
		// 移除常见前缀
		word = strings.TrimLeft(word, "./")

		for _, keyword := range d.dangerousKeywords {
			if word == keyword || strings.HasPrefix(word, keyword+" ") {
				return true
			}
		}
	}

	return false
}

// compileDangerousPatterns 编译危险命令正则模式
func compileDangerousPatterns() []*regexp.Regexp {
	patterns := []string{
		`\brm\s+(-[rf]+\s+)?/`,                // rm -rf /
		`\bdd\s+.*of=/dev/(sd|hd|nvme)`,       // dd 覆盖磁盘
		`\bmkfs\.`,                            // 格式化文件系统
		`\b(shutdown|reboot|halt|poweroff)\b`, // 系统关机重启
		`>\s*/dev/(sd|hd|nvme)`,               // 重定向到磁盘设备
		`\bchmod\s+777`,                       // 危险的权限设置
		`\bchmod\s+.*\s+/`,                    // 修改根目录权限
		`\buserdel\s+root`,                    // 删除root用户
		`\bmv\s+.*\s+/dev/null`,               // 移动文件到黑洞
		`:\(\)\{\s*:\|:&\s*\};:`,              // Fork炸弹
		`\bcurl\s+.*\|\s*(bash|sh)`,           // curl | bash 危险模式
		`\bwget\s+.*\|\s*(bash|sh)`,           // wget | bash 危险模式
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}

	return compiled
}
