package audit

import (
	"context"
	"time"
)

// ConnectionType 连接类型
type ConnectionType string

const (
	ConnectionTypeSSHGateway ConnectionType = "ssh_gateway" // SSH客户端通过2223端口连接
	ConnectionTypeWebShell   ConnectionType = "webshell"    // Web浏览器通过WebSocket连接
	ConnectionTypeAPIServer  ConnectionType = "api_server"  // API服务器直连模式
)

// SessionInfo 统一的会话信息
type SessionInfo struct {
	SessionID      string         // 会话ID（唯一标识）
	ConnectionType ConnectionType // 连接类型
	ProxyID        string         // 代理ID或来源标识

	// 用户信息
	UserID   string // 平台用户ID
	Username string // 平台用户名
	ClientIP string // 客户端IP

	// 目标主机信息
	HostID       string // 目标主机ID
	HostIP       string // 目标主机IP
	HostPort     int    // 目标主机端口
	HostUsername string // 目标主机用户名
	HostName     string // 目标主机名称

	// 会话状态
	StartTime time.Time  // 开始时间
	EndTime   *time.Time // 结束时间
	Status    string     // 状态: connecting, active, closed, failed

	// 终端信息
	TerminalCols int // 终端列数
	TerminalRows int // 终端行数

	// 统计信息
	BytesIn  int64 // 输入字节数
	BytesOut int64 // 输出字节数
}

// CommandInfo 统一的命令信息
type CommandInfo struct {
	SessionID  string    // 会话ID
	ProxyID    string    // 代理ID或来源标识
	HostID     string    // 主机ID
	HostIP     string    // 主机IP
	UserID     string    // 用户ID
	Username   string    // 用户名
	Command    string    // 命令内容
	Output     string    // 命令输出（可选）
	ExitCode   int       // 退出码（可选）
	ExecutedAt time.Time // 执行时间
	DurationMs int64     // 执行耗时（毫秒）
}

// LoginRecord 统一的登录记录
type LoginRecord struct {
	ID         string     // 记录ID
	SessionID  string     // 会话ID
	UserID     string     // 用户ID
	Username   string     // 用户名
	HostID     string     // 主机ID
	HostName   string     // 主机名
	HostIP     string     // 主机IP
	LoginIP    string     // 登录源IP
	UserAgent  string     // 用户代理
	LoginTime  time.Time  // 登录时间
	LogoutTime *time.Time // 登出时间
	Duration   *int       // 持续时间（秒）
	Status     string     // 状态: connecting, active, completed, failed
}

// Auditor 统一的审计器接口
type Auditor interface {
	// === 登录审计 ===

	// AuditLoginStart 审计登录开始（连接尝试）
	// 在建立连接前调用，状态为 connecting
	AuditLoginStart(ctx context.Context, session *SessionInfo) error

	// AuditLoginSuccess 审计登录成功
	// 在连接成功后调用，创建会话记录，状态改为 active
	AuditLoginSuccess(ctx context.Context, session *SessionInfo) error

	// AuditLoginFailed 审计登录失败
	// 在连接失败时调用，状态改为 failed
	AuditLoginFailed(ctx context.Context, sessionID string, endTime time.Time, reason string) error

	// === 会话审计 ===

	// AuditSessionEnd 审计会话结束
	// 在会话正常结束时调用，更新会话和登录记录
	AuditSessionEnd(ctx context.Context, sessionID string, endTime time.Time, recording string) error

	// === 命令审计 ===

	// AuditCommand 审计命令执行
	AuditCommand(ctx context.Context, cmd *CommandInfo) error

	// === 数据流审计（可选） ===

	// AuditData 审计数据流（用于实时监控）
	AuditData(ctx context.Context, sessionID string, direction string, data []byte) error
}

// Recorder 统一的会话录制器接口
type Recorder interface {
	// RecordStart 开始录制
	RecordStart(session *SessionInfo)

	// RecordInput 记录用户输入
	RecordInput(sessionID string, data string)

	// RecordOutput 记录输出
	RecordOutput(sessionID string, data string)

	// RecordEnd 结束录制
	RecordEnd(sessionID string, endTime time.Time)

	// GetRecording 获取录制内容（Asciinema格式）
	GetRecording(sessionID string) (string, error)

	// Close 关闭录制器
	Close() error
}

// AuditService 统一的审计服务（组合审计器和录制器）
type AuditService struct {
	Auditor  Auditor
	Recorder Recorder
}

// NewAuditService 创建审计服务
func NewAuditService(auditor Auditor, recorder Recorder) *AuditService {
	return &AuditService{
		Auditor:  auditor,
		Recorder: recorder,
	}
}
