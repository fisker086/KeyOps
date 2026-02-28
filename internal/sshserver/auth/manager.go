package auth

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/internal/sshserver/types"
	"golang.org/x/crypto/ssh"
)

// AuthManager 认证管理器
type AuthManager struct {
	authService *service.AuthService
	handlers    []AuthHandler
	// 记录公钥认证失败的连接，阻止后续密码尝试
	failedPubkeyAttempts map[string]bool // key: remoteAddr
	attemptsMu           sync.RWMutex
}

// NewAuthManager 创建认证管理器
func NewAuthManager(authService *service.AuthService) *AuthManager {
	manager := &AuthManager{
		authService:          authService,
		handlers:             make([]AuthHandler, 0),
		failedPubkeyAttempts: make(map[string]bool),
	}

	// 注册认证处理器
	manager.RegisterHandler(NewPasswordHandler(authService))
	manager.RegisterHandler(NewPublicKeyHandler(authService))
	manager.RegisterHandler(NewMFAHandler(authService)) // 启用 MFA 接口

	logger.Infof("[Auth Manager] Initialized with %d handlers", len(manager.handlers))
	for _, h := range manager.handlers {
		logger.Infof("[Auth Manager] - Registered handler: %s", h.GetName())
	}

	// 启动清理 goroutine
	go manager.cleanupFailedAttempts()

	return manager
}

// cleanupFailedAttempts 定期清理过期的失败记录
func (m *AuthManager) cleanupFailedAttempts() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.attemptsMu.Lock()
		// 清空所有记录（5分钟后重置）
		m.failedPubkeyAttempts = make(map[string]bool)
		m.attemptsMu.Unlock()
	}
}

// GetAuthService 获取认证服务（用于SSH服务器）
func (m *AuthManager) GetAuthService() *service.AuthService {
	return m.authService
}

// RegisterHandler 注册认证处理器
func (m *AuthManager) RegisterHandler(handler AuthHandler) {
	m.handlers = append(m.handlers, handler)
}

// AuthenticatePassword 密码认证
func (m *AuthManager) AuthenticatePassword(username, password string, clientIP string) (*types.AuthResult, error) {
	logger.Infof("[Auth Manager] Password authentication request for user: %s from IP: %s", username, clientIP)

	//  首先检查：该连接之前是否有公钥认证失败
	m.attemptsMu.RLock()
	hasFailed := m.failedPubkeyAttempts[clientIP]
	m.attemptsMu.RUnlock()

	if hasFailed {
		logger.Infof("[Auth Manager]  BLOCKING password attempt - previous publickey authentication failed for this connection")
		logger.Infof("[Auth Manager]  User should fix their SSH key configuration, not fallback to password")
		return nil, fmt.Errorf("authentication method not allowed")
	}

	// 查找密码认证处理器
	for _, handler := range m.handlers {
		if handler.GetName() == "password" {
			// 检查该处理器是否可以处理该用户
			canHandle, err := handler.CanHandle(username)
			if err != nil {
				logger.Infof("[Auth Manager]  Handler check failed: %v", err)
				return nil, err
			}

			if !canHandle {
				logger.Infof("[Auth Manager]  Password authentication NOT allowed for user %s (check user's auth_method setting)", username)
				return nil, fmt.Errorf("password authentication is not allowed for this user")
			}

			logger.Infof("[Auth Manager]  Password authentication allowed for user %s, proceeding with authentication...", username)
			return handler.Authenticate(username, password, clientIP)
		}
	}

	return nil, fmt.Errorf("password handler not found")
}

// AuthenticatePublicKey 公钥认证
func (m *AuthManager) AuthenticatePublicKey(username string, key ssh.PublicKey, clientIP string) (*types.AuthResult, error) {
	logger.Infof("[Auth Manager] PublicKey authentication request for user: %s from IP: %s", username, clientIP)
	logger.Infof("[Auth Manager] Client key fingerprint: %s", ssh.FingerprintSHA256(key))

	// 查找公钥认证处理器
	for _, handler := range m.handlers {
		if handler.GetName() == "publickey" {
			// 检查该处理器是否可以处理该用户
			canHandle, err := handler.CanHandle(username)
			if err != nil {
				logger.Infof("[Auth Manager]  Handler check failed: %v", err)
				return nil, err
			}

			if !canHandle {
				logger.Infof("[Auth Manager]  PublicKey authentication NOT allowed for user %s (check user's auth_method setting)", username)
				// 不记录到 failedPubkeyAttempts，因为用户本来就不应该用公钥
				return nil, fmt.Errorf("publickey authentication is not allowed for this user")
			}

			logger.Infof("[Auth Manager]  PublicKey authentication allowed for user %s, proceeding with authentication...", username)

			// 尝试认证
			result, err := handler.Authenticate(username, key, clientIP)

			//  如果公钥认证失败，记录这个连接，阻止后续的密码尝试
			if err != nil || !result.Success {
				logger.Infof("[Auth Manager]  PublicKey authentication FAILED - marking connection to block password fallback")
				m.attemptsMu.Lock()
				m.failedPubkeyAttempts[clientIP] = true
				m.attemptsMu.Unlock()
			}

			return result, err
		}
	}

	return nil, fmt.Errorf("publickey handler not found")
}

// PasswordAuthCallback SSH密码认证回调
func (m *AuthManager) PasswordAuthCallback(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	result, err := m.AuthenticatePassword(c.User(), string(pass), c.RemoteAddr().String())
	if err != nil || !result.Success {
		return nil, fmt.Errorf("authentication failed")
	}

	return &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":  result.UserID,
			"username": c.User(),
		},
	}, nil
}

// PublicKeyAuthCallback SSH公钥认证回调
func (m *AuthManager) PublicKeyAuthCallback(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	result, err := m.AuthenticatePublicKey(c.User(), key, c.RemoteAddr().String())
	if err != nil || !result.Success {
		return nil, fmt.Errorf("authentication failed")
	}

	return &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":  result.UserID,
			"username": c.User(),
		},
	}, nil
}
