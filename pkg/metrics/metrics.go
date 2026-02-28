package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// API Server Metrics

	// APIRequestsTotal API请求总数
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// APIRequestDuration API请求处理时长
	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// RegisteredProxies 已注册的proxy数量
	RegisteredProxies = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "registered_proxies_total",
			Help: "Total number of registered proxies",
		},
	)

	// OnlineProxies 在线的proxy数量
	OnlineProxies = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "online_proxies_total",
			Help: "Total number of online proxies",
		},
	)

	// OfflineProxies 离线的proxy数量
	OfflineProxies = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "offline_proxies_total",
			Help: "Total number of offline proxies",
		},
	)

	// ProxyHeartbeatTimestamp Proxy最后心跳时间戳
	ProxyHeartbeatTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proxy_heartbeat_timestamp",
			Help: "Timestamp of last heartbeat from proxy",
		},
		[]string{"proxy_id", "hostname"},
	)

	// Proxy Metrics

	// ProxyUp Proxy是否在线 (1=在线, 0=离线)
	ProxyUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proxy_up",
			Help: "Whether the proxy is up (1) or down (0)",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyActiveSessions 当前活跃会话数
	ProxyActiveSessions = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proxy_active_sessions",
			Help: "Number of active SSH sessions on the proxy",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyCommandsTotal 执行的命令总数
	ProxyCommandsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_commands_total",
			Help: "Total number of commands executed through the proxy",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxySessionsTotal 会话总数
	ProxySessionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_sessions_total",
			Help: "Total number of SSH sessions created on the proxy",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyConnectionDuration 连接时长
	ProxyConnectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxy_connection_duration_seconds",
			Help:    "SSH connection duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyCommandDuration 命令执行时长
	ProxyCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxy_command_duration_seconds",
			Help:    "Command execution duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10, 30},
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyDataSynced 已同步到后端的记录数
	ProxyDataSynced = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_data_synced_total",
			Help: "Total number of records synced to backend",
		},
		[]string{"proxy_id", "hostname", "type"}, // type: commands, sessions
	)

	// ProxyDataUnsyncedCommands 未同步的命令数
	ProxyDataUnsyncedCommands = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proxy_data_unsynced_commands",
			Help: "Number of unsynced command records",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxyDataUnsyncedSessions 未同步的会话数
	ProxyDataUnsyncedSessions = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proxy_data_unsynced_sessions",
			Help: "Number of unsynced session records",
		},
		[]string{"proxy_id", "hostname"},
	)

	// ProxySyncErrors 同步错误计数
	ProxySyncErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_sync_errors_total",
			Help: "Total number of sync errors",
		},
		[]string{"proxy_id", "hostname", "type"}, // type: commands, sessions
	)
)
