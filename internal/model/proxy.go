package model

import (
	"time"
)

// Proxy 代理服务器信息
type Proxy struct {
	ID            string    `gorm:"column:id;primaryKey;type:varchar(64)" json:"id"`
	ProxyID       string    `gorm:"column:proxy_id;uniqueIndex;not null;type:varchar(128)" json:"proxy_id"` // Proxy唯一ID
	HostName      string    `gorm:"column:host_name;type:varchar(255)" json:"host_name"`                    // 主机名
	IP            string    `gorm:"column:ip;type:varchar(64)" json:"ip"`                                   // IP地址
	Port          int       `gorm:"column:port" json:"port"`                                                // 端口
	Type          string    `gorm:"column:type;type:varchar(32)" json:"type"`                               // 代理类型: ssh, rdp
	Status        string    `gorm:"column:status;default:'offline';type:varchar(32)" json:"status"`         // 状态: online, offline
	Version       string    `gorm:"column:version;type:varchar(32)" json:"version"`                         // 版本
	NetworkZone   string    `gorm:"column:network_zone;type:varchar(50);index" json:"network_zone"`         // 网络区域（新增）
	StartTime     time.Time `gorm:"column:start_time" json:"start_time"`                                    // 启动时间
	LastHeartbeat time.Time `gorm:"column:last_heartbeat" json:"last_heartbeat"`                            // 最后心跳时间
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (Proxy) TableName() string {
	return "proxies"
}

// CommandHistory 命令历史记录（从proxy同步过来的）
type CommandHistory struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProxyID    string    `gorm:"column:proxy_id;index;type:varchar(128)" json:"proxy_id"`     // Proxy ID
	SessionID  string    `gorm:"column:session_id;index;type:varchar(128)" json:"session_id"` // 会话ID
	HostID     string    `gorm:"column:host_id;index;type:varchar(64)" json:"host_id"`        // 主机ID
	UserID     string    `gorm:"column:user_id;index;type:varchar(64)" json:"user_id"`        // 用户ID
	Username   string    `gorm:"column:username;type:varchar(128)" json:"username"`           // 用户名
	HostIP     string    `gorm:"column:host_ip;type:varchar(64)" json:"host_ip"`              // 主机IP
	Command    string    `gorm:"column:command;type:text" json:"command"`                     // 执行的命令
	Output     string    `gorm:"column:output;type:text" json:"output"`                       // 命令输出
	ExitCode   int       `gorm:"column:exit_code" json:"exit_code"`                           // 退出码
	ExecutedAt time.Time `gorm:"column:executed_at;index" json:"executed_at"`                 // 执行时间
	DurationMs int64     `gorm:"column:duration_ms" json:"duration_ms"`                       // 执行耗时(毫秒)
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (CommandHistory) TableName() string {
	return "command_histories"
}

// SessionHistory 会话历史记录（从proxy同步过来的）
type SessionHistory struct {
	ID           uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProxyID      string     `gorm:"column:proxy_id;index;type:varchar(128)" json:"proxy_id"`           // Proxy ID
	SessionID    string     `gorm:"column:session_id;uniqueIndex;type:varchar(128)" json:"session_id"` // 会话ID
	HostID       string     `gorm:"column:host_id;index;type:varchar(64)" json:"host_id"`              // 主机ID
	UserID       string     `gorm:"column:user_id;index;type:varchar(64)" json:"user_id"`              // 用户ID
	Username     string     `gorm:"column:username;type:varchar(128)" json:"username"`                 // 用户名
	HostIP       string     `gorm:"column:host_ip;type:varchar(64)" json:"host_ip"`                    // 主机IP
	StartTime    time.Time  `gorm:"column:start_time;index" json:"start_time"`                         // 开始时间
	EndTime      *time.Time `gorm:"column:end_time" json:"end_time"`                                   // 结束时间
	Status       string     `gorm:"column:status;type:varchar(32)" json:"status"`                      // 状态: active, closed
	Recording    string     `gorm:"column:recording;type:longtext" json:"recording"`                   // asciinema 格式录制数据
	TerminalCols int        `gorm:"column:terminal_cols;default:120" json:"terminal_cols"`             // 终端列数
	TerminalRows int        `gorm:"column:terminal_rows;default:30" json:"terminal_rows"`              // 终端行数
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (SessionHistory) TableName() string {
	return "session_histories"
}

// ProxyInfo Proxy 注册信息（用于查询 Proxy 地址）- 已合并到 Proxy，使用同一张表
// 这是一个轻量级视图，只包含路由所需的关键字段
type ProxyInfo struct {
	ID            string    `gorm:"column:id;primaryKey;type:varchar(64)" json:"id"`
	ProxyID       string    `gorm:"column:proxy_id;type:varchar(128);uniqueIndex" json:"proxy_id"`
	IP            string    `gorm:"column:ip;type:varchar(64)" json:"ip"`
	Port          int       `gorm:"column:port" json:"port"`
	Type          string    `gorm:"column:type;type:varchar(32)" json:"type"`
	Status        string    `gorm:"column:status;type:varchar(32)" json:"status"`
	NetworkZone   string    `gorm:"column:network_zone;type:varchar(50);index" json:"network_zone"` // 网络区域
	LastHeartbeat time.Time `gorm:"column:last_heartbeat" json:"last_heartbeat"`
}

// TableName 指定表名 - 现在使用 proxies 表
func (ProxyInfo) TableName() string {
	return "proxies"
}
