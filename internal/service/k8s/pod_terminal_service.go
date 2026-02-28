package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

// PodTerminalService Pod 终端服务
type PodTerminalService struct {
	*K8sService
}

// NewPodTerminalService 创建 Pod 终端服务
func NewPodTerminalService(k8sService *K8sService) *PodTerminalService {
	return &PodTerminalService{K8sService: k8sService}
}

// terminalStreamHandler 终端流处理器，用于桥接 WebSocket 和 Kubernetes exec
type terminalStreamHandler struct {
	wsConn      *websocket.Conn
	resizeEvent chan remotecommand.TerminalSize
	mu          sync.Mutex
	closed      bool
	reader      *io.PipeReader
	writer      *io.PipeWriter
	// 命令审计相关字段
	clusterID   string
	clusterName string
	namespace   string
	podName     string
	container   string
	userID      string
	username    string
	commandBuf  strings.Builder // 命令缓冲区
}

// xtermMessage WebSocket 消息结构
type xtermMessage struct {
	MsgType string `json:"type"`  // 类型: resize 客户端调整终端, input 客户端输入
	Input   string `json:"input"` // msgtype=input 情况下使用
	Rows    uint16 `json:"rows"`  // msgtype=resize 情况下使用
	Cols    uint16 `json:"cols"`  // msgtype=resize 情况下使用
}

// Next 实现 TerminalSizeQueue 接口，获取终端大小
func (h *terminalStreamHandler) Next() *remotecommand.TerminalSize {
	select {
	case size := <-h.resizeEvent:
		return &size
	default:
		return nil
	}
}

// Read 实现 io.Reader 接口，从管道读取用户输入（由 goroutine 从 WebSocket 读取并写入管道）
func (h *terminalStreamHandler) Read(p []byte) (int, error) {
	if h.reader == nil {
		return 0, io.EOF
	}

	n, err := h.reader.Read(p)
	if err != nil && err != io.EOF {
		logger.Debugf("从管道读取失败: %v", err)
	}

	return n, err
}

// readFromWebSocket 从 WebSocket 读取消息并写入管道（在 goroutine 中运行）
func (h *terminalStreamHandler) readFromWebSocket() {
	defer func() {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		if h.writer != nil {
			h.writer.Close()
		}
	}()

	for {
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()

		// 读取 WebSocket 消息
		messageType, message, err := h.wsConn.ReadMessage()
		if err != nil {
			// 如果是正常关闭，不记录日志
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			// 其他错误记录调试日志
			logger.Debugf("WebSocket 读取消息失败: %v", err)
			return
		}

		// 只处理文本消息
		if messageType != websocket.TextMessage {
			logger.Debugf("收到非文本消息类型: %d", messageType)
			continue
		}

		// 尝试解析 JSON 消息
		var xtermMsg xtermMessage
		if err := json.Unmarshal(message, &xtermMsg); err == nil && xtermMsg.MsgType != "" {
			// 成功解析为 JSON 且包含 MsgType
			logger.Debugf("收到 JSON 消息: type=%s", xtermMsg.MsgType)

			// 处理不同类型的消息
			switch xtermMsg.MsgType {
			case "resize":
				// 终端大小调整
				logger.Debugf("终端大小调整: cols=%d, rows=%d", xtermMsg.Cols, xtermMsg.Rows)
				select {
				case h.resizeEvent <- remotecommand.TerminalSize{
					Width:  xtermMsg.Cols,
					Height: xtermMsg.Rows,
				}:
				default:
					// channel 已满，跳过
				}
				// resize 消息不写入管道，继续读取下一个消息
				continue
			case "ping":
				// 心跳消息，忽略，不写入管道
				logger.Debugf("收到心跳消息，忽略")
				continue
			case "input":
				// 用户输入，写入管道（需要拦截命令）
				if h.writer != nil {
					h.interceptAndWriteCommand([]byte(xtermMsg.Input))
				}
				continue
			default:
				// 未知类型，作为输入处理（需要拦截命令）
				if h.writer != nil {
					h.interceptAndWriteCommand(message)
				}
				continue
			}
		}

		// 如果不是 JSON 格式或解析失败，直接作为输入处理（原始字符串）
		// 这里需要拦截命令（检测回车键）
		if h.writer != nil {
			h.interceptAndWriteCommand(message)
		}
	}
}

// interceptAndWriteCommand 拦截命令并写入管道
func (h *terminalStreamHandler) interceptAndWriteCommand(data []byte) {
	for _, b := range data {
		// 检查是否是回车键或换行符（命令执行前拦截）
		if b == '\r' || b == '\n' {
			// 获取完整命令
			command := strings.TrimSpace(h.commandBuf.String())
			h.commandBuf.Reset()

			// 记录命令（忽略空命令和某些系统命令）
			if command != "" && !shouldIgnorePodCommand(command) {
				h.auditCommand(command)
			}
		} else if b == 0x03 { // Ctrl+C
			// 清空缓冲区
			h.commandBuf.Reset()
		} else if b == 0x7f || b == 0x08 { // 退格
			// 从缓冲区删除最后一个字符
			s := h.commandBuf.String()
			if len(s) > 0 {
				h.commandBuf.Reset()
				h.commandBuf.WriteString(s[:len(s)-1])
			}
		} else if b >= 32 && b <= 126 { // 可打印字符
			// 添加到命令缓冲区
			h.commandBuf.WriteByte(b)
		}
	}

	// 写入原始数据到管道
	if h.writer != nil {
		if _, err := h.writer.Write(data); err != nil {
			logger.Debugf("写入管道失败: %v", err)
		}
	}
}

// shouldIgnorePodCommand 判断是否应该忽略该命令
func shouldIgnorePodCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	// 忽略的命令列表
	ignoreList := []string{
		"exit",
		"logout",
		"clear",
		"reset",
		"history",
		"pwd",
	}

	for _, ignore := range ignoreList {
		if cmd == ignore {
			return true
		}
	}

	return false
}

// auditCommand 审计命令（异步记录到数据库）
func (h *terminalStreamHandler) auditCommand(command string) {
	// 异步记录命令，不阻塞终端操作
	go func() {
		record := &model.PodCommandRecord{
			ClusterID:   h.clusterID,
			ClusterName: h.clusterName,
			Namespace:   h.namespace,
			PodName:     h.podName,
			Container:   h.container,
			UserID:      h.userID,
			Username:    h.username,
			Command:     command,
			ExecutedAt:  time.Now(),
		}

		if err := database.DB.Create(record).Error; err != nil {
			logger.Debugf("记录 Pod 命令失败: %v", err)
		} else {
			logger.Debugf("已记录 Pod 命令: user=%s, pod=%s/%s, command=%s", h.username, h.namespace, h.podName, command)
		}
	}()
}

// Write 实现 io.Writer 接口，将 Kubernetes exec 输出写入 WebSocket
func (h *terminalStreamHandler) Write(p []byte) (int, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0, io.EOF
	}
	h.mu.Unlock()

	// 将数据发送到 WebSocket
	if err := h.wsConn.WriteMessage(websocket.TextMessage, p); err != nil {
		logger.Debugf("WebSocket Write 失败: %v", err)
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		return 0, err
	}

	return len(p), nil
}

// Close 关闭流处理器
func (h *terminalStreamHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.closed = true
		if h.writer != nil {
			h.writer.Close()
		}
		if h.reader != nil {
			h.reader.Close()
		}
		close(h.resizeEvent)
	}
}

// createRestConfig 从集群配置创建 Kubernetes REST Config
func (s *PodTerminalService) createRestConfig(cluster *model.K8sCluster) (*rest.Config, error) {
	if cluster.AuthType == "kubeconfig" && cluster.Kubeconfig != "" {
		// 从 kubeconfig 创建配置
		config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Kubeconfig))
		if err != nil {
			return nil, fmt.Errorf("解析 kubeconfig 失败: %v", err)
		}
		// 设置超时
		if config.Timeout == 0 {
			config.Timeout = 30 * time.Second
		}
		// 只有在没有 CA 证书的情况下才禁用 TLS 验证
		// 如果 kubeconfig 中已经包含了 CA 证书（CAFile 或 CAData），则使用它
		// 否则设置 Insecure = true
		if config.TLSClientConfig.CAFile == "" && len(config.TLSClientConfig.CAData) == 0 {
			config.TLSClientConfig.Insecure = true
		}
		return config, nil
	} else if cluster.AuthType == "token" && cluster.Token != "" && cluster.APIServer != "" {
		// 从 token 创建配置
		config := &rest.Config{
			Host:        cluster.APIServer,
			BearerToken: cluster.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
			Timeout: 30 * time.Second,
		}
		return config, nil
	}

	return nil, fmt.Errorf("缺少有效的认证信息")
}

// HandlePodTerminal 处理 Pod 终端 WebSocket 连接
func (s *PodTerminalService) HandlePodTerminal(clusterID, clusterName, namespace, podName, container, command string, userID, username string, ws *websocket.Conn) error {
	logger.Infof("开始处理 Pod 终端连接: clusterID=%s, namespace=%s, pod=%s, container=%s", clusterID, namespace, podName, container)

	cluster, err := s.GetClusterConfig(clusterID, clusterName)
	if err != nil {
		logger.Errorf("获取集群配置失败: %v", err)
		return fmt.Errorf("获取集群配置失败: %v", err)
	}

	// 如果 clusterName 为空，使用集群对象的 Name
	if clusterName == "" && cluster != nil {
		clusterName = cluster.Name
	}

	if podName == "" {
		return fmt.Errorf("pod_name 参数必填")
	}

	ns := s.getNamespace(cluster, namespace)
	logger.Infof("使用命名空间: %s", ns)

	// 创建 Kubernetes REST Config
	restConfig, err := s.createRestConfig(cluster)
	if err != nil {
		logger.Errorf("创建 Kubernetes REST Config 失败: %v", err)
		return fmt.Errorf("创建 Kubernetes 配置失败: %v", err)
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Errorf("创建 Kubernetes 客户端失败: %v", err)
		return fmt.Errorf("创建 Kubernetes 客户端失败: %v", err)
	}

	// 如果没有指定容器，使用第一个容器
	if container == "" {
		pod, err := clientset.CoreV1().Pods(ns).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("获取 Pod 信息失败: %v", err)
		}
		if len(pod.Spec.Containers) == 0 {
			return fmt.Errorf("Pod 中没有找到容器")
		}
		container = pod.Spec.Containers[0].Name
		logger.Infof("使用默认容器: %s", container)
	}

	// 如果没有指定命令，默认使用 /bin/sh
	if command == "" {
		command = "/bin/sh"
	}

	// 验证命令是否安全（只允许 shell 命令）
	validShells := []string{"/bin/sh", "/bin/bash", "sh", "bash"}
	isValidShell := false
	for _, shell := range validShells {
		if command == shell {
			isValidShell = true
			break
		}
	}
	if !isValidShell {
		return fmt.Errorf("不允许的命令，只允许使用 shell: %v", validShells)
	}

	// 创建 exec 请求
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(ns).
		SubResource("exec")

	// 设置 exec 选项
	execOptions := &corev1.PodExecOptions{
		Container: container,
		Command:   []string{command},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}

	req.VersionedParams(execOptions, scheme.ParameterCodec)

	// 创建 SPDY executor
	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		logger.Errorf("创建 SPDY executor 失败: %v", err)
		return fmt.Errorf("创建 executor 失败: %v", err)
	}
	logger.Infof("成功创建 SPDY executor, URL: %s", req.URL().String())

	// 创建管道用于桥接 WebSocket 和 Kubernetes exec
	reader, writer := io.Pipe()

	// 创建流处理器
	handler := &terminalStreamHandler{
		wsConn:      ws,
		resizeEvent: make(chan remotecommand.TerminalSize, 1),
		reader:      reader,
		writer:      writer,
		clusterID:   clusterID,
		clusterName: clusterName,
		namespace:   namespace,
		podName:     podName,
		container:   container,
		userID:      userID,
		username:    username,
	}

	// 启动 goroutine 从 WebSocket 读取消息并写入管道
	go handler.readFromWebSocket()

	// 发送连接成功消息（验证 WebSocket 连接是否正常）
	successMsg := "\r\n\x1b[32m[成功] 正在连接到 Pod 终端...\x1b[0m\r\n"
	if writeErr := ws.WriteMessage(websocket.TextMessage, []byte(successMsg)); writeErr != nil {
		logger.Errorf("WebSocket 连接已关闭，无法发送消息: %v", writeErr)
		handler.Close()
		return fmt.Errorf("WebSocket 连接已关闭: %v", writeErr)
	}

	logger.Infof("开始执行终端命令: command=%s, container=%s", command, container)

	// 设置默认终端大小（如果前端还没有发送）
	select {
	case handler.resizeEvent <- remotecommand.TerminalSize{Width: 80, Height: 24}:
	default:
		// channel 已满，跳过
	}

	// 启动流式执行（这会阻塞直到连接关闭）
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             handler,
		Stdout:            handler,
		Stderr:            handler,
		TerminalSizeQueue: handler,
		Tty:               true,
	})

	// 关闭处理器
	handler.Close()

	if err != nil {
		// 检查是否是 WebSocket 关闭相关的错误（这是正常的）
		errStr := err.Error()
		isWebSocketClose := err == io.EOF ||
			err == io.ErrClosedPipe ||
			websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
			websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
			errStr == "websocket: close 1005 (no status)" ||
			errStr == "websocket: close sent" ||
			errStr == "use of closed network connection"

		if isWebSocketClose {
			logger.Infof("WebSocket 连接正常关闭")
			return nil
		}

		logger.Errorf("执行终端命令失败: %v", err)

		// 尝试发送错误消息到客户端（如果 WebSocket 还连接着）
		errorMsg := fmt.Sprintf("\r\n\x1b[31m[错误] %v\x1b[0m\r\n", err)
		if writeErr := ws.WriteMessage(websocket.TextMessage, []byte(errorMsg)); writeErr != nil {
			logger.Debugf("发送错误消息失败: %v", writeErr)
		}

		return err
	}

	logger.Infof("Pod 终端连接已关闭: clusterID=%s, namespace=%s, pod=%s", clusterID, ns, podName)
	return nil
}
