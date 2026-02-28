package terminal

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/sshserver/types"
	"golang.org/x/crypto/ssh"
)

// MenuV2 新版菜单系统 - 支持分组管理
type MenuV2 struct {
	selector types.HostSelector
	channel  ssh.Channel

	// 当前状态
	currentGroups []types.HostGroupInfo
	currentHosts  []types.HostInfo
	currentGroup  *types.HostGroupInfo
	currentPage   int
	pageSize      int
}

// NewMenuV2 创建新版菜单
func NewMenuV2(selector types.HostSelector, channel ssh.Channel) *MenuV2 {
	return &MenuV2{
		selector:    selector,
		channel:     channel,
		currentPage: 1,
		pageSize:    20, // 每页20台主机
	}
}

// ShowWelcome 显示欢迎信息
func (m *MenuV2) ShowWelcome(username string) {
	welcome := "\r\n\r\n"

	// 简洁的顶部分隔
	welcome += colorCyan + "  " + strings.Repeat("━", 70) + colorReset + "\r\n\r\n"

	// 主标题 - 使用渐变效果
	welcome += "     " + colorGreen + colorBold + "███████╗ " + colorReset + colorGreen + "     ██╗██╗   ██╗███╗   ███╗██████╗ " + colorReset + "\r\n"
	welcome += "     " + colorGreen + colorBold + "╚══███╔╝ " + colorReset + colorGreen + "     ██║██║   ██║████╗ ████║██╔══██╗" + colorReset + "\r\n"
	welcome += "       " + colorGreen + colorBold + "███╔╝  " + colorReset + colorGreen + "    ██║██║   ██║██╔████╔██║██████╔╝" + colorReset + "\r\n"
	welcome += "      " + colorGreen + colorBold + "███╔╝   " + colorReset + colorGreen + "██   ██║██║   ██║██║╚██╔╝██║██╔═══╝ " + colorReset + "\r\n"
	welcome += "     " + colorGreen + colorBold + "███████╗ " + colorReset + colorGreen + "╚█████╔╝╚██████╔╝██║ ╚═╝ ██║██║     " + colorReset + "\r\n"
	welcome += "     " + colorCyan + "╚══════╝  ╚════╝  ╚═════╝ ╚═╝     ╚═╝╚═╝     " + colorReset + "\r\n\r\n"

	// 副标题
	welcome += "              " + colorCyan + "🛡️  Secure SSH Gateway & Bastion Host" + colorReset + "\r\n\r\n"

	// 用户信息区域
	welcome += "  " + colorCyan + strings.Repeat("─", 70) + colorReset + "\r\n"
	welcome += "   " + colorWhite + "Welcome, " + colorYellow + colorBold + username + colorReset + "\r\n"
	welcome += "   " + colorWhite + "🔒 All operations are monitored and audited" + colorReset + "\r\n"
	welcome += "  " + colorCyan + strings.Repeat("─", 70) + colorReset + "\r\n\r\n"

	m.channel.Write([]byte(welcome))
}

// ShowGoodbye 显示再见信息
func (m *MenuV2) ShowGoodbye() {
	goodbye := "\r\n"
	goodbye += colorCyan + colorBold + "╔══════════════════════════════════════════════════════════════╗\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║                                                              ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║              " + colorGreen + "✓ Session Completed Successfully" + colorCyan + "                ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║                                                              ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║          " + colorYellow + "Thank you for using ZJump Gateway" + colorCyan + "           ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║              " + colorWhite + "Stay secure, stay connected! " + colorCyan + "             ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "║                                                              ║\r\n" + colorReset
	goodbye += colorCyan + colorBold + "╚══════════════════════════════════════════════════════════════╝\r\n" + colorReset
	m.channel.Write([]byte(goodbye))
}

// InteractiveMenuV2 交互式分组菜单
func (m *MenuV2) InteractiveMenuV2(userID string) (*types.HostInfo, bool) {
	// 加载分组列表（根据用户权限）
	if err := m.loadGroupsForUser(userID); err != nil {
		m.showError(fmt.Sprintf("Failed to load groups: %v", err))
		// 如果没有分组系统，降级到显示所有主机
		return m.fallbackToAllHosts(userID)
	}

	// 如果没有分组，降级处理
	if len(m.currentGroups) == 0 {
		m.showMessage("\r\n" + colorYellow + "No groups available for your account. Contact administrator for access." + colorReset + "\r\n")
		return m.fallbackToAllHosts(userID)
	}

	// 显示命令帮助
	m.showMainHelp()

	// 主命令循环
	for {
		// 显示提示符
		m.channel.Write([]byte(fmt.Sprintf("\r\n%sOpt>%s ", colorYellow+colorBold, colorReset)))

		// 读取命令
		command := m.readCommand()
		cmd := strings.ToLower(strings.TrimSpace(command))

		// 处理命令
		switch cmd {
		case "q", "quit", "exit":
			return nil, true

		case "g", "group":
			// 显示分组列表并进入分组选择
			if selectedHost := m.handleGroupSelection(); selectedHost != nil {
				return selectedHost, false
			}

		case "p", "page", "list":
			// 显示第一个分组的主机（默认分组）
			if len(m.currentGroups) > 0 {
				defaultGroup := &m.currentGroups[0]
				m.currentGroup = defaultGroup
				if selectedHost := m.handleHostSelection(defaultGroup); selectedHost != nil {
					return selectedHost, false
				}
			}

		case "h", "help", "?":
			m.showDetailedHelp()

		case "":
			continue

		default:
			// 尝试解析为数字
			if choice, err := strconv.Atoi(cmd); err == nil {
				// 如果当前有主机列表，认为是选择主机
				if len(m.currentHosts) > 0 && choice >= 1 && choice <= len(m.currentHosts) {
					return &m.currentHosts[choice-1], false
				}
			}
			m.showError(fmt.Sprintf("Invalid command: %s. Type 'h' for help", command))
		}
	}
}

// handleGroupSelection 处理分组选择
func (m *MenuV2) handleGroupSelection() *types.HostInfo {
	// 显示分组列表
	m.showGroupList()

	for {
		m.channel.Write([]byte(fmt.Sprintf("\r\n%sSelect Group>%s ", colorCyan+colorBold, colorReset)))
		command := m.readCommand()
		cmd := strings.ToLower(strings.TrimSpace(command))

		if cmd == "b" || cmd == "back" {
			return nil // 返回上级菜单
		}

		// 解析分组编号
		choice, err := strconv.Atoi(cmd)
		if err == nil && choice >= 1 && choice <= len(m.currentGroups) {
			selectedGroup := &m.currentGroups[choice-1]
			m.currentGroup = selectedGroup

			// 进入主机选择
			if selectedHost := m.handleHostSelection(selectedGroup); selectedHost != nil {
				return selectedHost
			}

			// 用户返回了，重新显示分组列表
			m.showGroupList()
		} else {
			m.showError(fmt.Sprintf("Invalid group number. Please enter 1-%d or 'b' to back", len(m.currentGroups)))
		}
	}
}

// handleHostSelection 处理主机选择（支持分页）
func (m *MenuV2) handleHostSelection(group *types.HostGroupInfo) *types.HostInfo {
	m.currentPage = 1

	// 加载第一页主机
	if err := m.loadHostsInGroup(group.ID, m.currentPage); err != nil {
		m.showError(fmt.Sprintf("Failed to load hosts: %v", err))
		return nil
	}

	// 显示主机列表
	m.showHostList(group)

	for {
		m.channel.Write([]byte(fmt.Sprintf("\r\n%sSelect Host>%s ", colorGreen+colorBold, colorReset)))
		command := m.readCommand()
		cmd := strings.ToLower(strings.TrimSpace(command))

		switch cmd {
		case "b", "back", "g", "group":
			return nil // 返回上级菜单（分组选择）

		case "q", "quit", "exit":
			// 退出（返回到主菜单）
			m.showMessage("Returning to main menu...")
			return nil

		case "h", "help", "?":
			// 显示帮助信息
			m.showHostSelectionHelp()
			m.showHostList(group) // 重新显示主机列表

		case "r", "refresh":
			// 刷新当前页
			if err := m.loadHostsInGroup(group.ID, m.currentPage); err == nil {
				m.showHostList(group)
				m.showMessage("Host list refreshed")
			} else {
				m.showError(fmt.Sprintf("Failed to refresh: %v", err))
			}

		case "n", "next":
			// 下一页
			m.currentPage++
			if err := m.loadHostsInGroup(group.ID, m.currentPage); err != nil || len(m.currentHosts) == 0 {
				m.showMessage("Already at last page, showing last page:")
				m.currentPage--
				if err := m.loadHostsInGroup(group.ID, m.currentPage); err == nil {
					m.showHostList(group)
				}
			} else {
				m.showHostList(group)
			}

		case "p", "prev":
			// 上一页
			if m.currentPage > 1 {
				m.currentPage--
				if err := m.loadHostsInGroup(group.ID, m.currentPage); err == nil {
					m.showHostList(group)
				}
			} else {
				// 已经在第一页，重新显示第一页内容
				m.showMessage("Already at first page, showing first page:")
				if err := m.loadHostsInGroup(group.ID, 1); err == nil {
					m.showHostList(group)
				}
			}

		case "":
			// 空输入，重新显示提示
			continue

		default:
			// 解析主机编号
			choice, err := strconv.Atoi(cmd)
			if err == nil && choice >= 1 && choice <= len(m.currentHosts) {
				return &m.currentHosts[choice-1]
			}
			m.showError(fmt.Sprintf("Invalid input '%s'. Enter: [1-%d]=Connect, n=Next, p=Prev, b=Back, h=Help, q=Quit", cmd, len(m.currentHosts)))
		}
	}
}

// loadGroups 加载分组列表
func (m *MenuV2) loadGroups() error {
	// 尝试获取分组列表
	selector, ok := m.selector.(*HostSelector)
	if !ok {
		return fmt.Errorf("selector does not support groups")
	}

	groups, err := selector.ListGroups()
	if err != nil {
		return err
	}

	m.currentGroups = groups
	logger.Infof("[MenuV2] Loaded %d groups", len(groups))
	return nil
}

// loadGroupsForUser 根据用户权限加载分组列表
func (m *MenuV2) loadGroupsForUser(userID string) error {
	// 尝试获取分组列表
	selector, ok := m.selector.(*HostSelector)
	if !ok {
		return fmt.Errorf("selector does not support groups")
	}

	// 尝试使用新的权限过滤方法
	groups, err := selector.ListGroupsForUser(userID)
	if err != nil {
		// 如果方法不支持，降级为显示所有分组
		logger.Infof("[MenuV2] ListGroupsForUser failed: %v, falling back to ListGroups", err)
		groups, err = selector.ListGroups()
		if err != nil {
			return err
		}
	}

	m.currentGroups = groups
	logger.Infof("[MenuV2] Loaded %d groups for user %s", len(groups), userID)
	return nil
}

// loadHostsInGroup 加载分组中的主机（分页）
func (m *MenuV2) loadHostsInGroup(groupID string, page int) error {
	selector, ok := m.selector.(*HostSelector)
	if !ok {
		return fmt.Errorf("selector does not support groups")
	}

	hosts, total, err := selector.ListHostsByGroup(groupID, page, m.pageSize)
	if err != nil {
		return err
	}

	m.currentHosts = hosts
	logger.Infof("[MenuV2] Loaded page %d: %d hosts (total: %d)", page, len(hosts), total)
	return nil
}

// showGroupList 显示分组列表
func (m *MenuV2) showGroupList() {
	output := "\r\n"
	output += colorCyan + colorBold + "╔══════════════════════════════════════════════════════════════╗\r\n" + colorReset
	output += colorCyan + colorBold + "║                    " + colorYellow + "🗂  HOST GROUPS" + colorCyan + "                          ║\r\n" + colorReset
	output += colorCyan + colorBold + "╠═════╤══════════════════════════════╤═══════════════════════════╣\r\n" + colorReset
	output += colorCyan + colorBold + "║ " + colorYellow + "No." + colorCyan + " │ " + colorYellow + "Group Name" + colorCyan + strings.Repeat(" ", 18) + " │ " + colorYellow + "Hosts Status" + colorCyan + "          ║\r\n" + colorReset
	output += colorCyan + colorBold + "╠═════╪══════════════════════════════╪═══════════════════════════╣\r\n" + colorReset

	for i, group := range m.currentGroups {
		groupName := truncate(group.Name, 28)
		statusInfo := fmt.Sprintf("%d Total | %s%d Online%s",
			group.HostCount,
			colorGreen, group.OnlineCount, colorCyan)

		// 计算对齐空格
		nameSpace := 28 - len(truncate(group.Name, 28))
		statusSpace := 25 - len(fmt.Sprintf("%d Total | %d Online", group.HostCount, group.OnlineCount))

		line := colorCyan + colorBold + "║ " + colorYellow + fmt.Sprintf("%3d", i+1) + colorCyan + " │ " + colorWhite + groupName + strings.Repeat(" ", nameSpace) + colorCyan + " │ " + statusInfo + strings.Repeat(" ", statusSpace) + " ║\r\n" + colorReset
		output += line
	}

	output += colorCyan + colorBold + "╚═════╧══════════════════════════════╧═══════════════════════════╝\r\n" + colorReset
	output += colorCyan + "   " + colorWhite + "Enter number to select, " + colorGreen + "'b'" + colorWhite + " to back\r\n" + colorReset

	m.channel.Write([]byte(output))
}

// showHostList 显示主机列表
func (m *MenuV2) showHostList(group *types.HostGroupInfo) {
	output := "\r\n"
	output += colorCyan + colorBold + "╔════════════════════════════════════════════════════════════════════════════════╗\r\n" + colorReset
	output += colorCyan + colorBold + "║  " + colorYellow + "📋 " + group.Name + colorCyan + strings.Repeat(" ", 66-len(group.Name)) + " [Page " + fmt.Sprintf("%d", m.currentPage) + "] ║\r\n" + colorReset
	output += colorCyan + colorBold + "╠═════╤═══════════════════════════╤════════════════════╤══════════╤═══════════╣\r\n" + colorReset
	output += colorCyan + colorBold + "║ " + colorYellow + "No." + colorCyan + " │ " + colorYellow + "Hostname" + colorCyan + strings.Repeat(" ", 18) + " │ " + colorYellow + "IP Address" + colorCyan + "       │ " + colorYellow + "Type" + colorCyan + "     │ " + colorYellow + "Status" + colorCyan + "    ║\r\n" + colorReset
	output += colorCyan + colorBold + "╠═════╪═══════════════════════════╪════════════════════╪══════════╪═══════════╣\r\n" + colorReset

	for i, host := range m.currentHosts {
		// 状态显示
		statusColor := colorReset
		statusText := host.Status
		switch strings.ToLower(host.Status) {
		case "online":
			statusColor = colorGreen
			statusText = "Online"
		case "offline":
			statusColor = colorRed
			statusText = "Offline"
		default:
			statusColor = colorYellow
			statusText = host.Status
		}

		hostname := truncate(host.Name, 25)
		ipAddr := truncate(host.IP, 18)
		deviceType := truncate(host.DeviceType, 8)

		// 计算对齐空格
		hostSpace := 25 - len(hostname)
		ipSpace := 18 - len(ipAddr)
		typeSpace := 8 - len(deviceType)
		statusSpace := 9 - len(statusText)

		line := colorCyan + colorBold + "║ " + colorYellow + fmt.Sprintf("%3d", i+1) + colorCyan + " │ " + colorWhite + hostname + strings.Repeat(" ", hostSpace) + colorCyan + " │ " + colorWhite + ipAddr + strings.Repeat(" ", ipSpace) + colorCyan + " │ " + colorWhite + deviceType + strings.Repeat(" ", typeSpace) + colorCyan + " │ " + statusColor + statusText + strings.Repeat(" ", statusSpace) + colorCyan + " ║\r\n" + colorReset
		output += line
	}

	output += colorCyan + colorBold + "╚═════╧═══════════════════════════╧════════════════════╧══════════╧═══════════╝\r\n" + colorReset
	output += colorCyan + "   " + colorWhite + "Commands: " + colorGreen + "[Number]" + colorWhite + "=Connect  " + colorGreen + "n" + colorWhite + "=Next  " + colorGreen + "p" + colorWhite + "=Prev  " + colorGreen + "b" + colorWhite + "=Back  " + colorGreen + "h" + colorWhite + "=Help  " + colorGreen + "q" + colorWhite + "=Quit\r\n" + colorReset

	m.channel.Write([]byte(output))
}

// showHostSelectionHelp 显示主机选择帮助
func (m *MenuV2) showHostSelectionHelp() {
	help := "\r\n"
	help += colorCyan + colorBold + "╔══════════════════════════════════════════════════════════════╗\r\n" + colorReset
	help += colorCyan + colorBold + "║              " + colorYellow + "⌨️  HOST SELECTION COMMANDS" + colorCyan + "                   ║\r\n" + colorReset
	help += colorCyan + colorBold + "╠══════════════════════════════════════════════════════════════╣\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "[1-N]" + colorCyan + "  → " + colorWhite + "Connect to host by number" + strings.Repeat(" ", 27) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "n/next" + colorCyan + " → " + colorWhite + "Go to next page" + strings.Repeat(" ", 37) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "p/prev" + colorCyan + " → " + colorWhite + "Go to previous page" + strings.Repeat(" ", 33) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "b/back" + colorCyan + " → " + colorWhite + "Return to group selection" + strings.Repeat(" ", 27) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "g/group" + colorCyan + "→ " + colorWhite + "Return to group selection" + strings.Repeat(" ", 27) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "r/refresh" + colorCyan + " " + colorWhite + "Refresh current page" + strings.Repeat(" ", 32) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "h/help" + colorCyan + " → " + colorWhite + "Show this help message" + strings.Repeat(" ", 30) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorGreen + "q/quit" + colorCyan + " → " + colorWhite + "Return to main menu" + strings.Repeat(" ", 33) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "╠══════════════════════════════════════════════════════════════╣\r\n" + colorReset
	help += colorCyan + colorBold + "║  " + colorYellow + " Tip:" + colorWhite + " Press ENTER without input to show prompt again" + strings.Repeat(" ", 4) + colorCyan + "║\r\n" + colorReset
	help += colorCyan + colorBold + "╚══════════════════════════════════════════════════════════════╝\r\n" + colorReset
	m.channel.Write([]byte(help))
}

// showMainHelp 显示主菜单帮助
func (m *MenuV2) showMainHelp() {
	help := "\r\n"

	// 标题
	help += "  " + colorYellow + colorBold + " QUICK COMMANDS" + colorReset + "\r\n\r\n"

	// 主菜单命令
	help += "  " + colorCyan + "Main Menu:" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "g" + colorReset + "  " + colorWhite + "→  View & Select Host Groups" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "p" + colorReset + "  " + colorWhite + "→  Quick access to Default Group" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "h" + colorReset + "  " + colorWhite + "→  Show Detailed Help" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "q" + colorReset + "  " + colorWhite + "→  Quit & Disconnect" + colorReset + "\r\n\r\n"

	// 分组选择命令
	help += "  " + colorCyan + "Group Selection:" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "[1-9]" + colorReset + "  " + colorWhite + "→  Select Group" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "b" + colorReset + "      " + colorWhite + "→  Back to Main Menu" + colorReset + "\r\n\r\n"

	// 主机选择命令
	help += "  " + colorCyan + "Host Selection:" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "[1-9]" + colorReset + "  " + colorWhite + "→  Connect to Host" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "n" + colorReset + "      " + colorWhite + "→  Next Page" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "p" + colorReset + "      " + colorWhite + "→  Previous Page" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "b/g" + colorReset + "    " + colorWhite + "→  Back to Groups" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "r" + colorReset + "      " + colorWhite + "→  Refresh List" + colorReset + "\r\n"
	help += "    " + colorGreen + colorBold + "q" + colorReset + "      " + colorWhite + "→  Quit to Main Menu" + colorReset + "\r\n\r\n"

	m.channel.Write([]byte(help))
}

// showDetailedHelp 显示详细帮助
func (m *MenuV2) showDetailedHelp() {
	help := "\r\n"
	help += "  " + colorYellow + colorBold + "📖 HELP GUIDE" + colorReset + "\r\n\r\n"

	// 主菜单命令
	help += "  " + colorCyan + colorBold + "📌 Main Menu:" + colorReset + "\r\n"
	help += "    " + colorGreen + "g" + colorReset + "  " + colorWhite + "View and select host groups" + colorReset + "\r\n"
	help += "    " + colorGreen + "p" + colorReset + "  " + colorWhite + "Quick access to default group" + colorReset + "\r\n"
	help += "    " + colorGreen + "h" + colorReset + "  " + colorWhite + "Show this help guide" + colorReset + "\r\n"
	help += "    " + colorGreen + "q" + colorReset + "  " + colorWhite + "Quit and disconnect" + colorReset + "\r\n\r\n"

	// 分组选择
	help += "  " + colorCyan + colorBold + "📂 Group Selection:" + colorReset + "\r\n"
	help += "    " + colorGreen + "[1-9]" + colorReset + "  " + colorWhite + "Select group by number" + colorReset + "\r\n"
	help += "    " + colorGreen + "b" + colorReset + "      " + colorWhite + "Back to main menu" + colorReset + "\r\n\r\n"

	// 主机选择
	help += "  " + colorCyan + colorBold + "🖥  Host Selection:" + colorReset + "\r\n"
	help += "    " + colorGreen + "[1-9]" + colorReset + "      " + colorWhite + "Connect to host" + colorReset + "\r\n"
	help += "    " + colorGreen + "n/next" + colorReset + "    " + colorWhite + "Next page" + colorReset + "\r\n"
	help += "    " + colorGreen + "p/prev" + colorReset + "    " + colorWhite + "Previous page" + colorReset + "\r\n"
	help += "    " + colorGreen + "b/back" + colorReset + "    " + colorWhite + "Back to groups" + colorReset + "\r\n"
	help += "    " + colorGreen + "r/refresh" + colorReset + "  " + colorWhite + "Refresh list" + colorReset + "\r\n"
	help += "    " + colorGreen + "h/?" + colorReset + "       " + colorWhite + "Show help" + colorReset + "\r\n"
	help += "    " + colorGreen + "q/quit" + colorReset + "    " + colorWhite + "Return to menu" + colorReset + "\r\n\r\n"

	// 提示
	help += "  " + colorYellow + " Tips:" + colorReset + "\r\n"
	help += "    • Commands are case-insensitive\r\n"
	help += "    • Press ENTER for prompt\r\n"
	help += "    • Type 'exit' to go back\r\n\r\n"

	// 安全提示
	help += "  " + colorRed + "  Security:" + colorReset + "\r\n"
	help += "    • All sessions are recorded\r\n"
	help += "    • Unauthorized access will be reported\r\n\r\n"

	m.channel.Write([]byte(help))
}

// fallbackToAllHosts 降级到显示所有主机（无分组模式）
func (m *MenuV2) fallbackToAllHosts(userID string) (*types.HostInfo, bool) {
	hosts, err := m.selector.ListAvailableHosts(userID)
	if err != nil {
		m.showError(fmt.Sprintf("Failed to list hosts: %v", err))
		return nil, true
	}

	if len(hosts) == 0 {
		m.showError("No hosts available for your account.")
		return nil, true
	}

	m.currentHosts = hosts
	m.showSimpleHostList()

	for {
		m.channel.Write([]byte(fmt.Sprintf("\r\n%sOpt>%s ", colorYellow+colorBold, colorReset)))
		command := m.readCommand()
		cmd := strings.ToLower(strings.TrimSpace(command))

		if cmd == "q" || cmd == "quit" || cmd == "exit" {
			return nil, true
		}

		choice, err := strconv.Atoi(cmd)
		if err == nil && choice >= 1 && choice <= len(hosts) {
			return &hosts[choice-1], false
		}

		m.showError(fmt.Sprintf("Invalid selection. Please enter 1-%d or 'q' to quit", len(hosts)))
	}
}

// showSimpleHostList 显示简单主机列表（无分组）
func (m *MenuV2) showSimpleHostList() {
	m.channel.Write([]byte(fmt.Sprintf("\r\n%s=== Available Hosts ===%s\r\n", colorCyan+colorBold, colorReset)))
	m.channel.Write([]byte(fmt.Sprintf("%s%-5s %-25s %-20s %-10s %-10s%s\r\n",
		colorCyan+colorBold, "No.", "Name", "IP Address", "Type", "Status", colorReset)))
	m.channel.Write([]byte(fmt.Sprintf("%s%s%s\r\n",
		colorCyan, strings.Repeat("-", 75), colorReset)))

	for i, host := range m.currentHosts {
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

// readCommand 读取用户命令
func (m *MenuV2) readCommand() string {
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
		case '\r', '\n':
			m.channel.Write([]byte("\r\n"))
			return string(inputBuffer)

		case 0x7f, 0x08: // Backspace
			if len(inputBuffer) > 0 {
				inputBuffer = inputBuffer[:len(inputBuffer)-1]
				m.channel.Write([]byte("\b \b"))
			}

		case 0x03: // Ctrl+C
			return "q"

		case 0x04: // Ctrl+D
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

// showError 显示错误信息
func (m *MenuV2) showError(errMsg string) {
	msg := "\r\n" + colorRed + "   " + colorRed + colorBold + "Error: " + colorWhite + errMsg + colorReset + "\r\n"
	m.channel.Write([]byte(msg))
}

// showMessage 显示消息
func (m *MenuV2) showMessage(msg string) {
	message := "\r\n" + colorCyan + "  ℹ️  " + colorWhite + msg + colorReset + "\r\n"
	m.channel.Write([]byte(message))
}

// ShowReturnToMenu 显示返回菜单的消息（自动返回，不需要等待）
func (m *MenuV2) ShowReturnToMenu() {
	msg := "\r\n"
	msg += colorGreen + "  ✓ " + colorWhite + "Connection closed. Returning to main menu..." + colorReset + "\r\n"
	m.channel.Write([]byte(msg))

	// 立即显示菜单帮助，让用户知道可以做什么
	m.showMainHelp()
}

// ShowError 显示错误信息（公开方法，兼容旧接口）
func (m *MenuV2) ShowError(errMsg string) {
	m.showError(errMsg)
}

// PromptPressToContinue 提示按任意键继续（兼容旧接口）
func (m *MenuV2) PromptPressToContinue() {
	m.channel.Write([]byte(fmt.Sprintf("\r\n%sPress Enter to continue...%s ", colorCyan, colorReset)))
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

// ShowConnectionInfo 显示连接信息（兼容旧接口）
func (m *MenuV2) ShowConnectionInfo(host *types.HostInfo) {
	// TODO: host.Username 已移除，需要从 SystemUser 获取
	username := host.Username
	if username == "" {
		username = "(system user)" // TODO: 显示实际的系统用户名
	}
	info := fmt.Sprintf("\r\n%sConnecting to %s (%s:%d) as %s...%s\r\n",
		colorGreen, host.Name, host.IP, host.Port, username, colorReset)
	m.channel.Write([]byte(info))
}
