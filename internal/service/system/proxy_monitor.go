package system

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/metrics"
	"gorm.io/gorm"
)

// ProxyMonitor Proxy监控器
type ProxyMonitor struct {
	db               *gorm.DB
	checkInterval    time.Duration // 检查间隔
	heartbeatTimeout time.Duration // 心跳超时时间
	stopChan         chan struct{}
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval    time.Duration // 检查间隔，默认1分钟
	HeartbeatTimeout time.Duration // 心跳超时时间，默认2分钟
}

// NewProxyMonitor 创建Proxy监控器
func NewProxyMonitor(db *gorm.DB, config MonitorConfig) *ProxyMonitor {
	if config.CheckInterval == 0 {
		config.CheckInterval = 1 * time.Minute
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 2 * time.Minute
	}

	return &ProxyMonitor{
		db:               db,
		checkInterval:    config.CheckInterval,
		heartbeatTimeout: config.HeartbeatTimeout,
		stopChan:         make(chan struct{}),
	}
}

// Start 启动监控
func (m *ProxyMonitor) Start() {
	logger.Infof("[ProxyMonitor] Starting proxy monitor, check interval: %v, heartbeat timeout: %v",
		m.checkInterval, m.heartbeatTimeout)

	// 立即执行一次检查
	go m.checkProxies()

	// 定时检查
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkProxies()
		case <-m.stopChan:
			logger.Info("[ProxyMonitor] Stopped")
			return
		}
	}
}

// Stop 停止监控
func (m *ProxyMonitor) Stop() {
	close(m.stopChan)
}

// checkProxies 通过请求Prometheus metrics端点检查proxy状态
func (m *ProxyMonitor) checkProxies() {
	// 查询所有proxy
	var proxies []model.Proxy
	if err := m.db.Find(&proxies).Error; err != nil {
		logger.Infof("[ProxyMonitor] Failed to query proxies: %v", err)
		return
	}

	if len(proxies) == 0 {
		m.printStats()
		return
	}

	onlineCount := 0
	offlineCount := 0
	httpClient := &http.Client{
		Timeout: time.Duration(m.heartbeatTimeout),
	}

	for _, proxy := range proxies {
		// 构造健康检查端点URL (使用proxy的实际端口)
		// 优先使用 /health 端点，因为更轻量
		healthURL := fmt.Sprintf("http://%s:%d/health", proxy.IP, proxy.Port)

		// 请求健康检查端点
		isOnline := m.checkProxyHealth(httpClient, healthURL)

		newStatus := "offline"
		if isOnline {
			newStatus = "online"
			onlineCount++
		} else {
			offlineCount++
		}

		// 更新数据库
		needUpdate := false
		updates := map[string]interface{}{}

		// 状态改变时更新status
		if proxy.Status != newStatus {
			logger.Infof("[ProxyMonitor] Proxy %s (%s) status changed: %s -> %s",
				proxy.ProxyID, proxy.HostName, proxy.Status, newStatus)
			updates["status"] = newStatus
			needUpdate = true
		}

		// 在线时总是更新心跳时间
		if newStatus == "online" {
			updates["last_heartbeat"] = time.Now()
			needUpdate = true
		}

		if needUpdate {
			if err := m.db.Model(&model.Proxy{}).
				Where("proxy_id = ?", proxy.ProxyID).
				Updates(updates).Error; err != nil {
				logger.Infof("[ProxyMonitor] Failed to update proxy: %v", err)
			}
		}
	}

	if offlineCount > 0 {
		logger.Infof("[ProxyMonitor] Health check completed - Online: %d, Offline: %d", onlineCount, offlineCount)
	}

	// 统计信息
	m.printStats()
}

// checkProxyHealth 检查单个proxy的健康状态
func (m *ProxyMonitor) checkProxyHealth(client *http.Client, metricsURL string) bool {
	resp, err := client.Get(metricsURL)
	if err != nil {
		// 网络错误、超时等，认为离线
		return false
	}
	defer resp.Body.Close()

	// HTTP 200表示在线
	return resp.StatusCode == http.StatusOK
}

// printStats 打印统计信息并更新Prometheus指标
func (m *ProxyMonitor) printStats() {
	var stats struct {
		Total   int64
		Online  int64
		Offline int64
	}

	m.db.Model(&model.Proxy{}).Count(&stats.Total)
	m.db.Model(&model.Proxy{}).Where("status = ?", "online").Count(&stats.Online)
	m.db.Model(&model.Proxy{}).Where("status = ?", "offline").Count(&stats.Offline)

	// 更新Prometheus指标
	metrics.RegisteredProxies.Set(float64(stats.Total))
	metrics.OnlineProxies.Set(float64(stats.Online))
	metrics.OfflineProxies.Set(float64(stats.Offline))

	if stats.Total > 0 {
		logger.Infof("[ProxyMonitor] Stats - Total: %d, Online: %d, Offline: %d",
			stats.Total, stats.Online, stats.Offline)
	}

	// 更新每个proxy的心跳时间戳和状态
	var proxies []model.Proxy
	if err := m.db.Find(&proxies).Error; err == nil {
		for _, proxy := range proxies {
			// 设置心跳时间戳
			metrics.ProxyHeartbeatTimestamp.WithLabelValues(
				proxy.ProxyID,
				proxy.HostName,
			).Set(float64(proxy.LastHeartbeat.Unix()))

			// 设置proxy up状态
			var up float64
			if proxy.Status == "online" {
				up = 1
			}
			metrics.ProxyUp.WithLabelValues(
				proxy.ProxyID,
				proxy.HostName,
			).Set(up)
		}
	}
}

// ForceCheck 立即执行一次检查
func (m *ProxyMonitor) ForceCheck() {
	logger.Info("[ProxyMonitor] Force check triggered")
	m.checkProxies()
}
