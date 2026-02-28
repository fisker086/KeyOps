package types

import (
	"context"
	"time"

	"golang.org/x/crypto/ssh"
)

// SessionInfo SSH会话信息
type SessionInfo struct {
	SessionID    string
	UserID       string
	Username     string
	ClientIP     string
	HostID       string
	HostName     string // 主机名称
	HostIP       string
	HostPort     int
	HostUsername string
	StartTime    time.Time
	EndTime      *time.Time
	Status       string // connecting, active, closed, error
	BytesIn      int64
	BytesOut     int64
	TerminalCols int
	TerminalRows int
}

// CommandInfo 命令信息
type CommandInfo struct {
	SessionID  string
	HostID     string
	HostIP     string
	UserID     string
	Username   string
	Command    string
	ExecutedAt time.Time
	Output     string
	ExitCode   int
	DurationMs int64
}

// AuthResult 认证结果
type AuthResult struct {
	Success           bool
	UserID            string
	Message           string
	RequiresTwoFactor bool // 是否需要MFA验证
}

// Authenticator 认证器接口
type Authenticator interface {
	// AuthenticatePassword 密码认证
	AuthenticatePassword(username, password string, clientIP string) (*AuthResult, error)

	// AuthenticatePublicKey 公钥认证
	AuthenticatePublicKey(username string, key ssh.PublicKey, clientIP string) (*AuthResult, error)
}

// Auditor 审计器接口
type Auditor interface {
	// AuditLoginStart 审计登录开始（连接尝试）
	AuditLoginStart(ctx context.Context, session *SessionInfo) error

	// AuditConnectionSuccess 审计连接成功
	AuditConnectionSuccess(ctx context.Context, session *SessionInfo) error

	// AuditSessionStart 审计会话开始（已废弃，使用AuditLoginStart+AuditConnectionSuccess）
	AuditSessionStart(ctx context.Context, session *SessionInfo) error

	// AuditSessionEnd 审计会话结束
	AuditSessionEnd(ctx context.Context, sessionID string, endTime time.Time) error

	// AuditSessionFailed 审计会话失败
	AuditSessionFailed(ctx context.Context, sessionID string, endTime time.Time, reason string) error

	// AuditCommand 审计命令
	AuditCommand(ctx context.Context, cmd *CommandInfo) error

	// AuditData 审计数据流
	AuditData(ctx context.Context, sessionID string, direction string, data []byte) error
}

// SessionRecorder 会话录制器接口
type SessionRecorder interface {
	// RecordStart 记录会话开始
	RecordStart(session *SessionInfo)

	// RecordData 记录数据
	RecordData(sessionID string, direction string, data []byte)

	// RecordEnd 记录会话结束
	RecordEnd(sessionID string, endTime time.Time)

	// RecordError 记录错误
	RecordError(sessionID string, errMsg string)

	// GetRecording 获取录制内容
	GetRecording(sessionID string) (string, error)

	// Close 关闭录制器
	Close() error
}

// HostSelector 主机选择器接口
type HostSelector interface {
	// ListAvailableHosts 列出可用主机
	ListAvailableHosts(userID string) ([]HostInfo, error)

	// GetHostInfo 获取主机信息
	GetHostInfo(hostID string) (*HostInfo, error)
}

// HostInfo 主机信息
type HostInfo struct {
	ID         string
	Name       string
	IP         string
	Port       int
	Username   string
	Password   string
	Tags       []string
	DeviceType string
	Status     string // 主机状态: online, offline, unknown
}

// HostGroupInfo 主机分组信息
type HostGroupInfo struct {
	ID          string
	Name        string
	Description string
	HostCount   int
	OnlineCount int
}

// TerminalHandler 终端处理器接口
type TerminalHandler interface {
	// HandleTerminal 处理终端会话
	HandleTerminal(ctx context.Context, channel ssh.Channel, session *SessionInfo) error
}
