package parser

import (
	"testing"
)

// TestIsPromptPattern 测试提示符模式识别
func TestIsPromptPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 应该识别为提示符的情况
		{"Docker容器提示符1", "root@6eefe58e10e0:", true},
		{"Docker容器提示符2", "root@container123:~", true},
		{"Docker容器提示符3", "root@abc123def456:/root#", true},
		{"普通提示符1", "user@hostname:~$", true},
		{"普通提示符2", "admin@server:/home/admin#", true},
		{"方括号提示符", "[root@localhost ~]#", true},
		{"简单提示符", "root@localhost:$", true},

		// 不应该识别为提示符的情况（正常命令）
		{"正常命令1", "ls -la", false},
		{"正常命令2", "cd /home/user", false},
		{"正常命令3", "pwd", false},
		{"正常命令4", "whoami", false},
		{"包含@的邮箱", "echo test@example.com", false},
		{"没有@符号", "echo hello:", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPromptPattern(tt.input)
			if result != tt.expected {
				t.Errorf("isPromptPattern(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestLooksLikeCommand 测试命令识别
func TestLooksLikeCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 应该识别为命令的情况
		{"ls命令", "ls -la", true},
		{"cd命令", "cd /home", true},
		{"grep命令", "grep pattern file.txt", true},
		{"自定义脚本", "./my-script.sh", true},

		// 不应该识别为命令的情况（提示符）
		{"Docker提示符1", "root@6eefe58e10e0:", false},
		{"Docker提示符2", "root@container123:~", false},
		{"Docker提示符3", "root@abc123def456:/root#", false},
		{"普通提示符", "user@hostname:~$", false},

		// 不应该识别为命令的情况（输出）
		{"空行", "", false},
		{"路径输出", "/home/user", false},
		{"带空格的提示符片段", "root@host: ", false}, // 去掉空格后是提示符
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeCommand(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikeCommand(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestShouldIgnoreSimple 测试命令忽略逻辑
func TestShouldIgnoreSimple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 应该忽略的命令
		{"exit命令", "exit", true},
		{"clear命令", "clear", true},
		{"reset命令", "reset", true},
		{"空命令", "", true},
		{"Docker提示符", "root@6eefe58e10e0:", true},
		{"普通提示符", "user@host:~$", true},
		{"Ctrl+C", "^C", true},

		// 不应该忽略的命令（需要记录）
		{"ls命令", "ls -la", false},
		{"rm命令", "rm -rf /tmp/test", false},
		{"cat命令", "cat /etc/passwd", false},
		{"自定义命令", "./important-script.sh", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldIgnoreSimple(tt.input)
			if result != tt.expected {
				t.Errorf("shouldIgnoreSimple(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCommandExtractorWithPrompts 测试命令提取器处理提示符
func TestCommandExtractorWithPrompts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // 期望记录的命令
	}{
		{
			name:     "正常命令序列",
			input:    "root@host:~# ls -la\r\ntotal 8\r\nroot@host:~# pwd\r\n/root\r\nroot@host:~# ",
			expected: []string{"ls -la", "pwd"},
		},
		{
			name:     "Ctrl+L清屏场景",
			input:    "root@6eefe58e10e0:# ls\r\nfile1 file2\r\nroot@6eefe58e10e0:# \x1b[H\x1b[2Jroot@6eefe58e10e0:# pwd\r\n/root\r\n",
			expected: []string{"ls", "pwd"},
		},
		{
			name:     "只有提示符无命令",
			input:    "root@host:~# \r\nroot@host:~# \r\n",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := []string{}
			extractor := NewCommandExtractor(func(cmd string) {
				commands = append(commands, cmd)
			})

			extractor.Feed(tt.input)

			if len(commands) != len(tt.expected) {
				t.Errorf("Expected %d commands, got %d: %v", len(tt.expected), len(commands), commands)
				return
			}

			for i, cmd := range commands {
				if cmd != tt.expected[i] {
					t.Errorf("Command %d: expected %q, got %q", i, tt.expected[i], cmd)
				}
			}
		})
	}
}

// TestExtractCommandFromLine 测试从行中提取命令
func TestExtractCommandFromLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"提示符后有命令", "root@host:~# ls -la", "ls -la"},
		{"提示符后有命令2", "user@server:/home$ pwd", "pwd"},
		{"方括号提示符后有命令", "[root@localhost ~]# whoami", "whoami"},
		{"只有提示符1", "root@host:~#", ""},
		{"只有提示符2", "user@server:~$", ""},
		{"只有提示符3", "root@6eefe58e10e0:#", ""},
		{"提示符后只有空格", "root@host:~# ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCommandFromLine(tt.input)
			if result != tt.expected {
				t.Errorf("extractCommandFromLine(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
