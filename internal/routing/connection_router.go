package routing

import (
	"encoding/json"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// ConnectionRouter 连接路由器 - 智能决策是直连还是通过代理
type ConnectionRouter struct {
	hostRepo    *repository.HostRepository
	proxyRepo   *repository.ProxyRepository
	settingRepo *repository.SettingRepository
	cache       *ConnectivityCache
}

// ConnectivityCache 连接性测试缓存
type ConnectivityCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
}

type CacheEntry struct {
	Reachable  bool
	LatencyMs  int
	ExpireTime time.Time
}

func NewConnectivityCache() *ConnectivityCache {
	cache := &ConnectivityCache{
		cache: make(map[string]*CacheEntry),
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

func (c *ConnectivityCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.cache[key]
	if !found {
		return nil, false
	}

	if time.Now().After(entry.ExpireTime) {
		return nil, false
	}

	return entry, true
}

func (c *ConnectivityCache) Set(key string, reachable bool, latencyMs int, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &CacheEntry{
		Reachable:  reachable,
		LatencyMs:  latencyMs,
		ExpireTime: time.Now().Add(ttl),
	}
}

func (c *ConnectivityCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.ExpireTime) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// NewConnectionRouter 创建连接路由器
func NewConnectionRouter(
	hostRepo *repository.HostRepository,
	proxyRepo *repository.ProxyRepository,
	settingRepo *repository.SettingRepository,
) *ConnectionRouter {
	return &ConnectionRouter{
		hostRepo:    hostRepo,
		proxyRepo:   proxyRepo,
		settingRepo: settingRepo,
		cache:       NewConnectivityCache(),
	}
}

// MakeRoutingDecision 核心路由决策函数
func (r *ConnectionRouter) MakeRoutingDecision(hostID string, userID string, username string) (*model.RoutingDecision, error) {
	// 1. 获取主机信息
	host, err := r.hostRepo.FindByID(hostID)
	if err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	logger.Infof("[Router] Making routing decision for host %s (%s:%d) - login user: %s", host.Name, host.IP, host.Port, username)

	// 2. 检查 Host 级别配置（最高优先级）
	if host.ConnectionMode == model.ConnectionModeDirect {
		logger.Infof("[Router] Host configured for direct connection")
		return &model.RoutingDecision{
			Mode:   model.ConnectionModeDirect,
			Direct: true,
			Reason: fmt.Sprintf("Host '%s' configured for direct connection", host.Name),
		}, nil
	}

	if host.ConnectionMode == model.ConnectionModeProxy {
		logger.Infof("[Router] Host configured for proxy connection, proxyID: %s", host.ProxyID)
		return r.useSpecifiedProxy(host, host.ProxyID, fmt.Sprintf("Host '%s' configured for proxy connection", host.Name))
	}

	// 3. 基于标签的路由决策
	logger.Infof("[Router] Checking host tags for proxy routing...")
	decision, matched := r.checkTagBasedRouting(host)
	if matched {
		logger.Infof("[Router] Tag-based routing decision: %s", decision.Reason)
		return decision, nil
	}

	// 4. 默认策略：直接连接
	// API Server 直接连接到目标主机，不经过 proxy agent
	logger.Infof("[Router] No matching tag found, using DEFAULT DIRECT connection mode")
	logger.Infof("[Router] API Server will directly connect to %s:%d", host.IP, host.Port)

	return &model.RoutingDecision{
		Mode:   model.ConnectionModeDirect,
		Direct: true,
		Reason: fmt.Sprintf("Default policy: API Server direct connection to %s", host.Name),
	}, nil
}

// checkTagBasedRouting 基于标签的路由决策
// 从系统设置中读取"需要代理的标签列表"和"默认代理ID"
// 如果主机的标签包含在列表中，则使用代理连接
func (r *ConnectionRouter) checkTagBasedRouting(host *model.Host) (*model.RoutingDecision, bool) {
	// 0. 先检查是否有可用的 proxy 节点，如果没有则跳过标签路由检查
	availableProxies, err := r.proxyRepo.FindOnlineProxies()
	if err != nil || len(availableProxies) == 0 {
		logger.Infof("[Router] No active proxy agents available, skipping tag-based routing")
		return nil, false
	}

	// 1. 获取系统设置：需要代理的标签列表
	proxyTagsSetting, err := r.settingRepo.GetSettingByKey("routing_proxy_tags")
	if err != nil || proxyTagsSetting == nil {
		// 如果没有配置 proxy tags，也跳过（不记录错误日志，因为这是正常情况）
		return nil, false
	}

	// 2. 解析标签列表（JSON 数组格式）
	var proxyTags []string
	if err := json.Unmarshal([]byte(proxyTagsSetting.Value), &proxyTags); err != nil {
		logger.Infof("[Router] Failed to parse proxy tags: %v", err)
		return nil, false
	}

	if len(proxyTags) == 0 {
		logger.Infof("[Router] Proxy tags list is empty")
		return nil, false
	}

	logger.Infof("[Router] Configured proxy tags: %v", proxyTags)

	// 3. 解析主机标签
	var hostTags []string
	if host.Tags != "" {
		if err := json.Unmarshal([]byte(host.Tags), &hostTags); err != nil {
			logger.Infof("[Router] Failed to parse host tags: %v", err)
			return nil, false
		}
	}

	if len(hostTags) == 0 {
		logger.Infof("[Router] Host has no tags")
		return nil, false
	}

	logger.Infof("[Router] Host tags: %v", hostTags)

	// 4. 检查是否有匹配的标签
	matchedTag := ""
	for _, proxyTag := range proxyTags {
		for _, hostTag := range hostTags {
			if strings.EqualFold(strings.TrimSpace(proxyTag), strings.TrimSpace(hostTag)) {
				matchedTag = hostTag
				break
			}
		}
		if matchedTag != "" {
			break
		}
	}

	if matchedTag == "" {
		logger.Infof("[Router] No matching proxy tag found")
		return nil, false
	}

	logger.Infof("[Router] Matched proxy tag: %s", matchedTag)

	// 5. 获取默认代理ID
	defaultProxySetting, err := r.settingRepo.GetSettingByKey("routing_default_proxy_id")
	if err != nil || defaultProxySetting == nil {
		logger.Infof("[Router] No default proxy ID configured")
		return nil, false
	}

	proxyID := strings.TrimSpace(defaultProxySetting.Value)
	if proxyID == "" {
		logger.Infof("[Router] Default proxy ID is empty")
		return nil, false
	}

	// 6. 使用指定的代理
	decision, err := r.useSpecifiedProxy(
		host,
		proxyID,
		fmt.Sprintf("Host tag '%s' requires proxy connection", matchedTag),
	)
	if err != nil {
		logger.Infof("[Router] Failed to use default proxy: %v", err)
		return nil, false
	}

	return decision, true
}

// useSpecifiedProxy 使用指定的代理
func (r *ConnectionRouter) useSpecifiedProxy(host *model.Host, proxyID string, reason string) (*model.RoutingDecision, error) {
	if proxyID == "" {
		return nil, fmt.Errorf("proxy ID is empty")
	}

	proxy, err := r.proxyRepo.FindProxyInfoByID(proxyID)
	if err != nil {
		return nil, fmt.Errorf("specified proxy not found: %s", proxyID)
	}

	if proxy.Status != "online" {
		return nil, fmt.Errorf("specified proxy is offline: %s", proxyID)
	}

	return &model.RoutingDecision{
		Mode:     model.ConnectionModeProxy,
		ProxyID:  proxy.ProxyID,
		ProxyURL: fmt.Sprintf("ws://%s:%d/ws/connect", proxy.IP, proxy.Port),
		Direct:   false,
		Reason:   reason,
	}, nil
}

// checkDirectConnectivity 检查是否可以直连
func (r *ConnectionRouter) checkDirectConnectivity(ip string, port int, timeout time.Duration) (bool, int) {
	// 先查缓存
	cacheKey := fmt.Sprintf("%s:%d", ip, port)
	if cached, found := r.cache.Get(cacheKey); found {
		logger.Infof("[Router] Using cached connectivity result for %s (reachable: %v)", cacheKey, cached.Reachable)
		return cached.Reachable, cached.LatencyMs
	}

	logger.Infof("[Router] Testing direct connectivity to %s:%d...", ip, port)

	// TCP 连接测试
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	latencyMs := int(time.Since(start).Milliseconds())

	if err != nil {
		logger.Infof("[Router] Direct connection failed: %v", err)
		r.cache.Set(cacheKey, false, 0, 30*time.Second) // 失败缓存30秒
		return false, latencyMs
	}
	defer conn.Close()

	logger.Infof("[Router] Direct connection successful (latency: %dms)", latencyMs)
	r.cache.Set(cacheKey, true, latencyMs, 5*time.Minute) // 成功缓存5分钟
	return true, latencyMs
}

// findBestProxy 查找最佳代理
func (r *ConnectionRouter) findBestProxy(host *model.Host) (*model.ProxyInfo, error) {
	// 优先选择同一网络区域的代理
	if host.NetworkZone != "" {
		proxies, err := r.proxyRepo.FindOnlineProxiesByZone(host.NetworkZone)
		if err == nil && len(proxies) > 0 {
			logger.Infof("[Router] Found %d online proxies in zone '%s'", len(proxies), host.NetworkZone)
			return &proxies[0], nil
		}
	}

	// 查找任意可用代理
	proxies, err := r.proxyRepo.FindOnlineProxies()
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no online proxy available")
	}

	logger.Infof("[Router] Found %d online proxies (any zone)", len(proxies))

	// TODO: 可以扩展为负载均衡算法（如轮询、最少连接等）
	return &proxies[0], nil
}
