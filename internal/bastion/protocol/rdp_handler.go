package protocol

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 获取 int 配置，支持 int/float64/string，失败返回默认值
func getIntOption(options map[string]any, key string, def int) int {
	if options == nil {
		return def
	}
	switch v := options[key].(type) {
	case int:
		if v > 0 {
			return v
		}
	case int32:
		if v > 0 {
			return int(v)
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	case string:
		if iv, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && iv > 0 {
			return iv
		}
	}
	return def
}

// formatInstruction 格式化 Guacamole 协议指令
// 格式：OPCODE,ARG1,ARG2,...; 每项格式：LENGTH.VALUE
// 例如：6.select,3.rdp; 或 4.size,1.0,4.1920,4.1080;
func formatInstruction(opcode string, args ...string) string {
	var parts []string
	// OPCODE 部分
	parts = append(parts, fmt.Sprintf("%d.%s", len(opcode), opcode))
	// 参数部分
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("%d.%s", len(arg), arg))
	}
	return strings.Join(parts, ",") + ";"
}

// validateGuacamoleMessage 验证 Guacamole 协议消息格式
// 格式：LENGTH.VALUE,LENGTH.VALUE,...; 其中 LENGTH 必须是数字
// 支持多个指令，用分号分隔，例如：4.sync,3.123;5.ready,36.uuid;
// 返回 true 如果消息格式有效，false 如果无效
func validateGuacamoleMessage(message string) bool {
	// 空消息无效
	if len(message) == 0 {
		return false
	}

	// 必须以分号结尾
	if !strings.HasSuffix(message, ";") {
		return false
	}

	// 移除末尾分号
	trimmed := strings.TrimSuffix(message, ";")
	if len(trimmed) == 0 {
		return false
	}

	// 按分号分割多个指令（如果有）
	// Guacamole 协议消息可能包含多个指令，例如：4.sync,3.123;5.ready,36.uuid;
	instructions := strings.Split(trimmed, ";")
	for _, instruction := range instructions {
		instruction = strings.TrimSpace(instruction)
		if len(instruction) == 0 {
			continue // 允许空指令（连续的分号）
		}

		// 按逗号分割元素
		parts := strings.Split(instruction, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) == 0 {
				continue // 允许空元素
			}

			// 每个元素必须是 LENGTH.VALUE 格式
			dotIndex := strings.Index(part, ".")
			if dotIndex <= 0 {
				// 没有点号或点号在开头，无效
				return false
			}

			// 提取长度部分
			lengthStr := part[:dotIndex]
			if len(lengthStr) == 0 {
				return false
			}

			// 验证长度部分是否全是数字
			for _, r := range lengthStr {
				if r < '0' || r > '9' {
					return false
				}
			}

			// 验证长度值是否合理（不能太大，避免溢出）
			length, err := strconv.Atoi(lengthStr)
			if err != nil || length < 0 || length > 1000000 {
				return false
			}

			// 验证值部分的长度是否匹配
			valuePart := part[dotIndex+1:]
			if len(valuePart) != length {
				// 长度不匹配，可能是消息被截断或格式错误
				return false
			}
		}
	}

	return true
}

// RDPHandler RDP 协议处理器
// 使用 Guacamole 协议连接到 RDP 服务器（通过 guacd）
type RDPHandler struct {
	config      *ConnectionConfig
	sessionInfo *SessionInfo
	recorder    SessionRecorder
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	connected   bool

	// Guacamole 连接
	guacdConn   net.Conn
	guacdReader *bufio.Reader
	guacdWriter *bufio.Writer

	// sync 消息计数（用于日志记录）
	syncCount int
	syncMu    sync.Mutex
}

// NewRDPHandler 创建 RDP 处理器
func NewRDPHandler(recorder SessionRecorder) ProtocolHandler {
	return &RDPHandler{
		recorder: recorder,
	}
}

// GetProtocolType 获取协议类型
func (h *RDPHandler) GetProtocolType() ProtocolType {
	return ProtocolRDP
}

// getSyncCount 获取 sync 消息计数
func (h *RDPHandler) getSyncCount() int {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	return h.syncCount
}

// incrementSyncCount 增加 sync 消息计数
func (h *RDPHandler) incrementSyncCount() {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	h.syncCount++
}

// Connect 连接到 RDP 服务器（通过 Guacamole）
func (h *RDPHandler) Connect(ctx context.Context, config *ConnectionConfig) error {
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
		Protocol:  ProtocolRDP,
		Status:    "connecting",
		StartTime: time.Now(),
	}

	// 从数据库 setting 表读取 Guacamole 配置（通过 config.Options 传入）
	// 所有 RDP 配置都由数据库 setting 表控制，不再从 config.yaml 读取
	rdpConfig := struct {
		GuacdHost        string
		GuacdPort        int
		RecordingEnabled bool
		RecordingPath    string
		RecordingFormat  string
	}{
		GuacdHost:        "localhost",  // 默认值
		GuacdPort:        4822,         // 默认值
		RecordingEnabled: true,         // 默认值
		RecordingPath:    "recordings", // 默认值
		RecordingFormat:  "guac",       // 默认值，保存为 .guac 格式
	}

	// 从 config.Options 读取配置（这些值来自数据库 setting 表）
	if v, ok := config.Options["guacd_host"].(string); ok && v != "" {
		rdpConfig.GuacdHost = v
	}
	if v, ok := config.Options["guacd_port"].(int); ok && v > 0 {
		rdpConfig.GuacdPort = v
	} else if v, ok := config.Options["guacd_port"].(float64); ok && int(v) > 0 {
		rdpConfig.GuacdPort = int(v)
	}
	if v, ok := config.Options["recording_enabled"].(bool); ok {
		rdpConfig.RecordingEnabled = v
	}
	if v, ok := config.Options["recording_path"].(string); ok && v != "" {
		rdpConfig.RecordingPath = v
	}
	if v, ok := config.Options["recording_format"].(string); ok && v != "" {
		rdpConfig.RecordingFormat = v
	}

	// 连接到 guacd
	guacdAddr := net.JoinHostPort(rdpConfig.GuacdHost, strconv.Itoa(rdpConfig.GuacdPort))
	log.Printf("[RDP] Connecting to guacd at %s", guacdAddr)
	conn, err := net.DialTimeout("tcp", guacdAddr, 10*time.Second)
	if err != nil {
		log.Printf("[RDP] Failed to connect to guacd at %s: %v", guacdAddr, err)
		return fmt.Errorf("failed to connect to guacd at %s: %w", guacdAddr, err)
	}
	log.Printf("[RDP] Successfully connected to guacd at %s", guacdAddr)

	// 打印 RDP 连接信息
	rdpPort := config.HostPort
	if rdpPort == 0 {
		rdpPort = 3389 // RDP 默认端口
	}
	log.Printf("[RDP] RDP connection info: host=%s:%d, username=%s, password=%s", config.HostIP, rdpPort, config.Username, config.Password)

	h.guacdConn = conn
	h.guacdReader = bufio.NewReader(conn)
	h.guacdWriter = bufio.NewWriter(conn)

	// Guacamole 协议：连接建立后，必须立即发送 select 命令，不能有任何延迟
	// 格式：6.select,3.rdp; （长度.值,长度.值;）
	selectCmd := formatInstruction("select", "rdp")
	selectCmdBytes := []byte(selectCmd)
	log.Printf("[RDP] Immediately sending select command after connection: %q (bytes: %v)", selectCmd, selectCmdBytes)

	// 直接使用原始连接写入，确保立即发送，不使用缓冲
	// 注意：必须刷新缓冲区，确保数据立即发送
	if _, err = conn.Write(selectCmdBytes); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send select command: %w", err)
	}
	// 刷新缓冲区，确保 select 命令立即发送
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err = tcpConn.SetNoDelay(true); err != nil {
			log.Printf("[RDP] Warning: failed to set TCP_NODELAY: %v", err)
		}
	}
	log.Printf("[RDP] Select command sent successfully")

	// 构建 Guacamole 连接配置
	// 优先使用前端传入的宽高，默认使用 1940x960（与前端固定分辨率一致）；按 guacd 建议限制到 4096 以内
	width := getIntOption(config.Options, "width", 1940)
	height := getIntOption(config.Options, "height", 960)

	const guacMaxDimension = 4096 // guacd rdp 默认上限，超过会被截断
	if width > guacMaxDimension {
		log.Printf("[RDP] Clamp width from %d to %d to follow guacd limit", width, guacMaxDimension)
		width = guacMaxDimension
	}
	if height > guacMaxDimension {
		log.Printf("[RDP] Clamp height from %d to %d to follow guacd limit", height, guacMaxDimension)
		height = guacMaxDimension
	}

	allowClipboard := false
	if v, ok := config.Options["allow_clipboard"].(bool); ok {
		allowClipboard = v
	}
	enableFileTransfer := false
	if v, ok := config.Options["enable_file_transfer"].(bool); ok {
		enableFileTransfer = v
	}
	drivePath := "/replay-drive"
	if v, ok := config.Options["drive_path"].(string); ok && v != "" {
		drivePath = v
	}

	// 构建 Guacamole 连接参数
	// 注意：参数顺序需要根据 guacd 返回的 args 响应来调整
	params := []string{
		"version", "VERSION_1_5_0", // 协议版本（1.6.0 版本仍支持此协议版本）
		"client-name", "ZJump",
		"scheme", "rdp",
		"hostname", config.HostIP,
		"port", strconv.Itoa(rdpPort),
		"username", config.Username,
		"password", config.Password,
		"width", strconv.Itoa(width),
		"height", strconv.Itoa(height),
		"dpi", "96",
		"ignore-cert", "true",
		"resize-method", "display-update",
		"disable-copy", strconv.FormatBool(!allowClipboard),
		"disable-paste", strconv.FormatBool(!allowClipboard),
		"cert-tofu", "true", // Trust On First Use：首次使用时自动信任证书
		// 注意：不要设置 disable-auth，这会与 security 参数冲突
		// 但 guacd 可能期望这个参数，所以设置为 false 以明确启用身份验证
		"disable-auth", "false", // 明确启用身份验证（与 security 参数配合使用）
		// xrdp 兼容性参数：禁用可能不支持的高级功能
		"disable-audio", "true", // xrdp 可能不支持音频
		"enable-printing", "false", // xrdp 可能不支持打印
		"disable-bitmap-caching", "true", // xrdp 兼容性更好
		"disable-offscreen-caching", "true",
		"disable-glyph-caching", "true",
		// xrdp 兼容性：设置服务器布局（如果未设置，可能导致连接问题）
		// 常见的布局值：en-us-qwerty, en-us-qwerty-intl, fr-fr-azerty, de-de-qwertz 等
		// 如果不设置，使用空字符串让服务器使用默认布局
		"server-layout", "", // 让服务器使用默认布局
	}

	// 启用文件传输（驱动器映射）
	if enableFileTransfer {
		params = append(params,
			"enable-drive", "true",
			"drive-path", drivePath,
			"create-drive-path", "true",
		)
	}

	// 设置 security 参数
	// 对于 xrdp 服务器，必须使用 "rdp" 安全模式，否则会出现错误 519
	// 如果 config.Options["security"] 未设置，默认使用 "rdp" 以兼容 xrdp
	security := "rdp" // 默认值，兼容 xrdp
	if sec, ok := config.Options["security"].(string); ok && sec != "" {
		security = sec
		log.Printf("[RDP] Using security mode from config: %s", security)
	} else {
		log.Printf("[RDP] Using default security mode: %s (compatible with xrdp)", security)
	}
	params = append(params, "security", security)

	log.Printf("[RDP] Guacamole connection parameters: hostname=%s, port=%d, username=%s, width=%d, height=%d",
		config.HostIP, rdpPort, config.Username, width, height)

	// 添加域（如果配置）
	if domain, ok := config.Options["domain"].(string); ok && domain != "" {
		params = append(params, "domain", domain)
	}

	// 如果启用录制，使用 Guacamole 内置录制功能
	if rdpConfig.RecordingEnabled {
		// recording_path 配置的是宿主机路径（如 ./recordings）
		// 但传给 guacd 的应该是容器内路径（/replay）
		hostPath := rdpConfig.RecordingPath

		// 如果录制路径为空，说明配置有问题，直接返回错误
		if hostPath == "" {
			return fmt.Errorf("recording is enabled but recording_path is not configured")
		}

		// 将宿主机路径转换为容器内路径
		// 数据库存储的是宿主机路径（如 ./recordings），需要转换为容器内路径（/replay）
		// 获取容器内路径（从环境变量或使用默认值）
		containerBasePath := os.Getenv("RECORDING_CONTAINER_PATH")
		if containerBasePath == "" {
			containerBasePath = "/replay"
		}
		guacdContainerPath := containerBasePath

		log.Printf("[RDP] Recording path conversion: host=%s -> container=%s", hostPath, guacdContainerPath)

		// 按天创建目录：/guacdContainerPath/2024/01/15/
		now := time.Now()
		dayPath := filepath.Join(
			guacdContainerPath,
			strconv.Itoa(now.Year()),
			fmt.Sprintf("%02d", int(now.Month())),
			fmt.Sprintf("%02d", now.Day()),
		)
		guacdRecordingPath := dayPath

		// 文件名：sessionID_username.guac（例如：abc123_admin.guac）
		// 使用界面用户（登录 zjump 的用户）而不是 Windows 登录用户（系统用户）
		uiUsername := ""
		if v, ok := config.Options["ui_username"].(string); ok && v != "" {
			uiUsername = v
		}
		recordingName := config.SessionID
		if uiUsername != "" {
			recordingName = config.SessionID + "_" + uiUsername
		} else if config.Username != "" {
			// 如果没有界面用户名，回退到使用 Windows 登录用户名（兼容旧代码）
			recordingName = config.SessionID + "_" + config.Username
		}

		// 添加 .guac 扩展名（guacd 需要完整的文件名）
		recordingNameWithExt := recordingName + ".guac"

		// 在代码中创建录制目录（如果目录存在就忽略错误）
		// 注意：这里创建的是容器内的路径，需要确保有写权限
		if err := os.MkdirAll(guacdRecordingPath, 0755); err != nil {
			log.Printf("[RDP] Warning: Failed to create recording directory %s: %v (will continue anyway)", guacdRecordingPath, err)
		} else {
			log.Printf("[RDP] Recording directory created/verified: %s", guacdRecordingPath)
		}

		// 设置录制参数（传给 guacd，使用容器内的路径）
		// 注意：虽然代码已经创建了目录，但 guacd 可能还需要 create-recording-path 参数
		params = append(params, "recording-path", guacdRecordingPath)
		params = append(params, "create-recording-path", "true")
		params = append(params, "recording-name", recordingNameWithExt) // 使用带扩展名的文件名
		// format 使用 config 中的 recording_format（已通过 rdpConfig 覆盖）
		if rdpConfig.RecordingFormat != "" {
			params = append(params, "recording-format", rdpConfig.RecordingFormat)
		}
		log.Printf("[RDP] Recording enabled: guacd container path=%s, name=%s", guacdRecordingPath, recordingNameWithExt)
		log.Printf("[RDP] Note: guacd will create recording file at container path %s, ensure volume mount is configured correctly", guacdRecordingPath)
	}

	// select 命令已在连接建立后立即发送，这里不再重复发送

	// 读取 select 响应（guacd 会返回支持的参数列表，格式：args.<长度>.<参数>;）
	// 注意：响应以分号结尾
	log.Printf("[RDP] Waiting for select response...")
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	selectResponse, err := h.guacdReader.ReadString(';')
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("[RDP] Select response timeout, continuing...")
		} else {
			log.Printf("[RDP] Failed to read select response: %v", err)
			conn.Close()
			return fmt.Errorf("failed to read select response: %w", err)
		}
	} else {
		// 移除末尾的分号
		selectResponse = strings.TrimSuffix(selectResponse, ";")
		log.Printf("[RDP] Received select response: %q", selectResponse)
		// 响应格式：OPCODE,ARG1,ARG2,...; 例如：4.args,10.hostname,4.port,...
		parts := strings.Split(selectResponse, ",")
		if len(parts) == 0 {
			conn.Close()
			return fmt.Errorf("invalid select response: %s", selectResponse)
		}

		// 解析 OPCODE
		opcodePart := parts[0] // 格式：LENGTH.OPCODE
		opcodeParts := strings.SplitN(opcodePart, ".", 2)
		if len(opcodeParts) != 2 {
			conn.Close()
			return fmt.Errorf("invalid select response format: %s", selectResponse)
		}

		opcode := opcodeParts[1]
		if opcode == "error" {
			// 解析错误：error,<CODE长度>.<CODE>,<MESSAGE长度>.<MESSAGE>
			errorMsg := ""
			if len(parts) >= 3 {
				msgPart := parts[2] // 格式：LENGTH.MESSAGE
				msgParts := strings.SplitN(msgPart, ".", 2)
				if len(msgParts) == 2 {
					errorMsg = msgParts[1]
				}
			}
			conn.Close()
			return fmt.Errorf("guacd error: %s", errorMsg)
		}

		if opcode != "args" {
			log.Printf("[RDP] Warning: Unexpected select response opcode: %s (expected 'args')", opcode)
		} else {
			// 解析 args 响应，获取参数名列表
			// 格式：4.args,10.hostname,4.port,8.username,...
			argNames := make([]string, 0)
			for i := 1; i < len(parts); i++ {
				argPart := parts[i] // 格式：LENGTH.NAME
				argParts := strings.SplitN(argPart, ".", 2)
				if len(argParts) == 2 {
					argNames = append(argNames, argParts[1])
				}
			}

			// 根据 args 响应中的参数顺序，构建参数值列表
			// 参数名需要和 guacd 返回的参数名完全匹配（包括大小写）
			paramValues := make([]string, 0, len(argNames))
			paramMap := make(map[string]string)

			// 构建参数映射表
			// 但 guacd 返回的参数名可能是不同的大小写格式（如 "VERSION_1_5_0"）
			// 需要建立完整的映射关系，确保所有格式都能匹配
			for i := 0; i < len(params); i += 2 {
				if i+1 < len(params) {
					key := params[i]
					value := params[i+1]
					// 存储原始 key（小写，如 "version"）
					paramMap[key] = value
					// 同时存储小写版本（用于匹配）
					paramMap[strings.ToLower(key)] = value
					// 同时存储大写版本（用于匹配 VERSION_1_5_0 等）
					paramMap[strings.ToUpper(key)] = value
				}
			}

			// 特殊处理：VERSION_1_5_0 参数
			// guacd 返回的参数名是 VERSION_1_5_0，但我们的 key 是 version
			if v, ok := paramMap["version"]; ok {
				paramMap["VERSION_1_5_0"] = v
				log.Printf("[RDP] Mapped VERSION_1_5_0 -> version value: %s", v)
			}

			// 特殊处理：security 参数 - 确保所有可能的格式都能匹配
			// guacd 可能返回 "security" 或 "SECURITY" 等格式
			if v, ok := paramMap["security"]; ok {
				paramMap["SECURITY"] = v
				paramMap["Security"] = v
			}

			// 调试：打印参数映射（显示所有录制相关参数）
			log.Printf("[RDP] Parameter map size: %d", len(paramMap))
			// 打印所有录制相关参数
			recordingKeys := []string{"recording-path", "RECORDING-PATH", "recording_name", "recording-name", "RECORDING-NAME",
				"create-recording-path", "CREATE-RECORDING-PATH", "create_recording_path", "recording-format", "RECORDING-FORMAT"}
			for _, key := range recordingKeys {
				if v, ok := paramMap[key]; ok {
					log.Printf("[RDP] Recording parameter: %s = %s", key, v)
				}
			}
			// 打印前10个参数（用于调试）
			count := 0
			for k, v := range paramMap {
				if count < 10 {
					log.Printf("[RDP] Parameter map: %s = %s", k, v)
					count++
				}
			}

			// 直接按照 guacd 返回的参数名顺序，从 paramMap 中获取值
			for i, name := range argNames {
				var value string
				var found bool
				// 首先尝试精确匹配（guacd 返回的参数名，完全匹配）
				if v, ok := paramMap[name]; ok {
					value = v
					found = true
					// 记录关键参数和录制相关参数
					if i < 20 || strings.Contains(strings.ToLower(name), "recording") || strings.ToLower(name) == "security" || strings.ToLower(name) == "version" || strings.ToLower(name) == "hostname" {
						log.Printf("[RDP] Parameter[%d] %s = %s (exact match)", i, name, value)
					}
				} else {
					// 如果精确匹配失败，尝试小写匹配（兼容性处理）
					nameLower := strings.ToLower(name)
					if v, ok := paramMap[nameLower]; ok {
						value = v
						found = true
						if i < 20 || strings.Contains(strings.ToLower(name), "recording") || strings.ToLower(name) == "security" {
							log.Printf("[RDP] Parameter[%d] %s = %s (lowercase match)", i, name, value)
						}
					} else {
						// 如果还是找不到，尝试大写匹配
						nameUpper := strings.ToUpper(name)
						if v, ok := paramMap[nameUpper]; ok {
							value = v
							found = true
							if i < 20 || strings.Contains(strings.ToLower(name), "recording") || strings.ToLower(name) == "security" {
								log.Printf("[RDP] Parameter[%d] %s = %s (uppercase match)", i, name, value)
							}
						}
					}
				}

				if found {
					paramValues = append(paramValues, value)
				} else {
					// 如果参数不存在，发送空字符串
					paramValues = append(paramValues, "")
					// 特别处理 security 参数缺失的情况
					if strings.ToLower(name) == "security" {
						// 尝试修复：如果 security 参数缺失，使用默认值 "rdp"
						paramValues[i] = "rdp"
						value = "rdp"
						found = true
					} else {
						// 打印录制相关参数和前面的参数用于调试
						if strings.Contains(strings.ToLower(name), "recording") || i < 20 {
							log.Printf("[RDP] Parameter[%d] %s = (empty, not found in paramMap)", i, name)
							// 尝试查找可能的匹配
							for k := range paramMap {
								if strings.Contains(strings.ToLower(k), strings.ToLower(name)) {
									log.Printf("[RDP]   -> Found similar key in paramMap: %s = %s", k, paramMap[k])
								}
							}
						}
					}
				}
			}

			// 更新 params 为按顺序的参数值列表
			params = paramValues
			log.Printf("[RDP] Total parameters prepared: %d", len(params))

			// 调试：打印所有参数的值，特别是 security 参数
			for i, name := range argNames {
				if i < len(paramValues) {
					value := paramValues[i]
					// 特别关注 security 参数
					if strings.ToLower(name) == "security" {
						// 如果 security 参数为空，尝试修复
						if value == "" {
							paramValues[i] = "rdp"
							params[i] = "rdp"
						}
					}
					// 打印前 30 个参数用于调试
					if i < 30 {
						log.Printf("[RDP] Final param[%d] %s = %q", i, name, value)
					}
				}
			}
		}
	}

	// 发送 size, audio, video, image 指令
	// size 指令：4.size,1.0,4.1920,4.1080; (layer=0, width, height)
	widthStr := strconv.Itoa(width)
	heightStr := strconv.Itoa(height)
	dpiStr := "96"

	sizeCmd := formatInstruction("size", "0", widthStr, heightStr, dpiStr)
	log.Printf("[RDP] Sending size command: %q", sizeCmd)
	if _, err = h.guacdWriter.WriteString(sizeCmd); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send size command: %w", err)
	}
	if err = h.guacdWriter.Flush(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to flush after size: %w", err)
	}

	// audio 指令
	audioCmd := formatInstruction("audio", "audio/L8")
	log.Printf("[RDP] Sending audio command: %q", audioCmd)
	if _, err = h.guacdWriter.WriteString(audioCmd); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send audio command: %w", err)
	}
	if err = h.guacdWriter.Flush(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to flush after audio: %w", err)
	}

	// video 指令（无参数）
	videoCmd := formatInstruction("video")
	log.Printf("[RDP] Sending video command: %q", videoCmd)
	if _, err = h.guacdWriter.WriteString(videoCmd); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send video command: %w", err)
	}
	if err = h.guacdWriter.Flush(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to flush after video: %w", err)
	}

	// image 指令
	imageCmd := formatInstruction("image", "image/jpeg", "image/png", "image/webp")
	log.Printf("[RDP] Sending image command: %q", imageCmd)
	if _, err = h.guacdWriter.WriteString(imageCmd); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send image command: %w", err)
	}
	if err = h.guacdWriter.Flush(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to flush after image: %w", err)
	}

	// 发送 connect 指令
	// connect 指令的参数应该是按照 args 响应中的参数顺序的值列表
	// 格式：7.connect,<value1长度>.<value1>,<value2长度>.<value2>,...;
	// params 已经是按照 args 响应顺序的参数值列表
	connectCmd := formatInstruction("connect", params...)
	log.Printf("[RDP] Sending connect command with %d parameters", len(params))
	// 只打印 connect 命令的前 500 个字符，避免日志过长
	connectCmdPreview := connectCmd
	if len(connectCmdPreview) > 500 {
		connectCmdPreview = connectCmdPreview[:500] + "..."
	}
	log.Printf("[RDP] Connect command preview: %q", connectCmdPreview)
	if _, err = h.guacdWriter.WriteString(connectCmd); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send connect command: %w", err)
	}
	if err = h.guacdWriter.Flush(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to flush after connect: %w", err)
	}

	// 读取 connect 响应（guacd 会返回 ready.<UUID长度>.<UUID>; 或 error.<CODE长度>.<CODE>.<MESSAGE长度>.<MESSAGE>;）
	// 注意：响应以分号结尾
	log.Printf("[RDP] Waiting for connect response...")
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err := h.guacdReader.ReadString(';')
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		log.Printf("[RDP] Failed to read connect response: %v", err)
		conn.Close()
		return fmt.Errorf("failed to read connect response: %w", err)
	}

	// 移除末尾的分号
	response = strings.TrimSuffix(response, ";")
	log.Printf("[RDP] Received connect response: %q", response)

	// 解析响应
	// 格式：OPCODE,ARG1,ARG2,...; 例如：5.ready,36.12345678-1234-1234-1234-123456789abc; 或 5.error,3.123,10.error message;
	parts := strings.Split(response, ",")
	if len(parts) == 0 {
		conn.Close()
		return fmt.Errorf("invalid guacd response: %s", response)
	}

	// 解析 OPCODE
	opcodePart := parts[0] // 格式：LENGTH.OPCODE
	opcodeParts := strings.SplitN(opcodePart, ".", 2)
	if len(opcodeParts) != 2 {
		conn.Close()
		return fmt.Errorf("invalid guacd response format: %s", response)
	}

	opcode := opcodeParts[1]
	if opcode == "error" {
		// 解析错误：error,<CODE长度>.<CODE>,<MESSAGE长度>.<MESSAGE>
		errorMsg := ""
		if len(parts) >= 3 {
			msgPart := parts[2] // 格式：LENGTH.MESSAGE
			msgParts := strings.SplitN(msgPart, ".", 2)
			if len(msgParts) == 2 {
				errorMsg = msgParts[1]
			}
		}
		conn.Close()
		return fmt.Errorf("guacd connection error: %s", errorMsg)
	}

	if opcode != "ready" {
		conn.Close()
		return fmt.Errorf("unexpected guacd response: %s (expected 'ready')", response)
	}

	log.Printf("[RDP] Guacamole connection established successfully")

	// 连接成功
	h.connected = true
	h.sessionInfo.Status = "active"

	log.Printf("[RDP] Successfully connected to RDP server %s:%d via Guacamole (session: %s)",
		config.HostIP, rdpPort, config.SessionID)

	// 记录会话开始
	if h.recorder != nil {
		h.recorder.RecordStart(h.sessionInfo)
	}

	return nil
}

// HandleWebSocket 处理 WebSocket 连接（双向转发 Guacamole 协议）
func (h *RDPHandler) HandleWebSocket(ws *websocket.Conn) error {
	if !h.IsAlive() {
		return fmt.Errorf("RDP connection not established")
	}

	log.Printf("[RDP] Handling WebSocket connection (session: %s)", h.config.SessionID)

	// 错误通道
	errChan := make(chan error, 2)

	// WebSocket -> Guacd (客户端输入)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[RDP] Panic in WebSocket->Guacd goroutine: %v", r)
			}
		}()

		for {
			messageType, data, err := ws.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[RDP] WebSocket read error: %v", err)
				}
				errChan <- err
				return
			}

			// 处理文本消息（Guacamole 协议）
			if messageType == websocket.TextMessage {
				messageStr := string(data)

				// 验证消息格式：检查是否是未格式化的消息（纯文本，没有 Guacamole 协议格式）
				// Guacamole 协议格式：LENGTH.OPCODE 或 LENGTH.OPCODE,...
				// 如果收到纯文本（如 "ack"、"disconnect"），需要转换为正确格式
				if !strings.Contains(messageStr, ".") && !strings.Contains(messageStr, ",") && !strings.HasSuffix(messageStr, ";") {
					// 这是未格式化的消息，需要转换为 Guacamole 协议格式
					opcode := strings.TrimSpace(messageStr)
					opcodeLength := len(opcode)
					messageStr = fmt.Sprintf("%d.%s;", opcodeLength, opcode)
					log.Printf("[RDP] *** WARNING: Received unformatted message, converting to Guacamole format: %q -> %q ***", string(data), messageStr)
				} else if !strings.HasSuffix(messageStr, ";") {
					// 消息有格式但缺少分号，添加分号
					messageStr = messageStr + ";"
					log.Printf("[RDP] *** WARNING: Message missing semicolon, adding it: %q -> %q ***", string(data), messageStr)
				}

				// 修复 sync 消息格式：sync 消息必须有时间戳参数
				// 正确格式：4.sync,<timestamp_length>.<timestamp>;
				// 如果收到 "4.sync;" 或 "sync;"，需要添加时间戳参数
				if strings.HasSuffix(messageStr, "sync;") || strings.HasSuffix(messageStr, ".sync;") {
					// 检查是否已经有参数（包含逗号）
					if !strings.Contains(messageStr, ",") {
						// 没有参数，添加时间戳参数
						timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
						timestampLength := len(timestamp)
						// 提取 sync 部分（去掉分号）
						syncPart := strings.TrimSuffix(messageStr, ";")
						messageStr = fmt.Sprintf("%s,%d.%s;", syncPart, timestampLength, timestamp)
						log.Printf("[RDP] *** FIXED: Added timestamp parameter to sync message: %q -> %q ***", string(data), messageStr)
					}
				}

				// 记录收到的消息类型（用于调试）
				// sync 消息非常重要，需要确保及时转发，防止 776 超时错误
				if strings.HasPrefix(messageStr, "sync") || strings.Contains(messageStr, ",sync,") || strings.Contains(messageStr, ".sync,") {
					// 更新 sync 消息计数（不打印日志）
					h.incrementSyncCount()
				} else if strings.HasPrefix(messageStr, "key") || strings.HasPrefix(messageStr, "mouse") {
					// 键盘和鼠标消息太多，不记录
				} else {
					// 记录其他类型的消息（前100个字符）
					// 特别关注 ready 响应和其他重要消息
					preview := messageStr
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
				}

				// 验证消息格式，防止转发格式错误的消息导致 guacd 解析失败
				if !validateGuacamoleMessage(messageStr) {
					log.Printf("[RDP] *** WARNING: Invalid Guacamole message format from client, skipping ***")
					// 记录消息的详细信息用于调试
					log.Printf("[RDP] Message length: %d bytes", len(messageStr))
					preview := messageStr
					if len(preview) > 500 {
						preview = preview[:500] + "..."
					}
					log.Printf("[RDP] Invalid message preview: %q", preview)
					// 记录原始字节（前100字节）用于调试二进制数据问题
					if len(messageStr) > 0 {
						bytesPreview := messageStr
						if len(bytesPreview) > 100 {
							bytesPreview = bytesPreview[:100]
						}
						log.Printf("[RDP] First 100 bytes (hex): %x", []byte(bytesPreview))
					}
					// 不转发无效消息，继续处理下一条
					continue
				}

				// 直接转发到 guacd（前端发送的指令应该已经是正确的格式，以分号结尾）
				// 关键：sync 消息必须立即转发，不能延迟，否则会导致 776 超时错误
				if _, err := h.guacdWriter.WriteString(messageStr); err != nil {
					log.Printf("[RDP] Failed to write to guacd: %v", err)
					errChan <- err
					return
				}
				// 立即刷新，确保 sync 消息及时发送
				if err := h.guacdWriter.Flush(); err != nil {
					log.Printf("[RDP] Failed to flush guacd writer: %v", err)
					errChan <- err
					return
				}

				// 记录输入数据
				h.mu.Lock()
				h.sessionInfo.BytesIn += int64(len(data))
				h.mu.Unlock()

				if h.recorder != nil {
					h.recorder.RecordData(h.config.SessionID, "in", data)
				}
			}
		}
	}()

	// Guacd -> WebSocket (服务器输出)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[RDP] Panic in Guacd->WebSocket goroutine: %v", r)
			}
		}()

		for {
			// 读取 Guacamole 指令（以分号结尾）
			line, err := h.guacdReader.ReadString(';')
			if err != nil {
				if err.Error() != "EOF" {
					log.Printf("[RDP] Guacd read error: %v", err)
				}
				errChan <- err
				return
			}

			// 检查是否是错误消息（Guacamole 协议错误格式）
			// 支持多种错误格式
			// 格式1：5.error,<MESSAGE长度>.<MESSAGE>,<CODE长度>.<CODE>;
			// 格式2：error.<CODE长度>.<CODE>,<MESSAGE长度>.<MESSAGE>;
			lineTrimmed := strings.TrimSuffix(line, ";")
			isError := strings.HasPrefix(lineTrimmed, "error.") ||
				strings.HasPrefix(lineTrimmed, "5.error") ||
				strings.Contains(lineTrimmed, ",error,")

			if isError {
				// 解析错误消息
				parts := strings.Split(lineTrimmed, ",")
				errorCode := ""
				errorMsg := ""

				if len(parts) >= 3 {
					// 格式：5.error,<MESSAGE长度>.<MESSAGE>,<CODE长度>.<CODE>
					// 注意：消息在前，错误码在后
					// 例如：5.error,18.Aborted. See logs.,3.776
					msgPart := parts[1]  // 消息部分，例如：18.Aborted. See logs.
					codePart := parts[2] // 错误码部分，例如：3.776

					// 解析消息部分：格式为 <长度>.<消息内容>
					// 注意：消息内容可能包含点号，所以需要先找到第一个点号
					msgParts := strings.SplitN(msgPart, ".", 2)
					codeParts := strings.SplitN(codePart, ".", 2)

					if len(msgParts) == 2 && len(codeParts) == 2 {
						errorMsg = msgParts[1]   // 消息内容（可能包含点号）
						errorCode = codeParts[1] // 错误码
						log.Printf("[RDP] Guacd error: code=%s, message=%s", errorCode, errorMsg)
					}
				} else if len(parts) >= 2 {
					// 尝试解析其他格式
					log.Printf("[RDP] Guacd error message (alternative format): %q", line)
				} else {
					log.Printf("[RDP] Guacd error message (unparsed): %q", line)
				}

				// 即使发生错误，也先转发到 WebSocket，让客户端知道错误
				// 这样前端可以正确解析并显示错误信息
				if err := ws.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
					log.Printf("[RDP] Failed to write error to WebSocket: %v", err)
				} else {
					log.Printf("[RDP] Error message forwarded to client: code=%s, message=%s", errorCode, errorMsg)
				}

				// 对于致命错误（如超时、连接拒绝等），关闭连接
				// 注意：某些错误可能只是警告，但 776 (CLIENT_TIMEOUT) 通常是致命的
				log.Printf("[RDP] Closing connection due to Guacamole error")
				errChan <- fmt.Errorf("guacamole error: code=%s, message=%s", errorCode, errorMsg)
				return
			}

			// 验证消息格式，防止转发格式错误的消息导致前端解析失败
			if !validateGuacamoleMessage(line) {
				log.Printf("[RDP] *** WARNING: Invalid Guacamole message format from guacd, skipping ***")
				// 记录消息的详细信息用于调试
				log.Printf("[RDP] Message length: %d bytes", len(line))
				preview := line
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				log.Printf("[RDP] Invalid message preview: %q", preview)
				// 记录原始字节（前200字节）用于调试二进制数据问题
				if len(line) > 0 {
					bytesPreview := line
					if len(bytesPreview) > 200 {
						bytesPreview = bytesPreview[:200]
					}
					log.Printf("[RDP] First 200 bytes (hex): %x", []byte(bytesPreview))
					// 也记录可打印字符
					printable := ""
					for _, b := range []byte(bytesPreview) {
						if b >= 32 && b < 127 {
							printable += string(b)
						} else {
							printable += fmt.Sprintf("\\x%02x", b)
						}
					}
					log.Printf("[RDP] First 200 bytes (printable): %q", printable)
				}
				// 尝试分析问题：检查是否有非ASCII字符
				nonASCII := 0
				for _, r := range line {
					if r > 127 {
						nonASCII++
					}
				}
				if nonASCII > 0 {
					log.Printf("[RDP] Message contains %d non-ASCII characters", nonASCII)
				}
				// 不转发无效消息，继续处理下一条
				continue
			}

			// 记录从 guacd 收到的消息（用于调试）
			// 特别关注前几条消息，看看是否有错误或异常
			linePreview := line
			if len(linePreview) > 200 {
				linePreview = linePreview[:200] + "..."
			}

			// 转发到 WebSocket（保持分号结尾）
			if err := ws.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				log.Printf("[RDP] Failed to write to WebSocket: %v", err)
				errChan <- err
				return
			}

			// 记录输出数据
			data := []byte(line)
			h.mu.Lock()
			h.sessionInfo.BytesOut += int64(len(data))
			h.mu.Unlock()

			if h.recorder != nil {
				h.recorder.RecordData(h.config.SessionID, "out", data)
			}
		}
	}()

	// 等待错误或上下文取消
	select {
	case err := <-errChan:
		if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			log.Printf("[RDP] Connection error: %v", err)
		}
		return err
	case <-h.ctx.Done():
		log.Printf("[RDP] Context cancelled")
		return nil
	}
}

// Close 关闭连接
func (h *RDPHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel != nil {
		h.cancel()
	}

	// 关闭 guacd 连接
	if h.guacdConn != nil {
		h.guacdConn.Close()
	}

	h.connected = false
	h.sessionInfo.Status = "closed"
	now := time.Now()
	h.sessionInfo.EndTime = &now

	// 记录会话结束
	if h.recorder != nil {
		h.recorder.RecordEnd(h.config.SessionID, now)
	}

	log.Printf("[RDP] Connection closed (session: %s)", h.config.SessionID)

	return nil
}

// GetSessionInfo 获取会话信息
func (h *RDPHandler) GetSessionInfo() *SessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionInfo
}

// IsAlive 检查连接是否存活
func (h *RDPHandler) IsAlive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected && h.ctx != nil && h.ctx.Err() == nil && h.guacdConn != nil
}

// 注册 RDP 处理器到工厂
func init() {
	GetFactory().Register(ProtocolRDP, NewRDPHandler)
}
