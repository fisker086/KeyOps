package bastion

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProxyHandler Proxy处理器
type ProxyHandler struct {
	db *gorm.DB
}

// NewProxyHandler 创建Proxy处理器
func NewProxyHandler(db *gorm.DB) *ProxyHandler {
	return &ProxyHandler{db: db}
}

// RegisterProxy 注册Proxy
func (h *ProxyHandler) RegisterProxy(c *gin.Context) {
	var req struct {
		ProxyID   string    `json:"proxy_id" binding:"required"`
		HostName  string    `json:"host_name"`
		IP        string    `json:"ip"`
		Port      int       `json:"port"`
		Status    string    `json:"status"`
		Version   string    `json:"version"`
		StartTime time.Time `json:"start_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 检查是否已存在（使用 proxy_id 作为唯一标识）
	var existing model.Proxy
	result := h.db.Where("proxy_id = ?", req.ProxyID).First(&existing)

	if result.Error == nil {
		// 已存在，更新信息
		updates := map[string]interface{}{
			"host_name":      req.HostName,
			"ip":             req.IP,
			"port":           req.Port,
			"status":         req.Status,
			"version":        req.Version,
			"start_time":     req.StartTime,
			"last_heartbeat": time.Now(),
		}
		if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to update proxy"})
			return
		}

		// 记录日志：更新已存在的 proxy
		fmt.Printf("[RegisterProxy] Updated existing proxy: proxy_id=%s, id=%s, host=%s\n",
			req.ProxyID, existing.ID, req.HostName)
	} else if result.Error == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		proxy := model.Proxy{
			ID:            uuid.New().String(),
			ProxyID:       req.ProxyID,
			HostName:      req.HostName,
			IP:            req.IP,
			Port:          req.Port,
			Status:        req.Status,
			Version:       req.Version,
			StartTime:     req.StartTime,
			LastHeartbeat: time.Now(),
		}
		if err := h.db.Create(&proxy).Error; err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to register proxy"})
			return
		}

		// 记录日志：创建新的 proxy
		fmt.Printf("[RegisterProxy] Created new proxy: proxy_id=%s, id=%s, host=%s\n",
			req.ProxyID, proxy.ID, req.HostName)
	} else {
		// 其他数据库错误
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: fmt.Sprintf("Database error: %v", result.Error)})
		return
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "Proxy registered successfully",
	})
}

// Unregister 注销Proxy
func (h *ProxyHandler) Unregister(c *gin.Context) {
	var req struct {
		ProxyID string `json:"proxy_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 更新状态为offline
	if err := h.db.Model(&model.Proxy{}).
		Where("proxy_id = ?", req.ProxyID).
		Update("status", "offline").Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to unregister proxy"})
		return
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "Proxy unregistered successfully",
	})
}

// Heartbeat 心跳
func (h *ProxyHandler) Heartbeat(c *gin.Context) {
	var req struct {
		ProxyID   string `json:"proxy_id" binding:"required"`
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 更新心跳时间和状态
	updates := map[string]interface{}{
		"last_heartbeat": time.Now(),
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	result := h.db.Model(&model.Proxy{}).
		Where("proxy_id = ?", req.ProxyID).
		Updates(updates)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to update heartbeat"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "Proxy not found"})
		return
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "Heartbeat received",
	})
}

// CloseSession 关闭单个会话（实时上报）
func (h *ProxyHandler) CloseSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req struct {
		ProxyID   string    `json:"proxy_id" binding:"required"`
		Recording string    `json:"recording"`
		EndTime   time.Time `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 更新会话状态为已关闭
	result := h.db.Model(&model.SessionHistory{}).
		Where("session_id = ? AND proxy_id = ?", sessionID, req.ProxyID).
		Updates(map[string]interface{}{
			"status":    "closed",
			"end_time":  req.EndTime,
			"recording": req.Recording,
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to close session",
			"error":   result.Error.Error(),
		})
		return
	}

	// 注意：login_records 表是通过批量同步创建的，实时关闭会话时不更新它
	// 主要状态信息已在 session_histories 表中更新

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Session closed successfully",
		"data": gin.H{
			"session_id": sessionID,
			"status":     "closed",
		},
	})
}

// ReportSession 实时上报单个会话（供Proxy实时调用）
func (h *ProxyHandler) ReportSession(c *gin.Context) {
	var req struct {
		ProxyID string `json:"proxy_id" binding:"required"`
		Session struct {
			ProxyID      string    `json:"proxy_id"`
			SessionID    string    `json:"session_id"`
			HostID       string    `json:"host_id"`
			UserID       string    `json:"user_id"`
			Username     string    `json:"username"`
			HostIP       string    `json:"host_ip"`
			Status       string    `json:"status"`
			StartTime    time.Time `json:"start_time"`
			TerminalCols int       `json:"terminal_cols"`
			TerminalRows int       `json:"terminal_rows"`
		} `json:"session" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 创建统一的会话录制记录（标记为 webshell）
	sessionRecording := model.SessionRecording{
		ID:             req.Session.SessionID,
		SessionID:      req.Session.SessionID,
		ConnectionType: "webshell", // 标记为 webshell 连接
		ProxyID:        req.ProxyID,
		UserID:         req.Session.UserID,
		HostID:         req.Session.HostID,
		HostIP:         req.Session.HostIP,
		Username:       req.Session.Username,
		Status:         req.Session.Status,
		StartTime:      req.Session.StartTime,
		Duration:       "进行中",
		TerminalCols:   req.Session.TerminalCols,
		TerminalRows:   req.Session.TerminalRows,
	}

	// 保存到数据库（使用 FirstOrCreate 避免重复）
	result := h.db.Where("session_id = ?", sessionRecording.SessionID).FirstOrCreate(&sessionRecording)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save session",
			"error":   result.Error.Error(),
		})
		return
	}

	// 同时创建登录记录（虚拟机登录，host_id 必定存在）
	loginRecord := model.LoginRecord{
		ID:        sessionRecording.SessionID + "-login",
		UserID:    req.Session.UserID,
		HostID:    sessionRecording.HostID,
		HostName:  sessionRecording.HostName,
		HostIP:    sessionRecording.HostIP,
		Username:  sessionRecording.Username,
		LoginTime: sessionRecording.StartTime,
		Status:    "active", // 修正：使用正确的状态值
		SessionID: sessionRecording.SessionID,
	}
	h.db.Where("session_id = ?", loginRecord.SessionID).FirstOrCreate(&loginRecord)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Session reported successfully",
		"data": gin.H{
			"session_id": loginRecord.SessionID,
		},
	})
}

// SyncSessions 同步会话记录
func (h *ProxyHandler) SyncSessions(c *gin.Context) {
	// 使用 map 接收，手动解析时间
	var reqRaw struct {
		ProxyID string                   `json:"proxy_id" binding:"required"`
		Data    []map[string]interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&reqRaw); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 解析会话记录，支持多种时间格式
	sessions := make([]model.SessionRecording, 0, len(reqRaw.Data))
	timeFormats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999-07:00", // SQLite 格式
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05Z07:00",
	}

	for _, item := range reqRaw.Data {
		var sess model.SessionRecording
		sess.ConnectionType = "webshell" // 批量同步的都是 webshell
		sess.ProxyID = reqRaw.ProxyID

		if v, ok := item["session_id"].(string); ok {
			sess.ID = v // 使用 session_id 作为主键
			sess.SessionID = v
		}
		if v, ok := item["host_id"].(string); ok {
			sess.HostID = v
		}
		if v, ok := item["user_id"].(string); ok {
			sess.UserID = v
		}
		if v, ok := item["username"].(string); ok {
			sess.Username = v
		}
		if v, ok := item["host_ip"].(string); ok {
			sess.HostIP = v
		}
		if v, ok := item["status"].(string); ok {
			sess.Status = v
		}
		if v, ok := item["recording"].(string); ok {
			sess.Recording = v
		}
		if v, ok := item["terminal_cols"].(float64); ok {
			sess.TerminalCols = int(v)
		}
		if v, ok := item["terminal_rows"].(float64); ok {
			sess.TerminalRows = int(v)
		}

		// 解析 start_time
		if startTime, ok := item["start_time"].(string); ok {
			var parsed time.Time
			var err error
			for _, format := range timeFormats {
				parsed, err = time.Parse(format, startTime)
				if err == nil {
					sess.StartTime = parsed
					break
				}
			}
			if err != nil {
				c.JSON(http.StatusBadRequest, model.ErrorResponse{
					Error: "Invalid start_time format: " + startTime,
				})
				return
			}
		}

		// 解析 end_time (可选)
		if endTime, ok := item["end_time"].(string); ok && endTime != "" {
			var parsed time.Time
			var err error
			for _, format := range timeFormats {
				parsed, err = time.Parse(format, endTime)
				if err == nil {
					sess.EndTime = &parsed
					break
				}
			}
			if err != nil {
				c.JSON(http.StatusBadRequest, model.ErrorResponse{
					Error: "Invalid end_time format: " + endTime,
				})
				return
			}
		}

		// 计算持续时间
		if sess.EndTime != nil {
			diff := sess.EndTime.Sub(sess.StartTime)
			minutes := int(diff.Minutes())
			seconds := int(diff.Seconds()) % 60
			sess.Duration = fmt.Sprintf("%dm %ds", minutes, seconds)
		} else {
			sess.Duration = "进行中"
		}

		// 验证必填字段：跳过无效的会话记录
		if sess.SessionID == "" || sess.StartTime.IsZero() {
			continue // 跳过这条无效记录
		}

		sessions = append(sessions, sess)
	}

	if len(sessions) > 0 {
		// 使用批量插入（忽略重复）
		for _, sess := range sessions {
			h.db.Where("session_id = ?", sess.SessionID).FirstOrCreate(&sess)
		}

		// 同时创建登录记录
		loginRecords := make([]model.LoginRecord, 0, len(sessions))
		for _, sess := range sessions {
			loginRecord := model.LoginRecord{
				ID:         sess.SessionID + "-login",
				UserID:     sess.UserID,
				HostID:     sess.HostID,
				HostName:   sess.HostName,
				HostIP:     sess.HostIP,
				Username:   sess.Username,
				LoginTime:  sess.StartTime,
				LogoutTime: sess.EndTime,
				Status:     sess.Status,
				SessionID:  sess.SessionID,
			}

			// 计算时长
			if sess.EndTime != nil {
				duration := int(sess.EndTime.Sub(sess.StartTime).Seconds())
				loginRecord.Duration = &duration
			}

			loginRecords = append(loginRecords, loginRecord)
		}

		// 批量插入登录记录（忽略重复的）
		if len(loginRecords) > 0 {
			for _, record := range loginRecords {
				// 使用 FirstOrCreate 避免重复
				h.db.Where("session_id = ?", record.SessionID).FirstOrCreate(&record)
			}
		}
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "Sessions synced successfully",
		Data: gin.H{
			"count": len(sessions),
		},
	})
}

// ReportCommand 实时上报单个命令（供Proxy实时调用）
func (h *ProxyHandler) ReportCommand(c *gin.Context) {
	var req struct {
		ProxyID string `json:"proxy_id" binding:"required"`
		Command struct {
			ProxyID    string    `json:"proxy_id"`
			SessionID  string    `json:"session_id"`
			HostID     string    `json:"host_id"`
			UserID     string    `json:"user_id"`
			Username   string    `json:"username"`
			HostIP     string    `json:"host_ip"`
			Command    string    `json:"command"`
			Output     string    `json:"output"`
			ExitCode   int       `json:"exit_code"`
			ExecutedAt time.Time `json:"executed_at"`
		} `json:"command" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 创建统一的命令记录（ID是自增的，不需要手动设置）
	cmdRecord := model.CommandRecord{
		ProxyID:    req.ProxyID,
		SessionID:  req.Command.SessionID,
		HostID:     req.Command.HostID,
		UserID:     req.Command.UserID,
		HostIP:     req.Command.HostIP,
		Username:   req.Command.Username,
		Command:    req.Command.Command,
		Output:     req.Command.Output,
		ExitCode:   req.Command.ExitCode,
		ExecutedAt: req.Command.ExecutedAt,
	}

	// 保存到数据库
	if err := h.db.Create(&cmdRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save command",
			"error":   err.Error(),
		})
		return
	}

	// 更新会话的命令计数
	h.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", req.Command.SessionID).
		Update("command_count", gorm.Expr("command_count + 1"))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Command reported successfully",
		"data": gin.H{
			"command_id": cmdRecord.ID,
		},
	})
}

// SyncCommands 同步命令记录
func (h *ProxyHandler) SyncCommands(c *gin.Context) {
	// 使用 map 接收，手动解析时间
	var reqRaw struct {
		ProxyID string                   `json:"proxy_id" binding:"required"`
		Data    []map[string]interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&reqRaw); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// 解析命令记录，支持多种时间格式（ID是自增的，不需要手动设置）
	commands := make([]model.CommandRecord, 0, len(reqRaw.Data))
	for _, item := range reqRaw.Data {
		var cmd model.CommandRecord
		// ID是自增的，不需要手动设置

		if v, ok := item["proxy_id"].(string); ok {
			cmd.ProxyID = v
		}
		if v, ok := item["session_id"].(string); ok {
			cmd.SessionID = v
		}
		if v, ok := item["host_id"].(string); ok {
			cmd.HostID = v
		}
		if v, ok := item["user_id"].(string); ok {
			cmd.UserID = v
		}
		if v, ok := item["username"].(string); ok {
			cmd.Username = v
		}
		if v, ok := item["host_ip"].(string); ok {
			cmd.HostIP = v
		}
		if v, ok := item["command"].(string); ok {
			cmd.Command = v
		}
		if v, ok := item["output"].(string); ok {
			cmd.Output = v
		}
		if v, ok := item["exit_code"].(float64); ok {
			cmd.ExitCode = int(v)
		}
		if v, ok := item["duration_ms"].(float64); ok {
			cmd.DurationMs = int64(v)
		}

		// 解析时间，支持多种格式
		if execTime, ok := item["executed_at"].(string); ok {
			// 尝试多种时间格式
			formats := []string{
				time.RFC3339,
				time.RFC3339Nano,
				"2006-01-02 15:04:05.999999-07:00", // SQLite 格式
				"2006-01-02 15:04:05-07:00",
				"2006-01-02T15:04:05Z07:00",
			}

			var parsed time.Time
			var err error
			for _, format := range formats {
				parsed, err = time.Parse(format, execTime)
				if err == nil {
					cmd.ExecutedAt = parsed
					break
				}
			}

			if err != nil {
				c.JSON(http.StatusBadRequest, model.ErrorResponse{
					Error: "Invalid executed_at format: " + execTime,
				})
				return
			}
		}

		// 验证必填字段：跳过无效的命令记录
		if cmd.ExecutedAt.IsZero() {
			continue // 跳过这条无效记录
		}

		commands = append(commands, cmd)
	}

	if len(commands) > 0 {
		// 使用批量插入
		if err := h.db.CreateInBatches(commands, 100).Error; err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to sync commands"})
			return
		}

		// 更新每个会话的命令计数
		sessionCounts := make(map[string]int)
		for _, cmd := range commands {
			sessionCounts[cmd.SessionID]++
		}
		for sessionID, count := range sessionCounts {
			h.db.Model(&model.SessionRecording{}).
				Where("session_id = ?", sessionID).
				Update("command_count", gorm.Expr("command_count + ?", count))
		}
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "Commands synced successfully",
		Data: gin.H{
			"count": len(commands),
		},
	})
}

// ListProxies 列出所有Proxy
func (h *ProxyHandler) ListProxies(c *gin.Context) {
	var proxies []model.Proxy
	if err := h.db.Order("created_at DESC").Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Failed to list proxies"})
		return
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Data: proxies,
	})
}

// GetProxyStats 获取Proxy统计信息
func (h *ProxyHandler) GetProxyStats(c *gin.Context) {
	proxyID := c.Param("proxy_id")

	var stats struct {
		TotalSessions  int64 `json:"total_sessions"`
		ActiveSessions int64 `json:"active_sessions"`
		TotalCommands  int64 `json:"total_commands"`
	}

	// 统计会话
	h.db.Model(&model.SessionHistory{}).
		Where("proxy_id = ?", proxyID).
		Count(&stats.TotalSessions)

	h.db.Model(&model.SessionHistory{}).
		Where("proxy_id = ? AND status = ?", proxyID, "active").
		Count(&stats.ActiveSessions)

	// 统计命令
	h.db.Model(&model.CommandHistory{}).
		Where("proxy_id = ?", proxyID).
		Count(&stats.TotalCommands)

	c.JSON(http.StatusOK, model.SuccessResponse{
		Data: stats,
	})
}
