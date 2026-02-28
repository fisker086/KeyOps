package websocket

import (
	"context"
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/bastion/protocol"
	"github.com/fisker086/keyops/internal/bastion/recorder"
	"github.com/fisker086/keyops/internal/bastion/storage"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ProtocolHandler 统一协议处理器
type ProtocolHandler struct {
	hostRepo       *repository.HostRepository
	storage        storage.Storage
	proxyID        string
	sessionManager *SessionManager
	factory        *protocol.Factory
}

// NewProtocolHandler 创建统一协议处理器
func NewProtocolHandler(hostRepo *repository.HostRepository, st storage.Storage, proxyID string, sm *SessionManager) *ProtocolHandler {
	return &ProtocolHandler{
		hostRepo:       hostRepo,
		storage:        st,
		proxyID:        proxyID,
		sessionManager: sm,
		factory:        protocol.GetFactory(),
	}
}

// HandleConnection 处理各种协议的连接
func (h *ProtocolHandler) HandleConnection(c *gin.Context) {
	// 获取协议类型
	protocolStr := c.Query("protocol")
	if protocolStr == "" {
		protocolStr = "ssh" // 默认 SSH
	}

	protocolType := protocol.ProtocolType(protocolStr)

	// 检查协议是否支持
	if !h.factory.IsSupported(protocolType) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     fmt.Sprintf("Unsupported protocol: %s", protocolStr),
			"supported": h.factory.SupportedProtocols(),
		})
		return
	}

	// 获取主机信息
	host, err := h.getHostInfo(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 升级到 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Infof("Failed to upgrade to websocket: %v", err)
		return
	}
	defer ws.Close()

	// 生成会话 ID
	sessionID := uuid.New().String()
	logger.Infof("[%s] New connection for host %s (%s), session: %s",
		protocolType, host.Name, host.IP, sessionID)

	// 创建录制器
	rec := recorder.NewRecorder(sessionID, 120, 30)
	defer rec.Close()

	// 创建录制器适配器
	recorderAdapter := recorder.NewRecorderAdapter(rec)

	// 创建协议处理器
	handler, err := h.factory.Create(protocolType, recorderAdapter)
	if err != nil {
		logger.Infof("Failed to create protocol handler: %v", err)
		ws.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer handler.Close()

	// 添加到会话管理器
	h.sessionManager.AddSession(sessionID, ws)
	defer h.sessionManager.RemoveSession(sessionID)

	// 先创建登录记录（无论连接成功与否都需要记录）
	startTime := time.Now()
	userID := c.Query("userId")
	username := c.Query("username")

	loginRecord := &storage.LoginRecord{
		SessionID: sessionID,
		UserID:    userID,
		HostID:    host.ID,
		HostName:  host.Name,
		HostIP:    host.IP,
		Username:  username,
		LoginTime: startTime,
		Status:    "connecting",
	}
	h.storage.SaveLoginRecord(loginRecord)

	connectionSuccess := false // 标记连接是否成功

	defer func() {
		// 获取录制内容
		recording, _ := rec.ToAsciinema()

		if connectionSuccess {
			// 连接成功，关闭会话并更新登录记录为完成状态
			if err := h.storage.CloseSession(sessionID, recording); err != nil {
				logger.Infof("Failed to close session: %v", err)
			}
			h.storage.UpdateLoginRecordStatus(sessionID, "completed", time.Now())
		} else {
			// 连接失败，不创建会话录制记录，只更新登录记录为失败状态
			h.storage.UpdateLoginRecordStatus(sessionID, "failed", time.Time{})
		}

		logger.Infof("[%s] Session %s closed (success: %v)", protocolType, sessionID, connectionSuccess)
	}()

	// 连接配置
	config := &protocol.ConnectionConfig{
		HostID:     host.ID,
		HostIP:     host.IP,
		HostPort:   host.Port,
		Username:   "", // TODO: 从 SystemUser 获取
		Password:   "", // TODO: 从 SystemUser 获取
		PrivateKey: "", // TODO: 从 SystemUser 获取
		Protocol:   protocolType,
		SessionID:  sessionID,
		UserID:     userID,
		ProxyID:    h.proxyID,
		Timeout:    30 * time.Second,
		Options:    make(map[string]interface{}),
	}

	// ============================================================================
	// TODO: 获取 SystemUser 认证信息（RDP/SSH 都需要）
	// ============================================================================
	//
	// 实现步骤：
	// 1. 从查询参数或请求体中获取 systemUserId
	//    systemUserId := c.Query("systemUserId")
	//    或从权限规则中自动选择匹配的 SystemUser
	//
	// 2. 查询 SystemUser 信息
	//    systemUserRepo := repository.NewSystemUserRepository(h.storage.GetDB())
	//    systemUser, err := systemUserRepo.FindByID(systemUserId)
	//    if err != nil {
	//        return fmt.Errorf("system user not found: %w", err)
	//    }
	//
	// 3. 根据协议类型设置认证信息
	//    if protocolType == protocol.ProtocolRDP {
	//        // RDP 使用用户名和密码
	//        config.Username = systemUser.Username
	//        config.Password = systemUser.Password  // 需要解密
	//        // Windows 域（如果配置）
	//        if domain := systemUser.Domain; domain != "" {
	//            config.Options["domain"] = domain
	//        }
	//        // RDP 安全模式
	//        if systemUser.AuthType == "password" {
	//            config.Options["security"] = "nla"  // 推荐 NLA
	//        }
	//    } else if protocolType == protocol.ProtocolSSH {
	//        // SSH 可以使用密码或密钥
	//        config.Username = systemUser.Username
	//        if systemUser.AuthType == "password" {
	//            config.Password = systemUser.Password  // 需要解密
	//        } else if systemUser.AuthType == "key" {
	//            config.PrivateKey = systemUser.PrivateKey  // 需要解密
	//            if systemUser.Passphrase != "" {
	//                config.Options["passphrase"] = systemUser.Passphrase  // 需要解密
	//            }
	//        }
	//    }
	//
	// 4. 设置 RDP 特定选项（如果使用 RDP）
	//    if protocolType == protocol.ProtocolRDP {
	//        config.Options["width"] = 1920
	//        config.Options["height"] = 1080
	//        config.Options["dpi"] = 96
	//        config.Options["colorDepth"] = 24
	//        config.Options["enableDrive"] = true  // 启用文件传输
	//        config.Options["ignoreCert"] = false // 生产环境建议设为 false
	//    }
	//
	// 5. 解密敏感信息（密码、密钥等）
	//    - 使用从jwt_secret提取的AES密钥解密
	//

	// 连接到目标主机
	ctx := context.Background()
	if err := handler.Connect(ctx, config); err != nil {
		logger.Infof("Failed to connect: %v", err)
		ws.WriteJSON(map[string]string{"error": err.Error()})
		// connectionSuccess 保持 false，defer 中会标记为 failed
		return
	}

	// 连接成功，创建会话录制记录
	connectionSuccess = true
	sessionRecord := &storage.SessionRecord{
		ProxyID:      h.proxyID,
		SessionID:    sessionID,
		UserID:       userID,
		Username:     username,
		HostID:       host.ID,
		HostName:     host.Name,
		HostIP:       host.IP,
		StartTime:    startTime,
		Status:       "active",
		TerminalCols: 120,
		TerminalRows: 30,
	}

	if err := h.storage.SaveSession(sessionRecord); err != nil {
		logger.Infof("Failed to save session: %v", err)
	}

	// 处理 WebSocket 连接
	if err := handler.HandleWebSocket(ws); err != nil {
		logger.Infof("WebSocket handler error: %v", err)
	}

	logger.Infof("[%s] Session completed: %s", protocolType, sessionID)
}

// getHostInfo 获取主机信息（支持多种方式）
func (h *ProtocolHandler) getHostInfo(c *gin.Context) (*model.Host, error) {
	// 方式1：通过 hostId
	if hostID := c.Query("hostId"); hostID != "" {
		return h.hostRepo.FindByID(hostID)
	}

	// 方式2：通过 IP 地址
	if hostIP := c.Query("ip"); hostIP != "" {
		return h.hostRepo.FindByIP(hostIP)
	}

	// 方式3：通过主机名
	if hostName := c.Query("hostname"); hostName != "" {
		hosts, _, err := h.hostRepo.FindAll(1, 1, hostName, nil)
		if err != nil || len(hosts) == 0 {
			return nil, fmt.Errorf("host not found: %s", hostName)
		}
		return &hosts[0], nil
	}

	return nil, fmt.Errorf("hostId, ip or hostname is required")
}

// HandleSSH 处理 SSH 连接（保持向后兼容）
func (h *ProtocolHandler) HandleSSH(c *gin.Context) {
	c.Request.URL.RawQuery = c.Request.URL.RawQuery + "&protocol=ssh"
	h.HandleConnection(c)
}

// HandleRDP 处理 RDP 连接
func (h *ProtocolHandler) HandleRDP(c *gin.Context) {
	c.Request.URL.RawQuery = c.Request.URL.RawQuery + "&protocol=rdp"
	h.HandleConnection(c)
}
