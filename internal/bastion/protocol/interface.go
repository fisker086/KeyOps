package protocol

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolSSH ProtocolType = "ssh"
	ProtocolRDP ProtocolType = "rdp"
)

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	// 目标主机信息
	HostID   string `json:"hostId"`
	HostIP   string `json:"hostIp"`
	HostPort int    `json:"hostPort"`

	// 认证信息
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`

	// 协议类型
	Protocol ProtocolType `json:"protocol"`

	// 会话信息
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	ProxyID   string `json:"proxyId"`

	// 超时设置
	Timeout time.Duration `json:"timeout,omitempty"`

	// 协议特定选项
	Options map[string]interface{} `json:"options,omitempty"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID string       `json:"sessionId"`
	ProxyID   string       `json:"proxyId"`
	UserID    string       `json:"userId"`
	Username  string       `json:"username"`
	HostID    string       `json:"hostId"`
	HostIP    string       `json:"hostIp"`
	HostPort  int          `json:"hostPort"`
	Protocol  ProtocolType `json:"protocol"`
	Status    string       `json:"status"` // active, closed, error
	StartTime time.Time    `json:"startTime"`
	EndTime   *time.Time   `json:"endTime,omitempty"`
	BytesIn   int64        `json:"bytesIn"`
	BytesOut  int64        `json:"bytesOut"`
	ClientIP  string       `json:"clientIp"`
}

// ProtocolHandler 协议处理器接口 - 所有协议必须实现
type ProtocolHandler interface {
	// GetProtocolType 获取协议类型
	GetProtocolType() ProtocolType

	// Connect 建立连接
	Connect(ctx context.Context, config *ConnectionConfig) error

	// HandleWebSocket 处理 WebSocket 连接
	HandleWebSocket(ws *websocket.Conn) error

	// Close 关闭连接
	Close() error

	// GetSessionInfo 获取会话信息
	GetSessionInfo() *SessionInfo

	// IsAlive 检查连接是否存活
	IsAlive() bool
}

// TerminalHandler 终端类协议处理器（SSH）
type TerminalHandler interface {
	ProtocolHandler

	// Resize 调整终端大小
	Resize(width, height int) error
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
}

// SessionRecorder 会话记录器接口
type SessionRecorder interface {
	// RecordStart 记录会话开始
	RecordStart(info *SessionInfo) error

	// RecordEnd 记录会话结束
	RecordEnd(sessionID string, endTime time.Time) error

	// RecordData 记录数据流
	RecordData(sessionID string, direction string, data []byte) error

	// RecordCommand 记录命令
	RecordCommand(sessionID, command, output string, exitCode int) error

	// RecordError 记录错误
	RecordError(sessionID, errorMsg string) error
}
