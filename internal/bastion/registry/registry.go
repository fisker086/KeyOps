package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/fisker086/keyops/internal/bastion/storage"
)

// Registry Proxy注册器
type Registry struct {
	backendURL        string
	proxyInfo         storage.ProxyInfo
	heartbeatInterval time.Duration
	stopChan          chan struct{}
	registered        bool
}

// Config 注册器配置
type Config struct {
	BackendURL        string
	ProxyID           string
	Port              int
	HeartbeatInterval time.Duration // 心跳间隔，默认30秒
	Version           string        // Proxy版本
}

// NewRegistry 创建新的注册器
func NewRegistry(config Config) *Registry {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.Version == "" {
		config.Version = "1.0.0"
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &Registry{
		backendURL: config.BackendURL,
		proxyInfo: storage.ProxyInfo{
			ProxyID:   config.ProxyID,
			HostName:  hostname,
			IP:        getLocalIP(),
			Port:      config.Port,
			Status:    "online",
			Version:   config.Version,
			StartTime: time.Now(),
		},
		heartbeatInterval: config.HeartbeatInterval,
		stopChan:          make(chan struct{}),
	}
}

// Start 启动注册和心跳
func (r *Registry) Start() error {
	// 注册到后端
	if err := r.register(); err != nil {
		log.Printf("[Registry] Failed to register: %v", err)
		// 注册失败不阻止服务启动，后续心跳会重试
	} else {
		r.registered = true
		log.Println("[Registry] Successfully registered to backend")
	}

	// 启动心跳
	go r.startHeartbeat()

	return nil
}

// Stop 停止注册器
func (r *Registry) Stop() {
	close(r.stopChan)

	// 注销
	if r.registered {
		r.unregister()
	}
}

// register 注册到后端
func (r *Registry) register() error {
	url := r.backendURL + "/api/proxy/register"

	jsonData, err := json.Marshal(r.proxyInfo)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	return nil
}

// unregister 从后端注销
func (r *Registry) unregister() error {
	url := r.backendURL + "/api/proxy/unregister"

	payload := map[string]string{
		"proxy_id": r.proxyInfo.ProxyID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	log.Println("[Registry] Unregistered from backend")
	return nil
}

// startHeartbeat 启动心跳
func (r *Registry) startHeartbeat() {
	log.Printf("[Registry] Starting heartbeat, interval: %v", r.heartbeatInterval)

	ticker := time.NewTicker(r.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.sendHeartbeat(); err != nil {
				log.Printf("[Registry] Heartbeat failed: %v", err)

				// 如果心跳失败，可能是连接断开，尝试重新注册
				if !r.registered {
					if err := r.register(); err == nil {
						r.registered = true
						log.Println("[Registry] Re-registered successfully")
					}
				}
			}
		case <-r.stopChan:
			log.Println("[Registry] Heartbeat stopped")
			return
		}
	}
}

// sendHeartbeat 发送心跳
func (r *Registry) sendHeartbeat() error {
	url := r.backendURL + "/api/proxy/heartbeat"

	payload := map[string]interface{}{
		"proxy_id":  r.proxyInfo.ProxyID,
		"status":    "online",
		"timestamp": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		r.registered = false
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.registered = false
		return fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	r.registered = true
	return nil
}

// GetProxyInfo 获取Proxy信息
func (r *Registry) GetProxyInfo() storage.ProxyInfo {
	return r.proxyInfo
}

// getLocalIP 获取本机IP
func getLocalIP() string {
	// 方法1: 通过UDP连接获取本地出口IP（不会真的发送数据）
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// 方法2: 遍历网络接口
		return getIPFromInterfaces()
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// getIPFromInterfaces 从网络接口获取IP
func getIPFromInterfaces() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Failed to get local IP: %v", err)
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}
