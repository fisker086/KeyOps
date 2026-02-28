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

// AuthHandler 认证处理器接口
type AuthHandler interface {
	// GetName 获取处理器名称
	GetName() string

	// CanHandle 判断是否可以处理该用户的认证
	CanHandle(username string) (bool, error)

	// Authenticate 执行认证
	Authenticate(username string, credential interface{}, clientIP string) (*types.AuthResult, error)
}

// PasswordHandler 密码认证处理器
type PasswordHandler struct {
	authService *service.AuthService
	// 记录每个连接的认证尝试历史
	attempts   map[string][]authAttempt
	attemptsMu sync.RWMutex
}

// NewPasswordHandler 创建密码认证处理器
func NewPasswordHandler(authService *service.AuthService) *PasswordHandler {
	return &PasswordHandler{
		authService: authService,
		attempts:    make(map[string][]authAttempt),
	}
}

func (h *PasswordHandler) GetName() string {
	return "password"
}

// HasExceededMaxAttempts 检查是否超过最大尝试次数
func (h *PasswordHandler) HasExceededMaxAttempts(clientIP string, maxAttempts int) bool {
	h.attemptsMu.RLock()
	defer h.attemptsMu.RUnlock()

	attempts, exists := h.attempts[clientIP]
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

// recordAttempt 记录认证尝试
func (h *PasswordHandler) recordAttempt(clientIP, method string, rejected, shouldTerminate, failed bool) {
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	h.attempts[clientIP] = append(h.attempts[clientIP], authAttempt{
		method:          method,
		timestamp:       time.Now(),
		rejected:        rejected,
		shouldTerminate: shouldTerminate,
		failed:          failed,
	})
}

func (h *PasswordHandler) CanHandle(username string) (bool, error) {
	user, err := h.authService.GetUserByUsername(username)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	// 只有配置了 password 或 all 认证方式的用户才能使用密码认证
	return user.AuthMethod == "password" || user.AuthMethod == "all", nil
}

func (h *PasswordHandler) Authenticate(username string, credential interface{}, clientIP string) (*types.AuthResult, error) {
	password, ok := credential.(string)
	if !ok {
		return nil, fmt.Errorf("invalid credential type for password authentication")
	}

	logger.Infof("[Password Handler] Authenticating user: %s from IP: %s", username, clientIP)

	// 首先检查是否超过最大尝试次数
	if h.HasExceededMaxAttempts(clientIP, 3) {
		logger.Infof("[Password Handler] Authentication BLOCKED - too many failed attempts from IP: %s", clientIP)
		return nil, fmt.Errorf("too many authentication failures")
	}

	loginReq := &model.LoginRequest{
		Username: username,
		Password: password,
	}

	loginResp, err := h.authService.Login(loginReq, clientIP, "SSH-Client")
	if err != nil {
		logger.Infof("[Password Handler] Authentication failed for user %s: %v", username, err)
		// 记录失败尝试
		h.recordAttempt(clientIP, "password", false, false, true)
		return nil, fmt.Errorf("password authentication failed")
	}

	// 调试：检查MFA状态
	logger.Infof("[Password Handler] Login response: RequiresTwoFactor=%v, TwoFactorEnabled=%v", loginResp.RequiresTwoFactor, loginResp.TwoFactorEnabled)

	// 检查是否需要MFA验证
	if loginResp.RequiresTwoFactor {
		logger.Infof("[Password Handler] User %s requires 2FA verification (enabled: %v)", username, loginResp.TwoFactorEnabled)
		// 返回特殊结果，表示需要MFA验证
		return &types.AuthResult{
			Success:           false,
			UserID:            loginResp.User.ID,
			Message:           "2FA verification required",
			RequiresTwoFactor: true,
		}, nil
	}

	logger.Infof("[Password Handler] Authentication successful for user: %s (ID: %s)", username, loginResp.User.ID)

	return &types.AuthResult{
		Success: true,
		UserID:  loginResp.User.ID,
		Message: "Password authentication successful",
	}, nil
}

// PublicKeyHandler 公钥认证处理器
type PublicKeyHandler struct {
	authService *service.AuthService
}

// NewPublicKeyHandler 创建公钥认证处理器
func NewPublicKeyHandler(authService *service.AuthService) *PublicKeyHandler {
	return &PublicKeyHandler{
		authService: authService,
	}
}

func (h *PublicKeyHandler) GetName() string {
	return "publickey"
}

func (h *PublicKeyHandler) CanHandle(username string) (bool, error) {
	user, err := h.authService.GetUserByUsername(username)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	// 只有配置了 publickey 或 all 认证方式的用户才能使用公钥认证
	return user.AuthMethod == "publickey" || user.AuthMethod == "all", nil
}

func (h *PublicKeyHandler) Authenticate(username string, credential interface{}, clientIP string) (*types.AuthResult, error) {
	publicKey, ok := credential.(ssh.PublicKey)
	if !ok {
		return nil, fmt.Errorf("invalid credential type for publickey authentication")
	}

	logger.Infof("[PublicKey Handler] Authenticating user: %s from IP: %s", username, clientIP)
	logger.Infof("[PublicKey Handler] Client key fingerprint: %s", ssh.FingerprintSHA256(publicKey))

	// 获取用户的公钥
	user, err := h.authService.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	userPublicKeyStr, err := h.authService.GetUserPublicKey(username)
	if err != nil {
		logger.Infof("[PublicKey Handler] Failed to get user public key: %v", err)
		return nil, fmt.Errorf("failed to get user public key")
	}

	// 解析用户的公钥
	userPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(userPublicKeyStr))
	if err != nil {
		logger.Infof("[PublicKey Handler] Failed to parse user public key: %v", err)
		return nil, fmt.Errorf("invalid public key format")
	}

	logger.Infof("[PublicKey Handler] User stored key fingerprint: %s", ssh.FingerprintSHA256(userPublicKey))

	// 比对公钥
	if !bytes.Equal(publicKey.Marshal(), userPublicKey.Marshal()) {
		logger.Infof("[PublicKey Handler] Public key mismatch for user: %s", username)
		logger.Infof("[PublicKey Handler]   Client fingerprint: %s", ssh.FingerprintSHA256(publicKey))
		logger.Infof("[PublicKey Handler]   Server fingerprint: %s", ssh.FingerprintSHA256(userPublicKey))
		return nil, fmt.Errorf("public key does not match")
	}

	// 检查用户状态
	if user.Status != "active" {
		logger.Infof("[PublicKey Handler] User account is not active: %s", username)
		return nil, fmt.Errorf("user account is not active")
	}

	logger.Infof("[PublicKey Handler] Authentication successful for user: %s (ID: %s)", username, user.ID)

	return &types.AuthResult{
		Success: true,
		UserID:  user.ID,
		Message: "PublicKey authentication successful",
	}, nil
}

// MFAHandler MFA 多因素认证处理器（预留接口）
type MFAHandler struct {
	authService *service.AuthService
	// 可以添加 TOTP、短信验证等配置
}

// NewMFAHandler 创建 MFA 认证处理器
func NewMFAHandler(authService *service.AuthService) *MFAHandler {
	return &MFAHandler{
		authService: authService,
	}
}

func (h *MFAHandler) GetName() string {
	return "mfa"
}

func (h *MFAHandler) CanHandle(username string) (bool, error) {
	user, err := h.authService.GetUserByUsername(username)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	// 检查用户是否启用了 MFA
	return user.TwoFactorEnabled, nil
}

func (h *MFAHandler) Authenticate(username string, credential interface{}, clientIP string) (*types.AuthResult, error) {
	logger.Infof("[MFA Handler] Authenticating user: %s from IP: %s", username, clientIP)

	// 获取用户信息
	user, err := h.authService.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// 检查用户是否启用了MFA
	if !user.TwoFactorEnabled {
		return nil, fmt.Errorf("MFA not enabled for user")
	}

	// 解析MFA凭证（TOTP代码或备用码）
	var totpCode, backupCode string
	if code, ok := credential.(string); ok {
		// 简单判断：6位数字为TOTP，其他为备用码
		if len(code) == 6 && isNumeric(code) {
			totpCode = code
		} else {
			backupCode = code
		}
	} else {
		return nil, fmt.Errorf("invalid MFA credential type")
	}

	// 验证MFA代码
	loginReq := &model.LoginRequest{
		Username:      username,
		TwoFactorCode: totpCode,
		BackupCode:    backupCode,
	}

	loginResp, err := h.authService.Login(loginReq, clientIP, "SSH-MFA")
	if err != nil {
		logger.Infof("[MFA Handler] MFA verification failed for user %s: %v", username, err)
		return nil, fmt.Errorf("MFA verification failed: %w", err)
	}

	logger.Infof("[MFA Handler] MFA authentication successful for user: %s (ID: %s)", username, loginResp.User.ID)

	return &types.AuthResult{
		Success: true,
		UserID:  loginResp.User.ID,
		Message: "MFA authentication successful",
	}, nil
}

// isNumeric 检查字符串是否为纯数字
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
