package auth

import (
	"context"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/distributed"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
)

// HostMonitorService 主机状态监控服务
type HostMonitorService struct {
	hostRepo      *repository.HostRepository
	settingRepo   *repository.SettingRepository
	interval      time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
	config        *model.HostMonitorConfig
	configMu      sync.RWMutex
	ticker        *time.Ticker
	tickerMu      sync.Mutex    // 保护 ticker 的并发访问
	isRunning     bool          // 定时器是否正在运行
	tickerStop    chan struct{} // 用于停止当前运行的 ticker goroutine
	tickerStopped chan struct{} // 用于确认 ticker goroutine 已停止
}

// NewHostMonitorService 创建主机监控服务
func NewHostMonitorService(hostRepo *repository.HostRepository, settingRepo *repository.SettingRepository, intervalMinutes int) *HostMonitorService {
	if intervalMinutes <= 0 {
		intervalMinutes = 5 // 默认5分钟
	}

	service := &HostMonitorService{
		hostRepo:    hostRepo,
		settingRepo: settingRepo,
		interval:    time.Duration(intervalMinutes) * time.Minute,
		stopChan:    make(chan struct{}),
		config: &model.HostMonitorConfig{
			Enabled:    true,
			Interval:   intervalMinutes,
			Method:     model.MonitorMethodTCP,
			Timeout:    3,
			Concurrent: 20,
		},
	}

	// 从数据库加载配置
	service.loadConfig()

	// 如果启用了 Redis，监听配置变更
	if pkgredis.IsEnabled() {
		configSync := distributed.NewConfigSyncManager(pkgredis.GetClient(), "zjump:config:changes")
		configSync.AddListener(func(key string, value string) {
			// 只处理主机监控相关的配置变更
			if key == "host_monitor_enabled" ||
				key == "host_monitor_interval" ||
				key == "host_monitor_method" ||
				key == "host_monitor_timeout" ||
				key == "host_monitor_concurrent" {
				logger.Infof("[HostMonitor] Received config change from Redis: %s = %s", key, value)
				service.ReloadConfig()
			}
		})
		go configSync.Start()
	}

	return service
}

// loadConfig 从数据库加载配置
func (s *HostMonitorService) loadConfig() {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	// 读取配置项
	if enabled, err := s.settingRepo.Get("host_monitor_enabled"); err == nil && enabled != "" {
		s.config.Enabled = enabled == "true"
	}
	if interval, err := s.settingRepo.Get("host_monitor_interval"); err == nil && interval != "" {
		if val, err := strconv.Atoi(interval); err == nil && val > 0 {
			s.config.Interval = val
			s.interval = time.Duration(val) * time.Minute
		}
	}
	if method, err := s.settingRepo.Get("host_monitor_method"); err == nil && method != "" {
		s.config.Method = method
	}
	if timeout, err := s.settingRepo.Get("host_monitor_timeout"); err == nil && timeout != "" {
		if val, err := strconv.Atoi(timeout); err == nil && val > 0 {
			s.config.Timeout = val
		}
	}
	if concurrent, err := s.settingRepo.Get("host_monitor_concurrent"); err == nil && concurrent != "" {
		if val, err := strconv.Atoi(concurrent); err == nil && val > 0 {
			s.config.Concurrent = val
		}
	}

	logger.Infof("[HostMonitor] Config loaded: enabled=%v, interval=%dm, method=%s, timeout=%ds, concurrent=%d",
		s.config.Enabled, s.config.Interval, s.config.Method, s.config.Timeout, s.config.Concurrent)
}

// ReloadConfig 重新加载配置
func (s *HostMonitorService) ReloadConfig() {
	oldInterval := s.interval

	s.configMu.RLock()
	oldEnabled := s.config.Enabled
	s.configMu.RUnlock()

	s.loadConfig()

	s.configMu.RLock()
	newEnabled := s.config.Enabled
	s.configMu.RUnlock()

	// 检查启用状态是否变化
	if oldEnabled != newEnabled {
		if newEnabled {
			logger.Infof("[HostMonitor]  Monitoring enabled, starting ticker...")
			s.startTicker()
		} else {
			logger.Infof("[HostMonitor] ⏸️  Monitoring disabled, stopping ticker...")
			s.stopTicker()
		}
		return
	}

	// 如果启用状态未变，但间隔时间改变了，重启定时器
	if newEnabled && oldInterval != s.interval {
		logger.Infof("[HostMonitor] Interval changed from %v to %v, restarting ticker", oldInterval, s.interval)
		s.stopTicker()
		s.startTicker()
	}
}

// GetConfig 获取当前配置
func (s *HostMonitorService) GetConfig() model.HostMonitorConfig {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return *s.config
}

// Start 启动监控服务（定时检查，启动时不立即执行）
func (s *HostMonitorService) Start() {
	s.configMu.RLock()
	enabled := s.config.Enabled
	s.configMu.RUnlock()

	if enabled {
		logger.Infof("[HostMonitor]  Host monitoring service started (interval: %v)", s.interval)
		s.startTicker()
	} else {
		logger.Infof("[HostMonitor] ⏸️  Host monitoring is disabled, ticker not started")
	}
}

// startTicker 启动定时器
func (s *HostMonitorService) startTicker() {
	s.tickerMu.Lock()

	// 如果已经在运行，先完全停止旧的
	if s.isRunning {
		s.tickerMu.Unlock()
		s.stopTickerInternal() // 完全停止旧的 goroutine
		s.tickerMu.Lock()
	}

	logger.Infof("[HostMonitor] ▶️  Starting ticker (interval: %v)", s.interval)

	// 创建新的停止信号 channel
	s.tickerStop = make(chan struct{})
	s.tickerStopped = make(chan struct{})
	s.ticker = time.NewTicker(s.interval)
	s.isRunning = true

	// 保存 channels 的引用，避免在 goroutine 中被替换
	tickerStop := s.tickerStop
	tickerStopped := s.tickerStopped
	ticker := s.ticker

	s.tickerMu.Unlock()

	s.wg.Add(1)
	go func() {
		defer func() {
			s.wg.Done()
			close(tickerStopped) // 通知已完全停止
		}()

		for {
			select {
			case <-ticker.C:
				s.checkAllHosts()

			case <-tickerStop:
				// 收到停止信号，清理并退出
				logger.Info("[HostMonitor] ⏹️  Ticker goroutine stopping...")
				ticker.Stop()
				return

			case <-s.stopChan:
				// 整个服务停止
				logger.Info("[HostMonitor]  Host monitoring service stopped")
				ticker.Stop()
				s.tickerMu.Lock()
				s.isRunning = false
				s.ticker = nil
				s.tickerMu.Unlock()
				return
			}
		}
	}()
}

// stopTickerInternal 内部方法：完全停止当前的 ticker goroutine
func (s *HostMonitorService) stopTickerInternal() {
	s.tickerMu.Lock()

	if !s.isRunning {
		s.tickerMu.Unlock()
		return
	}

	logger.Infof("[HostMonitor] ⏹️  Stopping ticker...")

	tickerStop := s.tickerStop
	tickerStopped := s.tickerStopped
	s.isRunning = false

	s.tickerMu.Unlock()

	// 发送停止信号
	close(tickerStop)

	// 等待 goroutine 完全停止
	<-tickerStopped

	s.tickerMu.Lock()
	s.ticker = nil
	s.tickerStop = nil
	s.tickerStopped = nil
	s.tickerMu.Unlock()

	logger.Infof("[HostMonitor]  Ticker stopped successfully")
}

// stopTicker 停止定时器（公开方法）
func (s *HostMonitorService) stopTicker() {
	s.stopTickerInternal()
}

// Stop 停止监控服务
func (s *HostMonitorService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// CheckAllHosts 检查所有主机状态（公开方法）
func (s *HostMonitorService) CheckAllHosts() {
	s.checkAllHosts()
}

// checkAllHosts 检查所有主机状态（内部方法）
func (s *HostMonitorService) checkAllHosts() {
	// 检查是否启用监控
	s.configMu.RLock()
	enabled := s.config.Enabled
	s.configMu.RUnlock()

	if !enabled {
		logger.Infof("[HostMonitor] ⏸️  Monitoring is disabled, skipping check...")
		return
	}

	// 如果启用了 Redis，使用分布式锁
	if pkgredis.IsEnabled() {
		s.checkAllHostsWithLock()
	} else {
		s.doCheckAllHosts()
	}
}

// checkAllHostsWithLock 使用分布式锁检查所有主机
// 如果Redis未启用，会降级为单机模式（锁获取失败但不影响主流程）
func (s *HostMonitorService) checkAllHostsWithLock() {
	// 创建分布式锁，锁的有效期为检测间隔的2倍（防止检测时间过长）
	lockKey := "zjump:host_monitor:lock"
	lock := distributed.NewRedisLock(pkgredis.GetClient(), lockKey, s.interval*2)

	// 尝试获取锁
	acquired, err := lock.TryLock()
	if err != nil {
		logger.Infof("[HostMonitor]  Failed to acquire lock: %v", err)
		return
	}

	if !acquired {
		logger.Infof("[HostMonitor] ⏭️  Another instance is checking hosts, skipping...")
		return
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			logger.Infof("[HostMonitor]   Failed to release lock: %v", err)
		}
	}()

	logger.Infof("[HostMonitor] 🔒 Acquired distributed lock, starting check...")
	s.doCheckAllHosts()
}

// doCheckAllHosts 执行实际的主机检测
func (s *HostMonitorService) doCheckAllHosts() {
	s.configMu.RLock()
	method := s.config.Method
	concurrent := s.config.Concurrent
	s.configMu.RUnlock()

	logger.Infof("[HostMonitor]  Starting host status check (method: %s)...", method)
	startTime := time.Now()

	// 获取所有主机（不分页，直接获取全部）
	hosts, _, err := s.hostRepo.FindAllWithPagination(1, 10000, "", []string{})
	if err != nil {
		logger.Infof("[HostMonitor]  Failed to load hosts: %v", err)
		return
	}

	if len(hosts) == 0 {
		logger.Info("[HostMonitor] No hosts to monitor")
		return
	}

	logger.Infof("[HostMonitor] Checking %d hosts (concurrent: %d)...", len(hosts), concurrent)

	// 使用goroutine并发检查，但限制并发数
	sem := make(chan struct{}, concurrent)
	var wg sync.WaitGroup

	onlineCount := 0
	offlineCount := 0
	var mu sync.Mutex

	for i := range hosts {
		wg.Add(1)
		go func(host *model.Host) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 检查主机状态
			online := s.checkHostStatus(host)

			// 更新状态
			newStatus := "offline"
			if online {
				newStatus = "online"
			}

			// 只有状态变化时才更新数据库
			if host.Status != newStatus {
				oldStatus := host.Status
				if err := s.hostRepo.UpdateStatus(host.ID, newStatus); err != nil {
					logger.Infof("[HostMonitor] Failed to update status for %s (%s): %v",
						host.Name, host.IP, err)
				} else {
					logger.Infof("[HostMonitor]  Host %s (%s): %s → %s",
						host.Name, host.IP, oldStatus, newStatus)
				}
			}

			mu.Lock()
			if online {
				onlineCount++
			} else {
				offlineCount++
			}
			mu.Unlock()
		}(&hosts[i])
	}

	wg.Wait()

	duration := time.Since(startTime)
	logger.Infof("[HostMonitor]  Check completed in %v: %d online, %d offline (total: %d)",
		duration, onlineCount, offlineCount, len(hosts))
}

// checkHostStatus 检查单个主机状态
func (s *HostMonitorService) checkHostStatus(host *model.Host) bool {
	s.configMu.RLock()
	method := s.config.Method
	timeout := time.Duration(s.config.Timeout) * time.Second
	s.configMu.RUnlock()

	// 根据配置的检测方式进行检测
	switch method {
	case model.MonitorMethodICMP:
		return s.checkICMP(host.IP, timeout)
	case model.MonitorMethodHTTP:
		return s.checkHTTP(host.IP, host.Port, timeout)
	case model.MonitorMethodTCP:
		fallthrough
	default:
		// 使用主机配置的端口进行TCP检测
		port := host.Port
		if port == 0 {
			port = 22 // 默认SSH端口
		}
		return s.checkTCPPort(host.IP, port, timeout)
	}
}

// checkTCPPort 检查TCP端口是否可达
func (s *HostMonitorService) checkTCPPort(ip string, port int, timeout time.Duration) bool {
	address := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkICMP 通过ICMP Ping检查主机是否在线
func (s *HostMonitorService) checkICMP(ip string, timeout time.Duration) bool {
	var cmd *exec.Cmd

	// 根据操作系统选择不同的ping命令
	switch runtime.GOOS {
	case "windows":
		// Windows: ping -n 1 -w <timeout_ms> <ip>
		timeoutMs := int(timeout.Milliseconds())
		cmd = exec.Command("ping", "-n", "1", "-w", fmt.Sprintf("%d", timeoutMs), ip)
	case "darwin":
		// macOS: ping -c 1 -W <timeout_ms> <ip>
		timeoutMs := int(timeout.Milliseconds())
		cmd = exec.Command("ping", "-c", "1", "-W", fmt.Sprintf("%d", timeoutMs), ip)
	default:
		// Linux: ping -c 1 -W <timeout_sec> <ip>
		timeoutSec := int(timeout.Seconds())
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		cmd = exec.Command("ping", "-c", "1", "-W", fmt.Sprintf("%d", timeoutSec), ip)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	err := cmd.Run()
	return err == nil
}

// checkHTTP 通过HTTP请求检查主机是否在线
func (s *HostMonitorService) checkHTTP(ip string, port int, timeout time.Duration) bool {
	// 默认使用80端口
	if port == 22 || port == 0 {
		port = 80
	}

	url := fmt.Sprintf("http://%s:%d", ip, port)

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随重定向
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 任何HTTP响应都算在线（包括4xx、5xx错误）
	return true
}

// CheckHostStatusNow 立即检查指定主机状态（手动触发）
func (s *HostMonitorService) CheckHostStatusNow(hostID string) (bool, error) {
	host, err := s.hostRepo.FindByID(hostID)
	if err != nil {
		return false, fmt.Errorf("主机不存在: %w", err)
	}

	online := s.checkHostStatus(host)

	newStatus := "offline"
	if online {
		newStatus = "online"
	}

	if err := s.hostRepo.UpdateStatus(host.ID, newStatus); err != nil {
		return online, fmt.Errorf("更新状态失败: %w", err)
	}

	logger.Infof("[HostMonitor] Manual check: %s (%s) is %s", host.Name, host.IP, newStatus)
	return online, nil
}
