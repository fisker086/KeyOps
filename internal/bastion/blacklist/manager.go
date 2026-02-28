package blacklist

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/bastion/client"
	"github.com/fisker086/keyops/internal/bastion/types"
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

// Manager 危险命令黑名单管理器
type Manager struct {
	apiClient        *client.ApiClient
	db               *gorm.DB              // 直接数据库访问（用于API Server）
	globalRules      []types.BlacklistRule // 全局规则
	userRules        []types.BlacklistRule // 用户级规则
	mu               sync.RWMutex
	refreshInterval  time.Duration
	stopChan         chan struct{}
	advancedDetector *AdvancedDetector // 高级检测器（防绕过）
	notificationMgr  interface{}       // 通知管理器（避免循环依赖）
}

// Config 黑名单管理器配置
type Config struct {
	ApiClient       *client.ApiClient
	RefreshInterval time.Duration // 刷新间隔，默认5分钟
}

// NewManager 创建黑名单管理器
func NewManager(config Config) *Manager {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 5 * time.Minute
	}

	manager := &Manager{
		apiClient:        config.ApiClient,
		globalRules:      make([]types.BlacklistRule, 0),
		userRules:        make([]types.BlacklistRule, 0),
		refreshInterval:  config.RefreshInterval,
		stopChan:         make(chan struct{}),
		advancedDetector: NewAdvancedDetector(), // 初始化高级检测器
	}

	// 初始加载黑名单
	if err := manager.refresh(); err != nil {
		log.Printf("[Blacklist] Warning: Failed to load initial blacklist: %v", err)
		// 使用默认黑名单
		manager.setDefaultBlacklist()
	}

	log.Printf("[Blacklist] Advanced detection enabled (anti-bypass)")
	return manager
}

// NewManagerFromDB 从数据库创建黑名单管理器（用于 API Server 直连模式）
func NewManagerFromDB(db *gorm.DB) *Manager {
	manager := &Manager{
		db:               db,
		globalRules:      make([]types.BlacklistRule, 0),
		userRules:        make([]types.BlacklistRule, 0),
		refreshInterval:  5 * time.Minute,
		stopChan:         make(chan struct{}),
		advancedDetector: NewAdvancedDetector(),
	}

	// 初始加载黑名单
	if err := manager.refreshFromDB(); err != nil {
		log.Printf("[Blacklist] Warning: Failed to load initial blacklist from DB: %v", err)
		manager.setDefaultBlacklist()
	}

	log.Printf("[Blacklist] Manager initialized from database with advanced detection")
	return manager
}

// Start 启动定期刷新
func (m *Manager) Start() {
	if m.apiClient != nil {
		// 使用 API Client 的定期刷新
		go m.startRefreshLoop()
	} else if m.db != nil {
		// 使用数据库的定期刷新
		go m.startRefreshLoopFromDB()
	}
}

// startRefreshLoopFromDB 从数据库定期刷新黑名单
func (m *Manager) startRefreshLoopFromDB() {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.refreshFromDB(); err != nil {
				log.Printf("[Blacklist] Failed to refresh from database: %v", err)
			}
		case <-m.stopChan:
			log.Println("[Blacklist] Stopping refresh loop (DB mode)")
			return
		}
	}
}

// Stop 停止刷新
func (m *Manager) Stop() {
	close(m.stopChan)
}

// IsBlocked 检查命令是否被阻止（针对特定用户）- 不发送通知
func (m *Manager) IsBlocked(command string, username string) bool {
	blocked, _ := m.checkBlocked(command, username)
	return blocked
}

// IsBlockedWithNotify 检查命令是否被阻止，并发送通知
func (m *Manager) IsBlockedWithNotify(command string, username string, hostIP string) bool {
	blocked, reason := m.checkBlocked(command, username)

	// 如果被阻止，发送通知
	if blocked {
		if m.notificationMgr != nil {
			if notifier, ok := m.notificationMgr.(interface {
				SendDangerousCommandAlert(username, hostIP, command, reason string)
			}); ok {
				go notifier.SendDangerousCommandAlert(username, hostIP, command, reason)
			}
		}
	}

	return blocked
}

// checkBlocked 内部方法：检查命令是否被阻止（不发送通知）
func (m *Manager) checkBlocked(command string, username string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	command = strings.TrimSpace(command)
	if command == "" {
		return false, ""
	}

	var blocked bool
	var reason string

	// 0. 首先使用高级检测器（防绕过）
	if m.advancedDetector != nil {
		if dangerous, detectedReason := m.advancedDetector.IsCommandDangerous(command); dangerous {
			blocked = true
			reason = detectedReason
		}
	}

	lowerCommand := strings.ToLower(command)

	// 1. 检查全局规则
	if !blocked {
		for _, rule := range m.globalRules {
			if m.matchCommand(lowerCommand, strings.ToLower(rule.Pattern)) {
				blocked = true
				reason = rule.Description
				if reason == "" {
					reason = "匹配全局黑名单规则: " + rule.Command
				}
				break
			}
		}
	}

	// 2. 检查用户级规则
	if !blocked && username != "" {
		for _, rule := range m.userRules {
			// 检查是否包含当前用户
			if m.containsUser(rule.Users, username) {
				if m.matchCommand(lowerCommand, strings.ToLower(rule.Pattern)) {
					blocked = true
					reason = rule.Description
					if reason == "" {
						reason = "匹配用户级黑名单规则: " + rule.Command
					}
					break
				}
			}
		}
	}

	return blocked, reason
}

// SetNotificationManager 设置通知管理器
func (m *Manager) SetNotificationManager(notificationMgr interface{}) {
	m.notificationMgr = notificationMgr
	log.Printf("[BlacklistManager] Notification manager set")
}

// GetBlockReason 获取命令被阻止的原因
func (m *Manager) GetBlockReason(command string, username string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	command = strings.TrimSpace(command)

	// 0. 首先检查高级检测器
	if m.advancedDetector != nil {
		if dangerous, reason := m.advancedDetector.IsCommandDangerous(command); dangerous {
			return fmt.Sprintf("高级安全检测: %s", reason)
		}
	}

	lowerCommand := strings.ToLower(command)

	// 检查全局规则
	for _, rule := range m.globalRules {
		if m.matchCommand(lowerCommand, strings.ToLower(rule.Pattern)) {
			if rule.Description != "" {
				return fmt.Sprintf("全局安全策略: %s", rule.Description)
			}
			return fmt.Sprintf("命令 '%s' 被全局安全策略禁止", rule.Command)
		}
	}

	// 检查用户级规则
	if username != "" {
		for _, rule := range m.userRules {
			if m.containsUser(rule.Users, username) {
				if m.matchCommand(lowerCommand, strings.ToLower(rule.Pattern)) {
					if rule.Description != "" {
						return fmt.Sprintf("用户安全策略: %s", rule.Description)
					}
					return fmt.Sprintf("命令 '%s' 对用户 '%s' 被禁止", rule.Command, username)
				}
			}
		}
	}

	return "命令被安全策略阻止"
}

// containsUser 检查用户列表中是否包含指定用户
func (m *Manager) containsUser(users []string, username string) bool {
	for _, u := range users {
		if strings.EqualFold(u, username) {
			return true
		}
	}
	return false
}

// matchCommand 命令匹配（支持 * 通配符）
func (m *Manager) matchCommand(command, pattern string) bool {
	// 精确匹配
	if command == pattern {
		return true
	}

	// 通配符匹配
	if strings.Contains(pattern, "*") {
		return matchPattern(command, pattern)
	}

	// 前缀匹配（命令开头）
	parts := strings.Fields(command)
	if len(parts) > 0 && strings.HasPrefix(parts[0], pattern) {
		return true
	}

	return false
}

// refreshFromDB 从数据库刷新黑名单
func (m *Manager) refreshFromDB() error {
	if m.db == nil {
		return fmt.Errorf("database connection not available")
	}

	var dbRules []model.BlacklistRule
	if err := m.db.Where("enabled = ?", true).Find(&dbRules).Error; err != nil {
		return err
	}

	// 转换为内部类型
	m.mu.Lock()
	defer m.mu.Unlock()

	m.globalRules = make([]types.BlacklistRule, 0)
	m.userRules = make([]types.BlacklistRule, 0)

	for _, rule := range dbRules {
		typeRule := types.BlacklistRule{
			ID:          rule.ID,
			Command:     rule.Command,
			Pattern:     rule.Pattern,
			Scope:       rule.Scope,
			Description: rule.Description,
			Enabled:     rule.Enabled,
			Users:       []string(rule.Users),
		}

		if rule.Scope == "global" {
			m.globalRules = append(m.globalRules, typeRule)
		} else if rule.Scope == "user" {
			m.userRules = append(m.userRules, typeRule)
		}
	}

	log.Printf("[Blacklist] Loaded %d global rules and %d user rules from database",
		len(m.globalRules), len(m.userRules))
	return nil
}

// refresh 从 API 刷新黑名单
func (m *Manager) refresh() error {
	rules, err := m.apiClient.FetchBlacklist()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 分类规则
	m.globalRules = make([]types.BlacklistRule, 0)
	m.userRules = make([]types.BlacklistRule, 0)

	for _, rule := range rules {
		if rule.Scope == "global" {
			m.globalRules = append(m.globalRules, rule)
		} else if rule.Scope == "user" {
			m.userRules = append(m.userRules, rule)
		}
	}

	log.Printf("[Blacklist] Loaded %d global rules, %d user rules",
		len(m.globalRules), len(m.userRules))
	return nil
}

// startRefreshLoop 启动定期刷新循环
func (m *Manager) startRefreshLoop() {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.refresh(); err != nil {
				log.Printf("[Blacklist] Failed to refresh blacklist: %v", err)
			} else {
				log.Printf("[Blacklist] Blacklist refreshed successfully")
			}
		case <-m.stopChan:
			log.Println("[Blacklist] Refresh loop stopped")
			return
		}
	}
}

// setDefaultBlacklist 设置默认黑名单（当 API 不可用时）
func (m *Manager) setDefaultBlacklist() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 默认全局危险命令
	m.globalRules = []types.BlacklistRule{
		{ID: "default-1", Command: "rm", Pattern: "rm", Description: "删除文件", Scope: "global", Enabled: true},
		{ID: "default-2", Command: "rm -rf", Pattern: "rm -rf *", Description: "递归强制删除", Scope: "global", Enabled: true},
		{ID: "default-3", Command: "dd", Pattern: "dd", Description: "磁盘操作", Scope: "global", Enabled: true},
		{ID: "default-4", Command: "mkfs", Pattern: "mkfs", Description: "格式化磁盘", Scope: "global", Enabled: true},
		{ID: "default-5", Command: "reboot", Pattern: "reboot", Description: "重启系统", Scope: "global", Enabled: true},
		{ID: "default-6", Command: "shutdown", Pattern: "shutdown", Description: "关机", Scope: "global", Enabled: true},
		{ID: "default-7", Command: "halt", Pattern: "halt", Description: "停机", Scope: "global", Enabled: true},
	}

	m.userRules = make([]types.BlacklistRule, 0)

	log.Printf("[Blacklist] Using default blacklist: %d global rules",
		len(m.globalRules))
}

// matchPattern 简单的模式匹配（支持 * 通配符）
func matchPattern(str, pattern string) bool {
	parts := strings.Split(pattern, "*")

	// 如果模式以 * 开头
	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}

	pos := 0
	for _, part := range parts {
		if part == "" {
			continue
		}

		idx := strings.Index(str[pos:], part)
		if idx == -1 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"global_rules": len(m.globalRules),
		"user_rules":   len(m.userRules),
		"total_rules":  len(m.globalRules) + len(m.userRules),
	}
}
