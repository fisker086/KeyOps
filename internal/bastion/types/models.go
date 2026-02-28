package types

import (
	"time"
)

// CommandRecord 命令执行记录
type CommandRecord struct {
	ProxyID    string    `json:"proxy_id"`    // 代理ID
	SessionID  string    `json:"session_id"`  // 会话ID
	HostID     string    `json:"host_id"`     // 主机ID
	UserID     string    `json:"user_id"`     // 用户ID
	Username   string    `json:"username"`    // 用户名
	HostIP     string    `json:"host_ip"`     // 主机IP
	Command    string    `json:"command"`     // 执行的命令
	Output     string    `json:"output"`      // 命令输出
	ExitCode   int       `json:"exit_code"`   // 退出码
	ExecutedAt time.Time `json:"executed_at"` // 执行时间
	DurationMs int64     `json:"duration_ms"` // 执行耗时(毫秒)

	// 文件存储专用字段（不序列化到JSON）
	FilePath string `json:"-"` // 文件路径（用于同步后移动）
}

// SessionRecord 会话记录
type SessionRecord struct {
	ProxyID      string     `json:"proxy_id"`           // 代理ID
	SessionID    string     `json:"session_id"`         // 会话ID
	HostID       string     `json:"host_id"`            // 主机ID
	HostName     string     `json:"host_name"`          // 主机名
	UserID       string     `json:"user_id"`            // 用户ID（关联users表）
	Username     string     `json:"username"`           // 平台登录用户（admin等，来自users表）
	HostIP       string     `json:"host_ip"`            // 主机IP
	StartTime    time.Time  `json:"start_time"`         // 开始时间
	EndTime      *time.Time `json:"end_time,omitempty"` // 结束时间
	Status       string     `json:"status"`             // 状态: active, closed
	Recording    string     `json:"recording"`          // asciinema 格式的录制数据
	TerminalCols int        `json:"terminal_cols"`      // 终端列数
	TerminalRows int        `json:"terminal_rows"`      // 终端行数

	// 文件存储专用字段（不序列化到JSON）
	FilePath string `json:"-"` // 文件路径（用于同步后移动）
}

// LoginRecord 登录记录
type LoginRecord struct {
	SessionID  string     `json:"session_id"`            // 会话ID
	UserID     string     `json:"user_id"`               // 用户ID（关联users表）
	HostID     string     `json:"host_id"`               // 主机ID
	HostName   string     `json:"host_name"`             // 主机名
	HostIP     string     `json:"host_ip"`               // 主机IP
	Username   string     `json:"username"`              // 平台登录用户（admin等，来自users表）
	LoginTime  time.Time  `json:"login_time"`            // 登录时间
	LogoutTime *time.Time `json:"logout_time,omitempty"` // 登出时间
	Status     string     `json:"status"`                // 状态: connecting, completed, failed
}

// ProxyInfo proxy信息（用于注册到后端）
type ProxyInfo struct {
	ProxyID   string    `json:"proxy_id"`   // Proxy唯一ID
	HostName  string    `json:"host_name"`  // 主机名
	IP        string    `json:"ip"`         // IP地址
	Port      int       `json:"port"`       // 端口
	Status    string    `json:"status"`     // 状态: online, offline
	Version   string    `json:"version"`    // 版本
	StartTime time.Time `json:"start_time"` // 启动时间
}
