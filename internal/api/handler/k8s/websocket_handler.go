package k8s

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/model"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// StreamPodLogs 流式传输 Pod 日志
// @Summary 流式传输 Pod 日志
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param container query string false "容器名称"
// @Param follow query bool false "是否跟随日志（默认false）"
// @Param tail_lines query int false "显示最后N行（默认100）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/ws/logs [get]
func (h *K8sHandler) StreamPodLogs(c *gin.Context) {
	// 验证 token（从 query 参数获取，因为 WebSocket 不能使用 Authorization header）
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, model.Error(401, "缺少 token 参数"))
		return
	}

	// 验证 token 并获取用户信息
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
		return
	}

	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	container := c.Query("container")
	follow := c.Query("follow") == "true"
	tailLines, _ := strconv.Atoi(c.Query("tail_lines"))

	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and pod_name are required"))
		return
	}

	// 权限检查（WebSocket 请求需要手动检查权限）
	if clusterID != "" {
		userID := claims.UserID
		roles, err := h.roleRepo.GetRolesByUserID(userID)
		if err != nil {
			roles = []model.Role{}
		}

		hasPermission := h.permissionService.CheckResourcePermission(userID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionRead, roles)
		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有访问该资源的权限"))
			return
		}
	}

	// 升级到 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("升级到 WebSocket 失败: %v", err)))
		return
	}
	defer ws.Close()

	// 创建日志服务
	logsService := k8sService.NewPodLogsService(h.service.(*k8sService.K8sService))

	// 流式传输日志
	if err := logsService.StreamPodLogs(clusterID, clusterName, namespace, podName, container, follow, tailLines, ws); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
		return
	}
}

// ConnectPodTerminal 连接 Pod 终端
// @Summary 连接 Pod 终端
// @Tags K8s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cluster_id query string false "集群ID（推荐）"
// @Param cluster_name query string false "集群名称（推荐）"
// @Param namespace query string true "命名空间"
// @Param pod_name query string true "Pod名称"
// @Param container query string false "容器名称"
// @Param command query string false "执行的命令（默认/bin/sh）"
// @Success 200 {object} model.Response
// @Failure 400 {object} model.Response
// @Failure 500 {object} model.Response
// @Router /api/v1/kube/pod/ws/terminal [get]
func (h *K8sHandler) ConnectPodTerminal(c *gin.Context) {
	logger.Infof("[PodTerminal] 收到 Pod 终端连接请求: %s %s", c.Request.Method, c.Request.URL.String())

	// 验证 token（从 query 参数获取，因为 WebSocket 不能使用 Authorization header）
	token := c.Query("token")
	if token == "" {
		logger.Warnf("[PodTerminal] 缺少 token 参数")
		c.JSON(http.StatusUnauthorized, model.Error(401, "缺少 token 参数"))
		return
	}

	// 验证 token 并获取用户信息
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		logger.Errorf("[PodTerminal] Token 验证失败: %v", err)
		c.JSON(http.StatusUnauthorized, model.Error(401, "Token无效或已过期: "+err.Error()))
		return
	}
	logger.Infof("[PodTerminal] Token 验证成功，用户ID: %s", claims.UserID)

	clusterID := c.Query("cluster_id")
	clusterName := c.Query("cluster_name")
	namespace := c.Query("namespace")
	podName := c.Query("pod_name")
	container := c.Query("container")
	command := c.Query("command")

	logger.Infof("[PodTerminal] 请求参数: clusterID=%s, namespace=%s, podName=%s, container=%s, command=%s",
		clusterID, namespace, podName, container, command)

	if namespace == "" || podName == "" {
		logger.Warnf("[PodTerminal] 缺少必要参数: namespace=%s, podName=%s", namespace, podName)
		c.JSON(http.StatusBadRequest, model.Error(400, "namespace and pod_name are required"))
		return
	}

	// 权限检查（WebSocket 请求需要手动检查权限，终端需要 write 权限）
	if clusterID != "" {
		userID := claims.UserID
		roles, err := h.roleRepo.GetRolesByUserID(userID)
		if err != nil {
			roles = []model.Role{}
		}

		hasPermission := h.permissionService.CheckResourcePermission(userID, clusterID, namespace, k8sService.ResourceTypePod, podName, k8sService.ActionWrite, roles)
		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有访问该资源的权限"))
			return
		}
	}

	// 升级到 WebSocket
	logger.Infof("[PodTerminal] 开始升级到 WebSocket")
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("[PodTerminal] 升级到 WebSocket 失败: %v", err)
		c.JSON(http.StatusInternalServerError, model.Error(500, fmt.Sprintf("升级到 WebSocket 失败: %v", err)))
		return
	}
	defer ws.Close()

	logger.Infof("[PodTerminal] WebSocket 连接已建立: clusterID=%s, namespace=%s, pod=%s", clusterID, namespace, podName)

	// 创建终端服务
	terminalService := k8sService.NewPodTerminalService(h.service.(*k8sService.K8sService))

	// 获取用户名
	username := ""
	var user model.User
	if err := database.DB.First(&user, "id = ?", claims.UserID).Error; err == nil {
		username = user.Username
	}

	// 处理终端连接（这会阻塞直到连接关闭）
	logger.Infof("[PodTerminal] 开始处理终端连接")
	if err := terminalService.HandlePodTerminal(clusterID, clusterName, namespace, podName, container, command, claims.UserID, username, ws); err != nil {
		logger.Errorf("[PodTerminal] 处理 Pod 终端连接失败: %v", err)
		// 确保错误消息发送到客户端
		errorMsg := fmt.Sprintf("\r\n\x1b[31m[错误] %v\x1b[0m\r\n", err)
		ws.WriteMessage(websocket.TextMessage, []byte(errorMsg))
		// 等待一下确保消息发送完成
		time.Sleep(500 * time.Millisecond)
		return
	}

	logger.Infof("[PodTerminal] Pod 终端连接处理完成")
}
