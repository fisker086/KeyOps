package terminal

import (
	"fmt"
	"io"
	"github.com/fisker086/keyops/pkg/logger"
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/sshserver/types"
	"golang.org/x/crypto/ssh"
)

const (
	// ANSI颜色代码
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// Menu 主机选择菜单
type Menu struct {
	selector types.HostSelector
	channel  ssh.Channel
}

// NewMenu 创建菜单
func NewMenu(selector types.HostSelector, channel ssh.Channel) *Menu {
	return &Menu{
		selector: selector,
		channel:  channel,
	}
}

// ShowWelcome 显示欢迎信息
func (m *Menu) ShowWelcome(username string) {
	// 清屏
	m.channel.Write([]byte("\033[2J\033[H"))

	banner := fmt.Sprintf("\r\n%s"+
		"================================================================\r\n"+
		"           Welcome to ZJump SSH Gateway\r\n"+
		"        Zero-Trust Jump Server with Full Audit\r\n"+
		"================================================================%s\r\n"+
		"\r\n"+
		"Hello, %s%s%s! This session is being recorded for security.\r\n",
		colorCyan+colorBold, colorReset,
		colorYellow+colorBold, username, colorReset)

	m.channel.Write([]byte(banner))
}

// SelectHost 选择主机
func (m *Menu) SelectHost(userID string) (*types.HostInfo, error) {
	// 获取可用主机列表
	logger.Infof("[Menu] Getting available hosts for user: %s", userID)
	hosts, err := m.selector.ListAvailableHosts(userID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to list hosts: %v", err)
		logger.Infof("[Menu] ERROR: %s", errMsg)
		m.channel.Write([]byte(fmt.Sprintf("\r\n%s %s%s\r\n", colorRed, errMsg, colorReset)))
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}

	logger.Infof("[Menu] Found %d hosts", len(hosts))

	if len(hosts) == 0 {
		logger.Infof("[Menu] No hosts available")
		m.channel.Write([]byte(fmt.Sprintf("\r\n%s No hosts available for your account.%s\r\n", colorRed, colorReset)))
		m.channel.Write([]byte(fmt.Sprintf("Please contact your administrator to add hosts.\r\n")))
		return nil, fmt.Errorf("no hosts available")
	}

	// 显示主机列表
	logger.Infof("[Menu] Showing host list...")
	m.showHostList(hosts)
	logger.Infof("[Menu] Host list displayed, waiting for user selection...")

	// 等待用户选择
	selectedHost := m.waitForSelection(hosts)
	if selectedHost == nil {
		logger.Infof("[Menu] No host selected (user cancelled or error)")
	} else {
		logger.Infof("[Menu] User selected host: %s (%s)", selectedHost.Name, selectedHost.IP)
	}
	return selectedHost, nil
}

// showHostList 显示主机列表
func (m *Menu) showHostList(hosts []types.HostInfo) {
	// 表头
	m.channel.Write([]byte(fmt.Sprintf("\r\n%s%-5s %-25s %-20s %-10s %-10s%s\r\n",
		colorCyan+colorBold, "No.", "Name", "IP Address", "Type", "Status", colorReset)))

	m.channel.Write([]byte(fmt.Sprintf("%s%s%s\r\n",
		colorCyan, strings.Repeat("-", 75), colorReset)))

	// 主机列表
	for i, host := range hosts {
		// 根据状态设置颜色
		statusColor := colorReset
		statusText := host.Status
		switch strings.ToLower(host.Status) {
		case "online":
			statusColor = colorGreen
			statusText = "●online"
		case "offline":
			statusColor = colorRed
			statusText = "●offline"
		default:
			statusColor = colorYellow
			statusText = "●" + host.Status
		}

		line := fmt.Sprintf("%s%-5d%s %-25s %-20s %-10s %s%-10s%s\r\n",
			colorYellow, i+1, colorReset,
			truncate(host.Name, 25),
			truncate(host.IP, 20),
			truncate(host.DeviceType, 10),
			statusColor, statusText, colorReset)

		m.channel.Write([]byte(line))
	}
}

// waitForSelection 等待用户选择 - 使用逐字节读取和实时回显
func (m *Menu) waitForSelection(hosts []types.HostInfo) *types.HostInfo {
	logger.Infof("[Menu] Starting to wait for user input...")

	// 输入缓冲区
	var inputBuffer []byte
	buf := make([]byte, 1) // 逐字节读取

	for {
		// 读取一个字节
		n, err := m.channel.Read(buf)
		if err != nil {
			if err == io.EOF {
				logger.Infof("[Menu] Channel closed by client")
			} else {
				logger.Infof("[Menu] Error reading input: %v", err)
			}
			return nil
		}

		if n == 0 {
			continue
		}

		ch := buf[0]

		switch ch {
		case '\r', '\n': // 回车
			logger.Infof("[Menu] Enter pressed, input buffer: %q", string(inputBuffer))

			// 发送回车换行
			m.channel.Write([]byte("\r\n"))

			// 处理输入
			input := strings.TrimSpace(string(inputBuffer))
			inputBuffer = inputBuffer[:0] // 清空缓冲区

			if input == "" {
				// 空输入，重新显示提示符
				m.channel.Write([]byte(fmt.Sprintf("%s>%s ", colorYellow+colorBold, colorReset)))
				continue
			}

			// 检查退出命令
			if strings.ToLower(input) == "q" || strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" {
				m.channel.Write([]byte(fmt.Sprintf("%sGoodbye!%s\r\n", colorGreen, colorReset)))
				return nil
			}

			// 解析选择
			selection, err := strconv.Atoi(input)
			if err != nil || selection < 1 || selection > len(hosts) {
				m.channel.Write([]byte(fmt.Sprintf("%sInvalid selection. Please enter a number between 1 and %d.%s\r\n%s>%s ",
					colorRed, len(hosts), colorReset, colorYellow+colorBold, colorReset)))
				continue
			}

			// 返回选择的主机
			selectedHost := &hosts[selection-1]
			m.channel.Write([]byte(fmt.Sprintf("%sConnecting to %s (%s)...%s\r\n\r\n",
				colorGreen, selectedHost.Name, selectedHost.IP, colorReset)))

			return selectedHost

		case 0x7f, 0x08: // Backspace (127) or Ctrl+H (8)
			if len(inputBuffer) > 0 {
				inputBuffer = inputBuffer[:len(inputBuffer)-1]
				// 发送退格序列：退格 + 空格 + 退格（擦除字符）
				m.channel.Write([]byte("\b \b"))
			}

		case 0x03: // Ctrl+C
			logger.Infof("[Menu] Ctrl+C received, exiting")
			m.channel.Write([]byte("\r\n^C\r\n"))
			return nil

		case 0x04: // Ctrl+D (EOF)
			logger.Infof("[Menu] Ctrl+D received, exiting")
			m.channel.Write([]byte("\r\n"))
			return nil

		case 0x1b: // ESC (方向键等ANSI序列的开始)
			// ESC序列通常后面还有1-2个字节，我们需要读取并忽略
			// 常见序列: ESC[A (上), ESC[B (下), ESC[C (右), ESC[D (左)
			// 简单策略：读取下一个字节，如果是'['，再读取一个
			nextBuf := make([]byte, 1)
			if n, _ := m.channel.Read(nextBuf); n > 0 && nextBuf[0] == '[' {
				// 读取方向键代码
				m.channel.Read(nextBuf)
			}
			// 忽略ESC序列

		default:
			// 可打印字符 (ASCII 32-126)
			if ch >= 32 && ch < 127 {
				inputBuffer = append(inputBuffer, ch)
				// 回显字符
				m.channel.Write([]byte{ch})
			}
		}
	}
}

// ShowError 显示错误信息
func (m *Menu) ShowError(errMsg string) {
	msg := fmt.Sprintf("\r\n%sError: %s%s\r\n", colorRed+colorBold, errMsg, colorReset)
	m.channel.Write([]byte(msg))
}

// ShowConnectionInfo 显示连接信息
func (m *Menu) ShowConnectionInfo(host *types.HostInfo) {
	// TODO: host.Username 已移除，需要从 SystemUser 获取
	username := host.Username
	if username == "" {
		username = "(system user)" // TODO: 显示实际的系统用户名
	}
	info := fmt.Sprintf("\r\n%sConnecting to %s (%s:%d) as %s...%s\r\n",
		colorGreen, host.Name, host.IP, host.Port, username, colorReset)

	m.channel.Write([]byte(info))
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// InteractiveMenu 交互式命令菜单 - 类似JumpServer
func (m *Menu) InteractiveMenu(userID string) (*types.HostInfo, bool) {
	// 获取主机列表
	hosts, err := m.selector.ListAvailableHosts(userID)
	if err != nil {
		m.ShowError(fmt.Sprintf("Failed to list hosts: %v", err))
		return nil, true // 发生错误，退出
	}

	if len(hosts) == 0 {
		m.ShowError("No hosts available for your account.")
		m.channel.Write([]byte("Please contact your administrator.\r\n"))
		return nil, true // 没有主机，退出
	}

	// 第一次显示：只显示命令帮助，不显示主机列表
	m.showCommandHelp()

	// 主命令循环
	for {
		// 显示提示符
		m.channel.Write([]byte(fmt.Sprintf("\r\n%sOpt>%s ", colorYellow+colorBold, colorReset)))

		// 读取命令
		command := m.readCommand()

		// 处理命令
		switch strings.ToLower(strings.TrimSpace(command)) {
		case "q", "quit", "exit":
			// 退出
			return nil, true

		case "p", "page", "list":
			// 刷新并显示主机列表
			hosts, err = m.selector.ListAvailableHosts(userID)
			if err != nil {
				m.ShowError(fmt.Sprintf("Failed to list hosts: %v", err))
				continue
			}
			m.showHostList(hosts)

		case "h", "help", "?":
			// 显示帮助
			m.showDetailedHelp()

		case "g", "group":
			// 显示主机组（暂时未实现）
			m.ShowMessage("\r\n" + colorYellow + "Host group feature coming soon..." + colorReset + "\r\n")

		case "":
			// 空命令，忽略
			continue

		default:
			// 尝试解析为数字（选择主机）
			choice, err := strconv.Atoi(strings.TrimSpace(command))
			if err == nil && choice >= 1 && choice <= len(hosts) {
				// 选择了主机
				selectedHost := &hosts[choice-1]
				return selectedHost, false
			}

			// 无效命令
			m.ShowError(fmt.Sprintf("Invalid: %s. Type 'h' for help", command))
		}
	}
}

// readCommand 读取用户命令
func (m *Menu) readCommand() string {
	var inputBuffer []byte
	buf := make([]byte, 1)

	for {
		n, err := m.channel.Read(buf)
		if err != nil {
			return ""
		}

		if n == 0 {
			continue
		}

		ch := buf[0]

		switch ch {
		case '\r', '\n': // 回车
			m.channel.Write([]byte("\r\n"))
			return string(inputBuffer)

		case 0x7f, 0x08: // Backspace
			if len(inputBuffer) > 0 {
				inputBuffer = inputBuffer[:len(inputBuffer)-1]
				m.channel.Write([]byte("\b \b"))
			}

		case 0x03: // Ctrl+C
			m.channel.Write([]byte("\r\n^C\r\n"))
			return "q"

		case 0x04: // Ctrl+D
			m.channel.Write([]byte("\r\n"))
			return "q"

		case 0x1b: // ESC
			nextBuf := make([]byte, 1)
			if n, _ := m.channel.Read(nextBuf); n > 0 && nextBuf[0] == '[' {
				m.channel.Read(nextBuf)
			}

		default:
			if ch >= 32 && ch < 127 {
				inputBuffer = append(inputBuffer, ch)
				m.channel.Write([]byte{ch})
			}
		}
	}
}

// showCommandHelp 显示命令帮助
func (m *Menu) showCommandHelp() {
	help := fmt.Sprintf("\r\n%sCommands:%s %s[Number]%s=Connect  %sp%s=Page  %sg%s=Group  %sh%s=Help  %sq%s=Quit\r\n",
		colorCyan+colorBold, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset)

	m.channel.Write([]byte(help))
}

// showDetailedHelp 显示详细帮助
func (m *Menu) showDetailedHelp() {
	help := fmt.Sprintf("\r\n%s=== ZJump Help ===%s\r\n"+
		"%sNavigation:%s\r\n"+
		"  [Number] - Connect to host by number\r\n"+
		"  p - Refresh and display host list (page)\r\n"+
		"  g - View hosts by group\r\n"+
		"  h/? - Show this help\r\n"+
		"  q - Quit and disconnect\r\n"+
		"\r\n"+
		"%sNote:%s After disconnecting, you'll return to this menu.\r\n"+
		"All sessions are recorded for security compliance.\r\n",
		colorCyan+colorBold, colorReset,
		colorCyan+colorBold, colorReset,
		colorCyan+colorBold, colorReset)

	m.channel.Write([]byte(help))
}

// ShowGoodbye 显示再见信息
func (m *Menu) ShowGoodbye() {
	goodbye := fmt.Sprintf("\r\n%sThank you for using ZJump SSH Gateway. Goodbye!%s\r\n",
		colorGreen+colorBold, colorReset)

	m.channel.Write([]byte(goodbye))
}

// ShowMessage 显示消息
func (m *Menu) ShowMessage(msg string) {
	m.channel.Write([]byte(msg))
}

// ShowReturnToMenu 显示返回菜单的消息和命令帮助
func (m *Menu) ShowReturnToMenu() {
	m.channel.Write([]byte(fmt.Sprintf("\r\n%sConnection closed. Returning to menu...%s\r\n", colorGreen, colorReset)))
	m.showCommandHelp()
}

// PromptPressToContinue 提示按任意键继续
func (m *Menu) PromptPressToContinue() {
	m.channel.Write([]byte(fmt.Sprintf("\r\n%sPress Enter to continue...%s ", colorCyan, colorReset)))

	// 等待用户按键
	buf := make([]byte, 1)
	for {
		n, err := m.channel.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if buf[0] == '\r' || buf[0] == '\n' {
			m.channel.Write([]byte("\r\n"))
			return
		}
	}
}
