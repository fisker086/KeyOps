package auth

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/fisker086/keyops/pkg/logger"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/internal/sshserver/types"
	"golang.org/x/crypto/ssh"
)

// authAttempt 记录认证尝试信息
type authAttempt struct {
	method          string // "password" 或 "publickey"
	timestamp       time.Time
	rejected        bool // 是否因为 authMethod 不匹配而被拒绝
	shouldTerminate bool // 是否应该立即终止连接（用于公钥认证失败的情况）
	failed          bool // 是否为认证失败（密码错误等）
}

// ServiceAuthenticator 基于现有AuthService的认证器
type ServiceAuthenticator struct {
	authService *service.AuthService
	// 记录每个连接的认证尝试历史
	// key: remoteAddr (IP:Port)
	attempts   map[string][]authAttempt
	attemptsMu sync.RWMutex
}

// NewServiceAuthenticator 创建服务认证器
func NewServiceAuthenticator(authService *service.AuthService) types.Authenticator {
	auth := &ServiceAuthenticator{
		authService: authService,
		attempts:    make(map[string][]authAttempt),
	}

	// 启动清理goroutine，定期清理过期的认证记录（避免内存泄漏）
	go auth.cleanupExpiredAttempts()

	return auth
}

// cleanupExpiredAttempts 定期清理过期的认证记录
func (a *ServiceAuthenticator) cleanupExpiredAttempts() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		a.attemptsMu.Lock()
		now := time.Now()
		for addr, attempts := range a.attempts {
			// 删除 5 分钟前的记录
			if len(attempts) > 0 && now.Sub(attempts[len(attempts)-1].timestamp) > 5*time.Minute {
				delete(a.attempts, addr)
			}
		}
		a.attemptsMu.Unlock()
	}
}

// recordAttempt 记录认证尝试
func (a *ServiceAuthenticator) recordAttempt(clientIP, method string, rejected bool) {
	a.recordAttemptWithTerminate(clientIP, method, rejected, false, false)
}

// recordAttemptWithTerminate 记录认证尝试（带终止标志）
func (a *ServiceAuthenticator) recordAttemptWithTerminate(clientIP, method string, rejected, shouldTerminate, failed bool) {
	a.attemptsMu.Lock()
	defer a.attemptsMu.Unlock()

	a.attempts[clientIP] = append(a.attempts[clientIP], authAttempt{
		method:          method,
		timestamp:       time.Now(),
		rejected:        rejected,
		shouldTerminate: shouldTerminate,
		failed:          failed,
	})
}

// hasRejectedAttempt 检查是否有被拒绝的认证尝试
func (a *ServiceAuthenticator) hasRejectedAttempt(clientIP string) (bool, string) {
	a.attemptsMu.RLock()
	defer a.attemptsMu.RUnlock()

	attempts, exists := a.attempts[clientIP]
	if !exists {
		return false, ""
	}

	// 检查最近的认证尝试是否被拒绝
	for _, attempt := range attempts {
		if attempt.rejected {
			return true, attempt.method
		}
	}

	return false, ""
}

// ShouldTerminateConnection 检查是否应该立即终止连接（公开方法）
func (a *ServiceAuthenticator) ShouldTerminateConnection(clientIP string) bool {
	a.attemptsMu.RLock()
	defer a.attemptsMu.RUnlock()

	attempts, exists := a.attempts[clientIP]
	if !exists {
		return false
	}

	// 检查是否有需要立即终止连接的尝试
	for _, attempt := range attempts {
		if attempt.shouldTerminate {
			return true
		}
	}

	return false
}

// HasExceededMaxAttempts 检查是否超过最大尝试次数
func (a *ServiceAuthenticator) HasExceededMaxAttempts(clientIP string, maxAttempts int) bool {
	a.attemptsMu.RLock()
	defer a.attemptsMu.RUnlock()

	attempts, exists := a.attempts[clientIP]
	if !exists {
		return false
	}

	// 统计最近的失败尝试次数（包括认证失败和认证方式不匹配）
	failedCount := 0
	for _, attempt := range attempts {
		if attempt.failed || attempt.rejected {
			failedCount++
		}
	}

	return failedCount >= maxAttempts
}

// clearAttempts 清除指定连接的认证记录（认证成功后调用）
func (a *ServiceAuthenticator) clearAttempts(clientIP string) {
	a.attemptsMu.Lock()
	defer a.attemptsMu.Unlock()
	delete(a.attempts, clientIP)
}

// GetAuthService 获取认证服务（用于SSH服务器）
func (a *ServiceAuthenticator) GetAuthService() *service.AuthService {
	return a.authService
}

// AuthenticatePassword 密码认证
func (a *ServiceAuthenticator) AuthenticatePassword(username, password string, clientIP string) (*types.AuthResult, error) {
	logger.Infof("[SSH Auth] Attempting password authentication for user: %s from IP: %s", username, clientIP)

	// 首先检查是否超过最大尝试次数
	if a.HasExceededMaxAttempts(clientIP, 3) {
		logger.Infof("[SSH Auth] Password authentication BLOCKED - too many failed attempts from IP: %s", clientIP)
		return nil, fmt.Errorf("too many authentication failures")
	}

	// 检查是否应该立即终止（之前的公钥认证失败）
	if a.ShouldTerminateConnection(clientIP) {
		logger.Infof("[SSH Auth] Password authentication BLOCKED - previous publickey authentication failed")
		// 不进行任何验证，直接返回错误，让连接快速失败
		return nil, fmt.Errorf("authentication method not allowed")
	}

	// 先获取用户信息，检查认证方式
	user, err := a.authService.GetUserByUsername(username)
	if err != nil {
		logger.Infof("[SSH Auth] User not found: %s", username)
		// 记录失败尝试（用户不存在）
		a.recordAttemptWithTerminate(clientIP, "password", true, false, true)
		// 直接返回error，立即断开连接
		return nil, fmt.Errorf("user not found")
	}

	// 检查用户的认证方式是否允许密码登录
	if user.AuthMethod == "publickey" {
		logger.Infof("[SSH Auth] Password authentication REJECTED for user %s (authMethod: %s - only publickey allowed)",
			username, user.AuthMethod)
		logger.Infof("[SSH Auth] User only supports publickey authentication - password is disabled")
		// 记录被拒绝的尝试，并标记需要终止连接
		a.recordAttemptWithTerminate(clientIP, "password", true, true, true)
		// 返回一个强制断开连接的错误
		return nil, fmt.Errorf("authentication method not allowed")
	}

	// 用户配置了 password 认证方式
	logger.Infof("[SSH Auth] Password authentication ALLOWED for user %s (authMethod: %s)",
		username, user.AuthMethod)

	// 检查是否有之前被拒绝的公钥认证尝试
	// 如果有，清除它（因为用户的正确认证方式就是密码，之前的公钥拒绝是正常的）
	if hasRejected, rejectedMethod := a.hasRejectedAttempt(clientIP); hasRejected && rejectedMethod == "publickey" {
		logger.Infof("[SSH Auth] Clearing previous rejected %s attempt (user's correct auth method is password)",
			rejectedMethod)
		a.clearAttempts(clientIP)
	}

	// 认证方式允许，再验证密码
	loginReq := &model.LoginRequest{
		Username: username,
		Password: password,
	}

	loginResp, err := a.authService.Login(loginReq, clientIP, "SSH-Client")
	if err != nil {
		logger.Infof("[SSH Auth] Password verification failed for user %s: %v", username, err)
		// 记录失败尝试（密码错误）
		a.recordAttemptWithTerminate(clientIP, "password", false, false, true)
		// 直接返回error，不允许客户端尝试其他认证方法（比如公钥）
		return nil, fmt.Errorf("password verification failed: invalid password")
	}

	// 调试：检查MFA状态
	logger.Infof("[SSH Auth] Login response: RequiresTwoFactor=%v, TwoFactorEnabled=%v", loginResp.RequiresTwoFactor, loginResp.TwoFactorEnabled)

	// 检查是否需要MFA验证
	if loginResp.RequiresTwoFactor {
		logger.Infof("[SSH Auth] User %s requires 2FA verification (enabled: %v)", username, loginResp.TwoFactorEnabled)
		// 密码认证成功但需要MFA，返回特殊结果让Keyboard Interactive处理
		return &types.AuthResult{
			Success:           true,
			UserID:            loginResp.User.ID,
			Message:           "Password authentication successful, MFA required",
			RequiresTwoFactor: true,
		}, nil
	}

	logger.Infof("[SSH Auth] Password authentication SUCCESSFUL for user: %s (ID: %s, AuthMethod: %s)",
		username, loginResp.User.ID, loginResp.User.AuthMethod)

	// 认证成功，清除认证记录
	a.clearAttempts(clientIP)

	return &types.AuthResult{
		Success: true,
		UserID:  loginResp.User.ID,
		Message: "Authentication successful",
	}, nil
}

// AuthenticatePublicKey 公钥认证
func (a *ServiceAuthenticator) AuthenticatePublicKey(username string, key ssh.PublicKey, clientIP string) (*types.AuthResult, error) {
	logger.Infof("[SSH Auth] Attempting public key authentication for user: %s from IP: %s", username, clientIP)
	logger.Infof("[SSH Auth] Client key fingerprint: %s", ssh.FingerprintSHA256(key))

	// 首先检查是否超过最大尝试次数
	if a.HasExceededMaxAttempts(clientIP, 3) {
		logger.Infof("[SSH Auth] Public key authentication BLOCKED - too many failed attempts from IP: %s", clientIP)
		return nil, fmt.Errorf("too many authentication failures")
	}

	// 先获取用户信息，检查认证方式
	user, err := a.authService.GetUserByUsername(username)
	if err != nil {
		logger.Infof("[SSH Auth] User not found: %s", username)
		// 记录失败尝试（用户不存在）
		a.recordAttemptWithTerminate(clientIP, "publickey", true, false, true)
		// 直接返回error，立即断开连接
		return nil, fmt.Errorf("user not found")
	}

	// 检查用户的认证方式是否允许公钥登录
	if user.AuthMethod == "password" {
		logger.Infof("[SSH Auth] Public key authentication REJECTED for user %s (authMethod: %s - only password allowed)",
			username, user.AuthMethod)
		logger.Infof("[SSH Auth] User only supports password authentication - publickey is disabled")
		// 记录被拒绝的尝试
		a.recordAttemptWithTerminate(clientIP, "publickey", true, false, true)
		// 直接返回error，告诉客户端公钥认证不可用
		return nil, fmt.Errorf("public key authentication is disabled for this user, only password authentication is allowed")
	}

	logger.Infof("[SSH Auth] Public key authentication ALLOWED for user %s (authMethod: %s)",
		username, user.AuthMethod)

	// 认证方式允许，获取用户的公钥
	userPublicKeyStr, err := a.authService.GetUserPublicKey(username)
	if err != nil {
		logger.Infof("[SSH Auth] Failed to get user public key: %v", err)
		// 记录失败尝试（用户配置了公钥认证但没有公钥，阻止后续 fallback 到密码）
		a.recordAttemptWithTerminate(clientIP, "publickey", true, false, true)
		// 直接返回error，不允许客户端尝试其他认证方法
		return nil, fmt.Errorf("failed to get user public key: %w", err)
	}

	// 4. 解析用户的公钥
	userPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(userPublicKeyStr))
	if err != nil {
		logger.Infof("[SSH Auth] Failed to parse user public key: %v", err)
		// 记录失败尝试（公钥格式错误，阻止后续 fallback 到密码）
		a.recordAttemptWithTerminate(clientIP, "publickey", true, false, true)
		// 直接返回error，不允许客户端尝试其他认证方法
		return nil, fmt.Errorf("invalid public key format: %w", err)
	}

	logger.Infof("[SSH Auth] User stored key fingerprint: %s", ssh.FingerprintSHA256(userPublicKey))

	// 5. 比对公钥
	if !bytes.Equal(key.Marshal(), userPublicKey.Marshal()) {
		logger.Infof("[SSH Auth] Public key mismatch for user: %s", username)
		logger.Infof("[SSH Auth]   Client fingerprint: %s", ssh.FingerprintSHA256(key))
		logger.Infof("[SSH Auth]   Server fingerprint: %s", ssh.FingerprintSHA256(userPublicKey))
		logger.Infof("[SSH Auth] Public key authentication failed - connection will be terminated if password is attempted")
		// 记录失败尝试（用户配置了公钥但不匹配，标记需要终止后续的密码尝试）
		a.recordAttemptWithTerminate(clientIP, "publickey", true, true, true)
		// 直接返回error，不允许客户端尝试其他认证方法（比如密码）
		return nil, fmt.Errorf("public key does not match")
	}

	// 6. 检查用户状态（user在第1步已经获取）
	if user.Status != "active" {
		logger.Infof("[SSH Auth] User account is not active: %s", username)
		// 记录失败尝试（账号不活跃，阻止后续 fallback 到密码）
		a.recordAttemptWithTerminate(clientIP, "publickey", true, false, true)
		// 直接返回error，不允许客户端尝试其他认证方法
		return nil, fmt.Errorf("user account is not active")
	}

	// 检查是否需要MFA验证
	if user.TwoFactorEnabled {
		logger.Infof("[SSH Auth] User %s requires 2FA verification after public key auth (enabled: %v)", username, user.TwoFactorEnabled)
		// 公钥认证成功但需要MFA，返回特殊结果让Keyboard Interactive处理
		return &types.AuthResult{
			Success:           true,
			UserID:            user.ID,
			Message:           "Public key authentication successful, MFA required",
			RequiresTwoFactor: true,
		}, nil
	}

	logger.Infof("[SSH Auth] Public key authentication SUCCESSFUL for user: %s (ID: %s, AuthMethod: %s)",
		username, user.ID, user.AuthMethod)

	// 认证成功，清除认证记录
	a.clearAttempts(clientIP)

	return &types.AuthResult{
		Success: true,
		UserID:  user.ID,
		Message: "Authentication successful",
	}, nil
}

// PasswordAuthCallback SSH密码认证回调
func (a *ServiceAuthenticator) PasswordAuthCallback(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	result, err := a.AuthenticatePassword(c.User(), string(pass), c.RemoteAddr().String())
	if err != nil || !result.Success {
		return nil, fmt.Errorf("authentication failed")
	}

	// 在Permissions中存储用户信息
	return &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":  result.UserID,
			"username": c.User(),
		},
	}, nil
}

// PublicKeyAuthCallback SSH公钥认证回调
func (a *ServiceAuthenticator) PublicKeyAuthCallback(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	result, err := a.AuthenticatePublicKey(c.User(), key, c.RemoteAddr().String())
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
