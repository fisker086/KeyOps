package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service"
	"github.com/fisker086/keyops/internal/sshserver/auth"
	"github.com/fisker086/keyops/internal/sshserver/types"
	"github.com/fisker086/keyops/pkg/sshkey"
	"gorm.io/gorm"
)

// sshContext 适配器，将gossh.ConnMetadata转换为ssh.Context
type sshContext struct {
	conn gossh.ConnMetadata
	ctx  context.Context
	mu   sync.Mutex
}

func (c *sshContext) User() string {
	return c.conn.User()
}

func (c *sshContext) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *sshContext) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *sshContext) ClientVersion() string {
	return string(c.conn.ClientVersion())
}

func (c *sshContext) ServerVersion() string {
	return string(c.conn.ServerVersion())
}

func (c *sshContext) SessionID() string {
	return string(c.conn.SessionID())
}

func (c *sshContext) Permissions() *ssh.Permissions {
	return &ssh.Permissions{}
}

// context.Context 接口实现
func (c *sshContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *sshContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *sshContext) Err() error {
	return c.ctx.Err()
}

func (c *sshContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// sync.Locker 接口实现
func (c *sshContext) Lock() {
	c.mu.Lock()
}

func (c *sshContext) Unlock() {
	c.mu.Unlock()
}

func (c *sshContext) SetValue(key, value interface{}) {
	// 简单的实现，实际应该使用context.WithValue
}

// Config SSH服务器配置
type Config struct {
	ListenAddress    string        // 监听地址，如 ":2222"
	MaxSessions      int           // 最大并发会话数
	SessionTimeout   time.Duration // 会话超时时间
	IdleTimeout      time.Duration // 空闲超时时间
	BannerMessage    string        // 欢迎横幅
	ServerVersion    string        // 服务器版本
	HostKeyPath      string        // 主机密钥路径（可选，用于本地存储备用）
	EnablePublicKey  bool          // 是否启用公钥认证
	EnablePassword   bool          // 是否启用密码认证
	DB               *gorm.DB      // 数据库连接（用于共享密钥）
	UseSharedHostKey bool          // 是否使用数据库共享密钥（多实例部署推荐）
}

// Server SSH服务器
type Server struct {
	config          *Config
	authenticator   types.Authenticator
	terminalHandler types.TerminalHandler
	sessions        map[string]*Session
	sessionsMu      sync.RWMutex
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	server          *ssh.Server
	// 认证状态管理
	authStates   map[string]*AuthState
	authStatesMu sync.RWMutex
	// 数据库连接和仓库
	db          *gorm.DB
	settingRepo *repository.SettingRepository
}

// AuthState 认证状态
type AuthState struct {
	Username      string
	PasswordValid bool
	RequiresMFA   bool
	MFAVerified   bool
	LastActivity  time.Time
}

// Session SSH会话
type Session struct {
	ID          string
	SessionInfo *types.SessionInfo
	StartTime   time.Time
	LastActive  time.Time
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewServer 创建SSH服务器
func NewServer(
	config *Config,
	authenticator types.Authenticator,
	terminalHandler types.TerminalHandler,
) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:          config,
		authenticator:   authenticator,
		terminalHandler: terminalHandler,
		sessions:        make(map[string]*Session),
		ctx:             ctx,
		cancel:          cancel,
		db:              config.DB,
		settingRepo:     repository.NewSettingRepository(config.DB),
	}

	// 配置SSH服务器
	if err := server.setupSSHConfig(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup SSH config: %w", err)
	}

	return server, nil
}

// setupSSHConfig 配置SSH服务器
func (s *Server) setupSSHConfig() error {
	// 创建gliderlabs/ssh服务器
	s.server = &ssh.Server{
		Addr: s.config.ListenAddress,
		Banner: func() string {
			if s.config.BannerMessage != "" {
				return s.config.BannerMessage
			}
			return "Welcome to ZJump SSH Gateway\n"
		}(),
		// 使用密码与公钥认证，避免Keyboard Interactive的客户端提示问题
		PasswordHandler:  s.handlePasswordAuth,
		PublicKeyHandler: s.handlePublicKeyAuth,
		ServerConfigCallback: func(ctx ssh.Context) *gossh.ServerConfig {
			config := &gossh.ServerConfig{
				ServerVersion: s.config.ServerVersion,
				NoClientAuth:  false,
			}

			// 添加密码认证回调
			config.PasswordCallback = func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
				// 获取认证服务
				authService := s.getAuthService()
				if authService == nil {
					return nil, fmt.Errorf("authentication service not available")
				}

				// 获取用户信息
				user, err := authService.GetUserByUsername(conn.User())
				if err != nil {
					return nil, fmt.Errorf("user not found")
				}

				// 验证密码
				if err := authService.ValidatePassword(user, string(password)); err != nil {
					return nil, fmt.Errorf("invalid password")
				}

				// 密码验证成功，返回权限信息
				return &gossh.Permissions{
					Extensions: map[string]string{
						"user_id":  user.ID,
						"username": conn.User(),
					},
				}, nil
			}

			// 添加公钥认证回调
			config.PublicKeyCallback = func(conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
				// 使用统一的认证器验证公钥
				result, err := s.authenticator.AuthenticatePublicKey(conn.User(), key, conn.RemoteAddr().String())
				if err != nil || result == nil || !result.Success {
					return nil, fmt.Errorf("authentication failed")
				}

				return &gossh.Permissions{
					Extensions: map[string]string{
						"user_id":  result.UserID,
						"username": conn.User(),
					},
				}, nil
			}

			return config
		},
	}

	// 加载或生成主机密钥
	if err := s.setupHostKey(); err != nil {
		return fmt.Errorf("failed to setup host key: %w", err)
	}

	return nil
}

// handlePublicKeyAuth 处理公钥认证
func (s *Server) handlePublicKeyAuth(ctx ssh.Context, key ssh.PublicKey) bool {
	// 获取认证服务
	authService := s.getAuthService()
	if authService == nil {
		return false
	}

	// 获取用户信息
	user, err := authService.GetUserByUsername(ctx.User())
	if err != nil {
		return false
	}

	// 检查用户认证方式是否允许公钥认证
	if user.AuthMethod == "password" {
		return false
	}

	// 使用认证器验证公钥
	result, err := s.authenticator.AuthenticatePublicKey(ctx.User(), key, ctx.RemoteAddr().String())
	if err != nil || !result.Success {
		return false
	}

	// 公钥认证成功，MFA 在会话阶段进行
	return true
}

// handlePasswordAuth 处理密码认证
func (s *Server) handlePasswordAuth(ctx ssh.Context, password string) bool {
	// 获取认证服务
	authService := s.getAuthService()
	if authService == nil {
		return false
	}

	// 获取用户信息
	user, err := authService.GetUserByUsername(ctx.User())
	if err != nil {
		return false
	}

	// 验证密码
	if err := authService.ValidatePassword(user, password); err != nil {
		return false
	}

	// 密码验证成功，无论是否需要MFA都返回true
	// MFA验证将在会话建立后进行
	return true
}

// getClientIP 获取客户端IP地址
func (s *Server) getClientIP(ctx ssh.Context) string {
	// 从SSH连接元数据中获取远程地址
	remoteAddr := ctx.RemoteAddr().String()

	// 如果地址包含端口，去掉端口部分
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}

	// 如果解析失败，返回原始地址
	return remoteAddr
}

// getAuthService 获取认证服务（支持不同类型的认证器）
func (s *Server) getAuthService() *service.AuthService {
	// 尝试从 AuthManager 获取
	if authManager, ok := s.authenticator.(*auth.AuthManager); ok {
		return authManager.GetAuthService()
	}

	// 尝试从 ServiceAuthenticator 获取
	if serviceAuth, ok := s.authenticator.(*auth.ServiceAuthenticator); ok {
		return serviceAuth.GetAuthService()
	}

	return nil
}

// setupHostKey 设置主机密钥
func (s *Server) setupHostKey() error {
	// 优先级1: 如果启用了数据库共享密钥，从数据库加载（多实例部署推荐）
	if s.config.UseSharedHostKey && s.config.DB != nil {
		signer, err := sshkey.GetOrGenerateSharedHostKey(s.config.DB, "rsa", "default")
		if err != nil {
			// 如果数据库加载失败，尝试从文件加载
			if s.config.HostKeyPath != "" {
				if err := s.loadOrGenerateHostKey(s.config.HostKeyPath); err != nil {
					return fmt.Errorf("failed to get shared host key from database and no fallback path configured: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get shared host key from database and no fallback path configured: %w", err)
			}
		} else {
			_ = gossh.FingerprintSHA256(signer.PublicKey())
			s.server.AddHostKey(signer)
			return nil
		}
	}

	// 优先级2: 从文件加载或生成
	if s.config.HostKeyPath != "" {
		return s.loadOrGenerateHostKey(s.config.HostKeyPath)
	}

	// 优先级3: 如果没有提供路径，生成临时RSA密钥（不推荐）
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	s.server.AddHostKey(signer)

	return nil
}

// loadOrGenerateHostKey 从文件加载或生成新的持久化密钥
func (s *Server) loadOrGenerateHostKey(path string) error {
	// 尝试从文件加载
	if _, err := os.Stat(path); err == nil {
		privateKeyBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read host key file %s: %w", path, err)
		}

		signer, err := gossh.ParsePrivateKey(privateKeyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key from %s: %w", path, err)
		}

		// 获取公钥指纹用于日志
		_ = gossh.FingerprintSHA256(signer.PublicKey())

		s.server.AddHostKey(signer)
		return nil
	}

	// 生成RSA密钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// 转换为SSH密钥格式
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// 保存到文件（权限 0600 - 只有owner可读写）
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, privateKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to save host key to %s: %w", err)
	}

	signer, err := gossh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// 获取公钥指纹
	_ = gossh.FingerprintSHA256(signer.PublicKey())

	s.server.AddHostKey(signer)

	return nil
}

// Start 启动SSH服务器
func (s *Server) Start() error {
	// 启动会话监控
	go s.monitorSessions()

	// 使用gliderlabs/ssh启动服务器
	s.server.Handler = s.handleSSHSession
	return s.server.ListenAndServe()
}

// handleSSHSession 处理SSH会话
func (s *Server) handleSSHSession(sess ssh.Session) {
	defer func() {
		// Recover from any panic in this session
		if r := recover(); r != nil {
			logger.Infof("[SSH Server] Panic in handleSSHSession: %v", r)
		}
	}()

	// 获取用户ID
	userID := ""
	permissions := sess.Permissions()
	if permissions.Permissions != nil && permissions.Permissions.Extensions != nil {
		if userIDExt := permissions.Permissions.Extensions["user_id"]; userIDExt != "" {
			userID = userIDExt
		}
	}

	// 如果无法从权限中获取用户ID，尝试从认证服务获取
	if userID == "" {
		authService := s.getAuthService()
		if authService != nil {
			if user, err := authService.GetUserByUsername(sess.User()); err == nil {
				userID = user.ID
			}
		}
	}

	// 创建会话信息
	sessionID := uuid.New().String()
	sessionCtx, sessionCancel := context.WithCancel(s.ctx)

	session := &Session{
		ID: sessionID,
		SessionInfo: &types.SessionInfo{
			SessionID:    sessionID,
			UserID:       userID,
			Username:     sess.User(),
			ClientIP:     sess.RemoteAddr().String(),
			StartTime:    time.Now(),
			TerminalCols: 80,
			TerminalRows: 24,
		},
		ctx:    sessionCtx,
		cancel: sessionCancel,
	}

	// 注册会话
	s.registerSession(session)
	defer s.unregisterSession(sessionID)

	// 检查是否需要MFA验证
	authService := s.getAuthService()
	if authService != nil {
		user, err := authService.GetUserByUsername(sess.User())
		if err == nil {
			// 检查全局MFA配置
			var globalConfig model.TwoFactorConfig
			if err := s.db.First(&globalConfig).Error; err == nil && globalConfig.Enabled {
				// 全局MFA已启用
				if !user.TwoFactorEnabled {
					// 用户未设置MFA，拒绝登录并要求先设置MFA
					sess.Write([]byte("\n"))
					sess.Write([]byte("⚠️  Global MFA is enabled. You must set up MFA before accessing SSH.\n"))
					sess.Write([]byte("Please log in to the web interface and configure MFA first.\n"))
					logger.Infof("[SSH Server] User %s denied SSH access - global MFA enabled but user MFA not set up", sess.User())
					return
				}
			}

			// 检查用户个人MFA设置
			if user.TwoFactorEnabled {
				// 需要进行MFA验证
				if !s.performSessionMFA(sess, authService, user) {
					// MFA验证失败，关闭会话
					logger.Infof("[SSH Server] MFA verification failed for user: %s", sess.User())
					return
				}
			}
		}
	}

	// 处理终端会话
	s.handleTerminalSession(sess, session)
}

// performSessionMFA 在会话中进行MFA验证
func (s *Server) performSessionMFA(sess ssh.Session, authService *service.AuthService, user *model.User) bool {
	const maxMfaAttempts = 3

	for attempt := 1; attempt <= maxMfaAttempts; attempt++ {
		// 发送MFA提示
		sess.Write([]byte("Please Enter MFA Code: "))

		// 读取用户输入（逐字符读取直到回车）
		var input strings.Builder
		for {
			buf := make([]byte, 1)
			n, err := sess.Read(buf)
			if err != nil {
				logger.Infof("[SSH Server] Failed to read MFA input: %v", err)
				return false
			}

			char := buf[0]
			// 如果是回车或换行，结束输入
			if char == '\r' || char == '\n' {
				break
			}

			// 回显字符（明文显示）
			sess.Write(buf[:n])
			input.WriteByte(char)
		}

		// 换行
		sess.Write([]byte("\n"))

		code := strings.TrimSpace(input.String())
		if code == "" {
			if attempt < maxMfaAttempts {
				continue
			}
			return false
		}

		// 验证2FA代码
		if authService.ValidateTwoFactorCode(user, code, "") {
			return true
		}

		// 验证失败
		if attempt < maxMfaAttempts {
			sess.Write([]byte("Invalid MFA code. Please try again.\n"))
		}
	}

	// 所有尝试都失败
	sess.Write([]byte("MFA verification failed. Connection closed.\n"))
	return false
}

// handleTerminalSession 处理终端会话
func (s *Server) handleTerminalSession(sess ssh.Session, session *Session) {
	// 调用终端处理器
	if err := s.terminalHandler.HandleTerminal(session.ctx, sess, session.SessionInfo); err != nil {
		logger.Infof("[SSH Server] Terminal handler error: %v", err)
	}
}

// Stop 停止SSH服务器
func (s *Server) Stop() error {
	// 1. 先取消上下文，通知所有goroutine开始关闭流程
	s.cancel()

	// 2. 关闭gliderlabs/ssh服务器
	if s.server != nil {
		return s.server.Close()
	}

	// 3. 主动关闭所有活动的SSH会话
	s.closeAllSessions()

	// 4. 等待所有goroutine完成（设置超时）
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	timeout := time.After(30 * time.Second)
	select {
	case <-done:
	case <-timeout:
		// 超时后强制关闭
		s.forceCloseAllSessions()
	}

	return nil
}

// registerSession 注册会话
func (s *Server) registerSession(session *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[session.ID] = session
}

// unregisterSession 注销会话
func (s *Server) unregisterSession(sessionID string) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if session, exists := s.sessions[sessionID]; exists {
		session.cancel()
		delete(s.sessions, sessionID)
	}
}

// getSessionCount 获取当前会话数
func (s *Server) getSessionCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return len(s.sessions)
}

// monitorSessions 监控会话
func (s *Server) monitorSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions 清理过期会话
func (s *Server) cleanupExpiredSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	now := time.Now()
	for sessionID, session := range s.sessions {
		// 检查会话超时（保留，防止会话无限期运行）
		if s.config.SessionTimeout > 0 && now.Sub(session.StartTime) > s.config.SessionTimeout {
			logger.Infof("[SSHServer] Closing timed out session: %s", sessionID)
			session.cancel()
			delete(s.sessions, sessionID)
		}
	}
}

// closeAllSessions 关闭所有会话
func (s *Server) closeAllSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	for _, session := range s.sessions {
		session.cancel()
	}
}

// forceCloseAllSessions 强制关闭所有会话
func (s *Server) forceCloseAllSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	for _, session := range s.sessions {
		session.cancel()
	}
}
