package protocol

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// SSHHandler SSH 协议处理器
type SSHHandler struct {
	config      *ConnectionConfig
	sessionInfo *SessionInfo
	recorder    SessionRecorder
	sshClient   *ssh.Client
	sshSession  *ssh.Session
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	connected   bool
}

// NewSSHHandler 创建 SSH 处理器
func NewSSHHandler(recorder SessionRecorder) ProtocolHandler {
	return &SSHHandler{
		recorder: recorder,
	}
}

// GetProtocolType 获取协议类型
func (h *SSHHandler) GetProtocolType() ProtocolType {
	return ProtocolSSH
}

// Connect 连接到 SSH 服务器
func (h *SSHHandler) Connect(ctx context.Context, config *ConnectionConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = config
	h.ctx, h.cancel = context.WithCancel(ctx)

	// 初始化会话信息
	h.sessionInfo = &SessionInfo{
		SessionID: config.SessionID,
		ProxyID:   config.ProxyID,
		UserID:    config.UserID,
		Username:  config.Username,
		HostID:    config.HostID,
		HostIP:    config.HostIP,
		HostPort:  config.HostPort,
		Protocol:  ProtocolSSH,
		Status:    "connecting",
		StartTime: time.Now(),
	}

	// 配置 SSH 客户端
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该验证
		Timeout:         config.Timeout,
	}

	// 添加认证方式
	if config.Password != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.Password))
	}

	if config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	}

	// 连接到 SSH 服务器
	addr := fmt.Sprintf("%s:%d", config.HostIP, config.HostPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		h.sessionInfo.Status = "error"
		if h.recorder != nil {
			h.recorder.RecordError(config.SessionID, err.Error())
		}
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	h.sshClient = client
	h.connected = true
	h.sessionInfo.Status = "active"

	// 记录会话开始
	if h.recorder != nil {
		h.recorder.RecordStart(h.sessionInfo)
	}

	log.Printf("[SSH] Connected to %s:%d as %s (session: %s)",
		config.HostIP, config.HostPort, config.Username, config.SessionID)

	return nil
}

// HandleWebSocket 处理 WebSocket 连接
func (h *SSHHandler) HandleWebSocket(ws *websocket.Conn) error {
	if !h.IsAlive() {
		return fmt.Errorf("SSH connection not established")
	}

	// 创建 SSH 会话
	session, err := h.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	h.mu.Lock()
	h.sshSession = session
	h.mu.Unlock()

	// 设置终端模式
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// 请求 PTY
	if err := session.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	// 获取会话的输入输出
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr: %w", err)
	}

	// 启动 shell
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// WebSocket -> SSH (客户端输入)
	go func() {
		defer stdin.Close()
		for {
			messageType, data, err := ws.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[SSH] WebSocket read error: %v", err)
				}
				return
			}

			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				// 处理特殊消息（如终端大小调整）
				if len(data) > 0 && data[0] == '{' {
					// 可能是 JSON 控制消息
					h.handleControlMessage(data)
					continue
				}

				// 发送到 SSH
				if _, err := stdin.Write(data); err != nil {
					log.Printf("[SSH] Failed to write to SSH stdin: %v", err)
					return
				}

				// 记录输入数据
				h.sessionInfo.BytesIn += int64(len(data))
				if h.recorder != nil {
					h.recorder.RecordData(h.config.SessionID, "in", data)
				}
			}
		}
	}()

	// SSH -> WebSocket (服务器输出)
	done := make(chan struct{})
	var wg sync.WaitGroup

	// 处理 stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				data := buf[:n]
				if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
					log.Printf("[SSH] Failed to write to WebSocket: %v", err)
					return
				}

				// 记录输出数据
				h.sessionInfo.BytesOut += int64(n)
				if h.recorder != nil {
					h.recorder.RecordData(h.config.SessionID, "out", data)
				}
			}

			if err != nil {
				if err != io.EOF {
					log.Printf("[SSH] stdout read error: %v", err)
				}
				return
			}
		}
	}()

	// 处理 stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				data := buf[:n]
				if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
					log.Printf("[SSH] Failed to write stderr to WebSocket: %v", err)
					return
				}

				h.sessionInfo.BytesOut += int64(n)
				if h.recorder != nil {
					h.recorder.RecordData(h.config.SessionID, "out", data)
				}
			}

			if err != nil {
				if err != io.EOF {
					log.Printf("[SSH] stderr read error: %v", err)
				}
				return
			}
		}
	}()

	// 等待会话结束
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待会话结束或上下文取消
	select {
	case <-done:
		log.Printf("[SSH] Session completed (session: %s)", h.config.SessionID)
	case <-h.ctx.Done():
		log.Printf("[SSH] Session cancelled (session: %s)", h.config.SessionID)
	}

	return session.Wait()
}

// handleControlMessage 处理控制消息（如终端大小调整）
func (h *SSHHandler) handleControlMessage(data []byte) {
	// 这里可以解析 JSON 消息来处理终端大小调整等控制命令
	// 简化实现，实际可以更复杂
	log.Printf("[SSH] Received control message: %s", string(data))
}

// Resize 调整终端大小
func (h *SSHHandler) Resize(width, height int) error {
	h.mu.RLock()
	session := h.sshSession
	h.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("no active SSH session")
	}

	return session.WindowChange(height, width)
}

// Close 关闭连接
func (h *SSHHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel != nil {
		h.cancel()
	}

	var err error
	if h.sshSession != nil {
		err = h.sshSession.Close()
		h.sshSession = nil
	}

	if h.sshClient != nil {
		if closeErr := h.sshClient.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		h.sshClient = nil
	}

	h.connected = false
	h.sessionInfo.Status = "closed"
	now := time.Now()
	h.sessionInfo.EndTime = &now

	// 记录会话结束
	if h.recorder != nil {
		h.recorder.RecordEnd(h.config.SessionID, now)
	}

	log.Printf("[SSH] Connection closed (session: %s, bytes in: %d, bytes out: %d)",
		h.config.SessionID, h.sessionInfo.BytesIn, h.sessionInfo.BytesOut)

	return err
}

// GetSessionInfo 获取会话信息
func (h *SSHHandler) GetSessionInfo() *SessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionInfo
}

// IsAlive 检查连接是否存活
func (h *SSHHandler) IsAlive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected && h.ctx != nil && h.ctx.Err() == nil
}

// 注册 SSH 处理器到工厂
func init() {
	GetFactory().Register(ProtocolSSH, NewSSHHandler)
}
