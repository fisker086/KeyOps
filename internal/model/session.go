package model

import (
	"time"
)

type LoginRecord struct {
	ID         string     `json:"id" gorm:"primaryKey;type:varchar(36)" bson:"id"`
	UserID     string     `json:"userId" gorm:"type:varchar(36);not null;index" bson:"user_id"` // 用户ID（必填，关联users表）
	HostID     string     `json:"hostId" gorm:"type:varchar(36);not null;index" bson:"host_id"`
	HostName   string     `json:"hostName" gorm:"type:varchar(255)" bson:"host_name"`
	HostIP     string     `json:"hostIp" gorm:"type:varchar(45)" bson:"host_ip"`
	Username   string     `json:"username" gorm:"type:varchar(100);index" bson:"username"` // 平台登录用户（admin等，来自users表）
	LoginIP    string     `json:"loginIp" gorm:"type:varchar(45)" bson:"login_ip,omitempty"` // 登录IP
	UserAgent  string     `json:"userAgent" gorm:"type:varchar(255)" bson:"user_agent,omitempty"` // 用户代理
	LoginTime  time.Time  `json:"loginTime" gorm:"type:timestamp;not null;index" bson:"login_time"`
	LogoutTime *time.Time `json:"logoutTime" gorm:"type:timestamp" bson:"logout_time,omitempty"`
	Duration   *int       `json:"duration" bson:"duration,omitempty"`                                              // 秒
	Status     string     `json:"status" gorm:"type:varchar(20);default:'active';index" bson:"status"` // active, completed, failed
	SessionID  string     `json:"sessionId" gorm:"type:varchar(100)" bson:"session_id"`
	CreatedAt  time.Time  `json:"createdAt" gorm:"autoCreateTime" bson:"created_at,omitempty"`
}

func (LoginRecord) TableName() string {
	return "login_records"
}

// LoginRecordWithType 带连接类型的登录记录（用于API返回）
// bson:",inline"：Mongo 聚合结果为扁平字段，必须内联解码，否则 LoginRecord 全为零值（历史页主机/时间错乱）
type LoginRecordWithType struct {
	LoginRecord    `bson:",inline"`
	ConnectionType string `json:"connectionType" gorm:"column:connection_type" bson:"connection_type"` // webshell, ssh_gateway, ssh_client
}

type SSHSession struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	SessionID string    `json:"sessionId" gorm:"type:varchar(100);uniqueIndex;not null"`
	HostID    string    `json:"hostId" gorm:"type:varchar(36);not null;index"`
	Username  string    `json:"username" gorm:"type:varchar(100)"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:'active'"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (SSHSession) TableName() string {
	return "ssh_sessions"
}

// SessionRecording 会话录制记录（统一表，支持 webshell 和 ssh_client）
type SessionRecording struct {
	ID             string     `json:"id" gorm:"primaryKey;type:varchar(36)" bson:"id"`
	SessionID      string     `json:"sessionId" gorm:"type:varchar(100);uniqueIndex;not null" bson:"session_id"`
	ConnectionType string     `json:"connectionType" gorm:"type:varchar(20);default:'webshell';index" bson:"connection_type"` // webshell, ssh_client
	ProxyID        string     `json:"proxyId" gorm:"type:varchar(100);index" bson:"proxy_id"`                          // Proxy ID (webshell使用)
	UserID         string     `json:"userId" gorm:"type:varchar(36);index" bson:"user_id"`                            // 用户ID（关联users表）
	HostID         string     `json:"hostId" gorm:"type:varchar(36);not null;index" bson:"host_id"`
	HostName       string     `json:"hostName" gorm:"type:varchar(255)" bson:"host_name"`
	HostIP         string     `json:"hostIp" gorm:"type:varchar(45)" bson:"host_ip"`
	Username       string     `json:"username" gorm:"type:varchar(100)" bson:"username"` // 平台登录用户（admin等，来自users表）
	StartTime      time.Time  `json:"startTime" gorm:"type:timestamp;not null;index" bson:"start_time"`
	EndTime        *time.Time `json:"endTime" gorm:"type:timestamp" bson:"end_time,omitempty"`
	Duration       string     `json:"duration" gorm:"type:varchar(50)" bson:"duration,omitempty"`
	CommandCount   int        `json:"commandCount" gorm:"default:0" bson:"command_count"`
	Status         string     `json:"status" gorm:"type:varchar(20);default:'active'" bson:"status"` // active, closed
	Recording      string     `json:"recording" gorm:"type:longtext" bson:"recording,omitempty"`                  // asciinema 格式的录制数据
	TerminalCols   int        `json:"terminalCols" gorm:"default:80" bson:"terminal_cols"`
	TerminalRows   int        `json:"terminalRows" gorm:"default:24" bson:"terminal_rows"`
	CreatedAt      time.Time  `json:"createdAt" gorm:"autoCreateTime" bson:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updatedAt" gorm:"autoUpdateTime" bson:"updated_at,omitempty"`
}

func (SessionRecording) TableName() string {
	return "session_recordings"
}

// CommandRecord 命令执行记录
type CommandRecord struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement" bson:"id,omitempty"`
	ProxyID    string    `json:"proxyId" gorm:"column:proxy_id;index;type:varchar(128)" bson:"proxy_id"`
	SessionID  string    `json:"sessionId" gorm:"column:session_id;index;type:varchar(128)" bson:"session_id"`
	HostID     string    `json:"hostId" gorm:"column:host_id;index;type:varchar(64)" bson:"host_id"`
	UserID     string    `json:"userId" gorm:"column:user_id;index;type:varchar(64)" bson:"user_id"`
	Username   string    `json:"username" gorm:"column:username;type:varchar(128)" bson:"username"`
	HostIP     string    `json:"hostIp" gorm:"column:host_ip;type:varchar(64)" bson:"host_ip"`
	Command    string    `json:"command" gorm:"column:command;type:text" bson:"command"`
	Output     string    `json:"output" gorm:"column:output;type:text" bson:"output,omitempty"`
	ExitCode   int       `json:"exitCode" gorm:"column:exit_code" bson:"exit_code"`
	ExecutedAt time.Time `json:"executedAt" gorm:"column:executed_at;index" bson:"executed_at"`
	DurationMs int64     `json:"durationMs" gorm:"column:duration_ms" bson:"duration_ms,omitempty"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at" bson:"created_at,omitempty"`
}

func (CommandRecord) TableName() string {
	return "command_histories"
}

// PodCommandRecord Pod 终端命令执行记录
type PodCommandRecord struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ClusterID   string    `json:"clusterId" gorm:"column:cluster_id;index;type:varchar(100)"`
	ClusterName string    `json:"clusterName" gorm:"column:cluster_name;type:varchar(255)"`
	Namespace   string    `json:"namespace" gorm:"column:namespace;index;type:varchar(255)"`
	PodName     string    `json:"podName" gorm:"column:pod_name;index;type:varchar(255)"`
	Container   string    `json:"container" gorm:"column:container;type:varchar(255)"`
	UserID      string    `json:"userId" gorm:"column:user_id;index;type:varchar(100)"`
	Username    string    `json:"username" gorm:"column:username;index;type:varchar(100)"`
	Command     string    `json:"command" gorm:"column:command;type:text"`
	ExecutedAt  time.Time `json:"executedAt" gorm:"column:executed_at;index"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (PodCommandRecord) TableName() string {
	return "pod_command_histories"
}

// LoginRecordsResponse 登录记录响应
type LoginRecordsResponse struct {
	Records []LoginRecord `json:"records"`
	Total   int64         `json:"total"`
}

// SessionRecordingsResponse 会话录制列表响应
type SessionRecordingsResponse struct {
	Sessions []SessionRecording `json:"sessions"`
	Total    int64              `json:"total"`
}

// CommandRecordsResponse 命令记录列表响应
type CommandRecordsResponse struct {
	Commands []CommandRecord `json:"commands"`
	Total    int64           `json:"total"`
}
