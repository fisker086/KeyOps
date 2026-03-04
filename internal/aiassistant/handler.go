package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/aiassistant/tools"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/fisker086/keyops/pkg/distributed"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/fisker086/keyops/pkg/redis"
)

// streamEvent 用于 SSE 推送
type streamEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// GetUserRoleIDsFunc 根据用户ID返回其所属平台角色ID列表，用于专家可见/可用校验
type GetUserRoleIDsFunc func(userID string) ([]string, error)

// Handler AI运维助手 HTTP + SSE
type Handler struct {
	sessionMgr      *SessionManager
	scheduleMgr     *ScheduleManager
	envMgr          *EnvManager
	store           *Store // 可选：DB 存目标环境与专家，为 nil 时用 config/内置
	streamHub        *streamHub
	agentStarted     sync.Map // sessionID -> struct{}，保证每会话只启动一次 runAgent
	jwtSecret        string
	getUserRoleIDs   GetUserRoleIDsFunc       // 可选：非 nil 时按用户角色过滤专家列表并校验创建任务时的专家可用性
	runners          map[string]tools.Runner   // 各工具集执行器（如 k8s），key 为工具集 ID
	reportSender     InspectionReportSender   // 可选：巡检完成后将报告发送到告警渠道
	scheduleStarter   ScheduleWorkflowStarter  // 可选：Temporal 等，触发时走工作流（原子 巡检+发报告）
	settingRepo      *repository.SettingRepository // 可选：用于从系统设置读取 LLM 配置
}

// NewHandler 创建。store 可选；getUserRoleIDs 可选；runners 为各工具集 ID 到 Runner 的映射；settingRepo 可选，用于从系统设置读取 LLM Key/URL。
func NewHandler(sessionMgr *SessionManager, scheduleMgr *ScheduleManager, envMgr *EnvManager, store *Store, jwtSecret string, getUserRoleIDs GetUserRoleIDsFunc, runners map[string]tools.Runner, settingRepo *repository.SettingRepository) *Handler {
	return &Handler{
		sessionMgr:    sessionMgr,
		scheduleMgr:   scheduleMgr,
		envMgr:        envMgr,
		store:         store,
		streamHub:     newStreamHub(),
		jwtSecret:     jwtSecret,
		getUserRoleIDs: getUserRoleIDs,
		runners:       runners,
		settingRepo:   settingRepo,
	}
}

// SetInspectionReportSender 设置巡检报告发送器（用于定时任务跑完后将报告发到告警渠道）
func (h *Handler) SetInspectionReportSender(sender InspectionReportSender) {
	h.reportSender = sender
}

// SetScheduleWorkflowStarter 设置定时任务工作流启动器（如 Temporal）；非 nil 时触发走工作流
func (h *Handler) SetScheduleWorkflowStarter(starter ScheduleWorkflowStarter) {
	h.scheduleStarter = starter
}

// GetSchedule 获取定时任务（供 Temporal Activity 等使用）
func (h *Handler) GetSchedule(id string) (*Schedule, error) {
	return h.scheduleMgr.GetSchedule(id)
}

// CreateSessionForSchedule 为定时任务创建会话并返回 sessionID（供 Temporal Activity 使用）
func (h *Handler) CreateSessionForSchedule(scheduleID, createdBy string) (string, error) {
	s, err := h.scheduleMgr.GetSchedule(scheduleID)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("schedule not found: %s", scheduleID)
	}
	modelName := ""
	if s.ModelID != "" && h.store != nil {
		if mc, _ := h.store.GetModel(s.ModelID); mc != nil {
			modelName = mc.Name
		}
	}
	return h.sessionMgr.CreateSession(s.TaskPrompt, s.EnvID, s.Role, s.ModelID, modelName, scheduleID, createdBy)
}

// RunAgentCore 同步执行巡检（供 Temporal Activity 使用）
func (h *Handler) RunAgentCore(sessionID string) {
	h.runAgentCore(sessionID, "")
}

// SendReportIfNeeded 若配置了渠道则发送巡检报告（供 Temporal Activity 使用）
func (h *Handler) SendReportIfNeeded(sessionID string) {
	h.sendReportIfNeeded(sessionID)
}

// RegisterRoutes 注册到 gin 路由组（需在已认证组下）
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/ai-assistant")
	{
		g.GET("/models", h.listModels)
		g.GET("/model-configs", h.listModelConfigs)
		g.GET("/models/:id", h.getModel)
		g.POST("/models", h.addModel)
		g.PUT("/models/:id", h.updateModel)
		g.DELETE("/models/:id", h.deleteModel)
		g.GET("/environments", h.listEnvironments)
		g.POST("/environments", h.addEnvironment)
		g.PUT("/environments/:id", h.updateEnvironment)
		g.DELETE("/environments/:id", h.deleteEnvironment)
		g.GET("/experts", h.listExperts)
		g.POST("/experts", h.addExpert)
		g.PUT("/experts/:id", h.updateExpert)
		g.DELETE("/experts/:id", h.deleteExpert)
		g.GET("/sessions", h.listSessions)
		g.GET("/sessions/:session_id/stream", h.handleStream) // SSE，须在 /sessions/:session_id 前注册
		g.GET("/sessions/:session_id", h.getSession)
		g.DELETE("/sessions/:session_id", h.deleteSession)
		g.POST("/sessions/:session_id/continue", h.continueSession)
		g.POST("/tasks", h.createTask)
		g.GET("/schedules", h.listSchedules)
		g.POST("/schedules", h.addSchedule)
		g.GET("/schedules/:schedule_id/sessions", h.listScheduleSessions)
		g.POST("/schedules/:schedule_id/trigger", h.triggerSchedule)
		g.PUT("/schedules/:schedule_id", h.updateSchedule)
		g.DELETE("/schedules/:schedule_id", h.deleteSchedule)
	}
}

func (h *Handler) listModels(c *gin.Context) {
	// 优先从 DB 模型配置读取；无配置时回退到 settings
	if h.store != nil {
		list, err := h.store.ListModels()
		if err == nil && len(list) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"models":          list,
				"default_model":  list[0].ID,
			})
			return
		}
	}
	models, defaultModel := GetAvailableModels(h.settingRepo)
	c.JSON(http.StatusOK, gin.H{
		"models":         models,
		"default_model":  defaultModel,
	})
}

func (h *Handler) listModelConfigs(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, []ModelConfig{})
		return
	}
	list, err := h.store.ListModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) getModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model config not available"})
		return
	}
	mc, err := h.store.GetModel(id)
	if err != nil || mc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model config not found"})
		return
	}
	c.JSON(http.StatusOK, mc)
}

func (h *Handler) addModel(c *gin.Context) {
	var req ModelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.Model == "" || req.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, model, api_key required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model config not available"})
		return
	}
	if err := h.store.CreateModel(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": req.ID})
}

func (h *Handler) updateModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	var req ModelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.ID = id
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model config not available"})
		return
	}
	if err := h.store.UpdateModel(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) deleteModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model config not available"})
		return
	}
	if err := h.store.DeleteModel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) listEnvironments(c *gin.Context) {
	if h.store != nil {
		var userRoleIDs []string
		if h.getUserRoleIDs != nil {
			if userID, exists := c.Get("userID"); exists {
				if uid, ok := userID.(string); ok && uid != "" {
					if ids, err := h.getUserRoleIDs(uid); err == nil {
						userRoleIDs = ids
					}
				}
			}
		}
		list, err := h.store.ListEnvironments(userRoleIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list environments"})
			return
		}
		for i := range list {
			list[i].EnabledSkills = GetEnabledSkills(&list[i], h.runners)
		}
		c.JSON(http.StatusOK, list)
		return
	}
	list := h.envMgr.ListEnvironments()
	for i := range list {
		list[i].EnabledSkills = GetEnabledSkills(&list[i], h.runners)
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) getEnvironment(envID string) *Environment {
	if h.store != nil {
		if e, _ := h.store.GetEnvironment(envID); e != nil {
			return e
		}
	}
	return h.envMgr.GetEnvironment(envID)
}

func (h *Handler) listSessions(c *gin.Context) {
	scheduleID := c.Query("schedule_id")
	createdBy := ""
	if u, exists := c.Get("username"); exists {
		createdBy, _ = u.(string)
	}
	list, err := h.sessionMgr.ListSessions(scheduleID, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) getSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !h.sessionMgr.IsValidSessionID(sessionID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}
	s, err := h.sessionMgr.GetSession(sessionID)
	if err != nil || s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) deleteSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !h.sessionMgr.IsValidSessionID(sessionID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}
	if err := h.sessionMgr.DeleteSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

type continueSessionReq struct {
	Task string `json:"task"`
}

// continueSession 在同一会话中继续对话（多轮）
func (h *Handler) continueSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !h.sessionMgr.IsValidSessionID(sessionID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}
	var req continueSessionReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Task == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task required"})
		return
	}
	s, err := h.sessionMgr.GetSession(sessionID)
	if err != nil || s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	if s.Status == "running" {
		c.JSON(http.StatusConflict, gin.H{"error": "任务正在执行中，请稍后再试"})
		return
	}
	// 允许从 completed 或 error 状态继续
	if err := h.sessionMgr.ContinueSession(sessionID, req.Task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.agentStarted.Store(sessionID, struct{}{}) // 标记已启动，避免 stream 连接时重复启动
	go func() {
		defer h.agentStarted.Delete(sessionID)
		h.runAgentCore(sessionID, req.Task)
		h.sendReportIfNeeded(sessionID)
	}()
	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}

type createTaskReq struct {
	Task     string `json:"task"`
	EnvID    string `json:"env_id"`
	Role     string `json:"role"`
	ModelID  string `json:"model_id"` // 可选，模型配置 ID
}

func (h *Handler) createTask(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Task == "" || req.EnvID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task and env_id required"})
		return
	}
	if req.Role == "" {
		req.Role = "sre"
	}
	env := h.getEnvironment(req.EnvID)
	if env == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}
	// 校验当前用户是否有权使用所选目标环境（与平台角色关联）
	if h.store != nil && h.getUserRoleIDs != nil && len(env.AllowedRoleIDs) > 0 {
		allowed := false
		if userID, exists := c.Get("userID"); exists {
			if uid, ok := userID.(string); ok && uid != "" {
				roleIDs, err := h.getUserRoleIDs(uid)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user roles"})
					return
				}
				for _, a := range env.AllowedRoleIDs {
					for _, r := range roleIDs {
						if a == r {
							allowed = true
							break
						}
					}
					if allowed {
						break
					}
				}
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "您所在的平台角色无权使用该目标环境"})
			return
		}
	}
	// 校验当前用户是否有权使用所选专家角色（与平台角色关联）
	if h.store != nil && h.getUserRoleIDs != nil {
		ex, err := h.store.GetExpert(req.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if ex != nil && len(ex.AllowedRoleIDs) > 0 {
			allowed := false
			if userID, exists := c.Get("userID"); exists {
				if uid, ok := userID.(string); ok && uid != "" {
					roleIDs, err := h.getUserRoleIDs(uid)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user roles"})
						return
					}
					for _, a := range ex.AllowedRoleIDs {
						for _, r := range roleIDs {
							if a == r {
								allowed = true
								break
							}
						}
						if allowed {
							break
						}
					}
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{"error": "您所在的平台角色无权使用该专家角色"})
				return
			}
		}
	}
	createdBy := ""
	if u, exists := c.Get("username"); exists {
		createdBy, _ = u.(string)
	}
	modelName := ""
	if req.ModelID != "" && h.store != nil {
		if mc, _ := h.store.GetModel(req.ModelID); mc != nil {
			modelName = mc.Name
		}
	}
	sessionID, err := h.sessionMgr.CreateSession(req.Task, req.EnvID, req.Role, req.ModelID, modelName, "", createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}

func (h *Handler) listExperts(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, []Expert{})
		return
	}
	var userRoleIDs []string
	if h.getUserRoleIDs != nil {
		if userID, exists := c.Get("userID"); exists {
			if uid, ok := userID.(string); ok && uid != "" {
				ids, err := h.getUserRoleIDs(uid)
				if err == nil {
					userRoleIDs = ids
				}
			}
		}
	}
	list, err := h.store.ListExperts(userRoleIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) listSchedules(c *gin.Context) {
	list, err := h.scheduleMgr.ListSchedules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

type scheduleReq struct {
	Name                   string  `json:"name"`
	EnvID                  string  `json:"env_id"`
	ModelID                string  `json:"model_id"`
	Cron                   string  `json:"cron"`
	TaskPrompt             string  `json:"task_prompt"`
	Role                   string  `json:"role"`
	LarkBotID              string  `json:"lark_bot_id"`
	LarkGroupName          string  `json:"lark_group_name"`
	LarkFolderID           string  `json:"lark_folder_id"`
	Enabled                *bool   `json:"enabled"`
	ResponsibleUser        string  `json:"responsible_user"`
	NotificationChannelIDs []uint  `json:"notification_channel_ids"` // 监控告警渠道 ID，巡检结果通知到这些渠道
}

func (h *Handler) addSchedule(c *gin.Context) {
	var req scheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	id, err := h.scheduleMgr.AddSchedule(req.Name, req.EnvID, req.ModelID, req.Cron, req.TaskPrompt, req.Role, req.LarkBotID, req.LarkGroupName, req.LarkFolderID, req.ResponsibleUser, enabled, req.NotificationChannelIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"schedule_id": id})
}

func (h *Handler) listScheduleSessions(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	createdBy := ""
	if u, exists := c.Get("username"); exists {
		createdBy, _ = u.(string)
	}
	list, err := h.sessionMgr.ListSessions(scheduleID, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// scheduleRunLockTTL 定时任务执行锁的有效期（执行期间持有，避免多机重复执行）
// 锁带 TTL 且会续期；进程崩溃后续期停止，Redis 键最多 scheduleRunLockTTL 后自动过期，不会永久占用
const scheduleRunLockTTL = 15 * time.Minute

// ErrScheduleAlreadyRunning 表示该定时任务已在执行（上一周期未结束或其它实例持有锁），cron 可视为跳过
var ErrScheduleAlreadyRunning = errors.New("该定时任务正在其他实例上执行")

// TriggerScheduleByID 由 cron 或内部调用，按 scheduleID 触发一次；idempotencyKey 非空时用于 Temporal 去重（多实例同一时间槽只启一次）
func (h *Handler) TriggerScheduleByID(ctx context.Context, scheduleID string) error {
	s, err := h.scheduleMgr.GetSchedule(scheduleID)
	if err != nil || s == nil {
		return err
	}
	if h.scheduleStarter != nil {
		// cron 触发时传时间槽作为幂等键，多实例下同一分钟只会有一次 workflow
		key := time.Now().Truncate(time.Minute).Format("200601021504")
		return h.scheduleStarter.StartScheduleRun(ctx, scheduleID, key)
	}
	lockKey := "ai_assistant:schedule_run:" + scheduleID
	lock := distributed.NewRedisLock(redis.GetClient(), lockKey, scheduleRunLockTTL)
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok && redis.IsEnabled() {
		return ErrScheduleAlreadyRunning
	}
	createdBy := s.ResponsibleUser
	if createdBy == "" {
		createdBy = "system"
	}
	modelName := ""
	if s.ModelID != "" && h.store != nil {
		if mc, _ := h.store.GetModel(s.ModelID); mc != nil {
			modelName = mc.Name
		}
	}
	sessionID, err := h.sessionMgr.CreateSession(s.TaskPrompt, s.EnvID, s.Role, s.ModelID, modelName, scheduleID, createdBy)
	if err != nil {
		if ok {
			_ = lock.Unlock()
		}
		return err
	}
	go func() {
		defer func() {
			if ok {
				_ = lock.Unlock()
			}
		}()
		h.runAgentCore(sessionID, "")
		h.sendReportIfNeeded(sessionID)
	}()
	return nil
}

func (h *Handler) triggerSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	s, err := h.scheduleMgr.GetSchedule(scheduleID)
	if err != nil || s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
		return
	}
	// 若配置了 Temporal 等工作流启动器，则原子执行「巡检 + 发报告」由工作流保证；手动触发不传幂等键
	if h.scheduleStarter != nil {
		if err := h.scheduleStarter.StartScheduleRun(c.Request.Context(), scheduleID, ""); err != nil {
			logger.Warnf("ai_assistant schedule workflow start error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start schedule workflow: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "workflow": true})
		return
	}
	// 多机部署时用 Redis 分布式锁保证同一定时任务每次只在一台机器上执行
	lockKey := "ai_assistant:schedule_run:" + scheduleID
	lock := distributed.NewRedisLock(redis.GetClient(), lockKey, scheduleRunLockTTL)
	ok, err := lock.TryLock()
	if err != nil {
		logger.Warnf("ai_assistant schedule lock error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acquire schedule lock"})
		return
	}
	// 手动触发且加锁失败时，若传 force=1 则先删除可能残留的锁（上一轮异常退出未释放），再重试一次；仅在确认上次已结束时使用
	if !ok && redis.IsEnabled() {
		if c.Query("force") == "1" {
			client := redis.GetClient()
			if client != nil {
				_ = client.Del(c.Request.Context(), lockKey).Err()
				lock = distributed.NewRedisLock(client, lockKey, scheduleRunLockTTL)
				ok, err = lock.TryLock()
				if err != nil {
					logger.Warnf("ai_assistant schedule lock error (after force clear): %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acquire schedule lock"})
					return
				}
			}
		}
		if !ok {
			c.JSON(http.StatusConflict, gin.H{"error": "该定时任务正在其他实例上执行，请稍后再试（若确认上次已结束可加 ?force=1 强制触发）"})
			return
		}
	}
	createdBy := s.ResponsibleUser
	if u, exists := c.Get("username"); exists {
		createdBy, _ = u.(string)
	}
	modelName := ""
	if s.ModelID != "" && h.store != nil {
		if mc, _ := h.store.GetModel(s.ModelID); mc != nil {
			modelName = mc.Name
		}
	}
	sessionID, err := h.sessionMgr.CreateSession(s.TaskPrompt, s.EnvID, s.Role, s.ModelID, modelName, scheduleID, createdBy)
	if err != nil {
		if ok {
			_ = lock.Unlock()
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 执行结束后释放锁；跑完巡检后若有配置渠道则发送报告（原子性：先跑完再发）
	go func() {
		defer func() {
			if ok {
				_ = lock.Unlock()
			}
		}()
		h.runAgentCore(sessionID, "")
		h.sendReportIfNeeded(sessionID)
	}()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "session_id": sessionID})
}

func (h *Handler) updateSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	var req scheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.EnvID != "" {
		updates["env_id"] = req.EnvID
	}
	if req.ModelID != "" {
		updates["model_id"] = req.ModelID
	}
	if req.Cron != "" {
		updates["cron"] = req.Cron
	}
	if req.TaskPrompt != "" {
		updates["task_prompt"] = req.TaskPrompt
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	updates["lark_bot_id"] = req.LarkBotID
	updates["lark_group_name"] = req.LarkGroupName
	updates["lark_folder_id"] = req.LarkFolderID
	updates["responsible_user"] = req.ResponsibleUser
	updates["notification_channel_ids"] = req.NotificationChannelIDs
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if err := h.scheduleMgr.UpdateSchedule(scheduleID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) deleteSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if err := h.scheduleMgr.DeleteSchedule(scheduleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ---------- 目标环境 CRUD（需 store 非 nil）----------
// 仅支持三种技能：Prometheus（prom_url）、Grafana（graf_url/graf_token）、K8s（k8s_cluster_id）
type environmentReq struct {
	Name           string                 `json:"name"`
	PromURL        string                 `json:"prom_url"`
	GrafURL        string                 `json:"graf_url"`
	GrafToken      string                 `json:"graf_token"`
	K8sClusterID   string                 `json:"k8s_cluster_id"` // 关联 K8s 集群 ID（来自 k8s 管理）
	AllowedRoleIDs []string               `json:"allowed_role_ids"`
	ExtraConfig    map[string]interface{} `json:"extra_config"`    // 扩展配置，如 K8s 安装节点（master/worker/etcd）
}

func (h *Handler) addEnvironment(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "environments are read-only (no DB)"})
		return
	}
	var req environmentReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	e := &Environment{
		Name:           req.Name,
		PromURL:        req.PromURL,
		GrafURL:        req.GrafURL,
		GrafToken:      req.GrafToken,
		K8sClusterID:   req.K8sClusterID,
		AllowedRoleIDs: req.AllowedRoleIDs,
		ExtraConfig:    req.ExtraConfig,
	}
	if err := h.store.CreateEnvironment(e); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": e.ID})
}

func (h *Handler) updateEnvironment(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "environments are read-only (no DB)"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	var req environmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	e := &Environment{
		ID:             id,
		Name:           req.Name,
		PromURL:        req.PromURL,
		GrafURL:        req.GrafURL,
		GrafToken:      req.GrafToken,
		K8sClusterID:   req.K8sClusterID,
		AllowedRoleIDs: req.AllowedRoleIDs,
		ExtraConfig:    req.ExtraConfig,
	}
	if err := h.store.UpdateEnvironment(e); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) deleteEnvironment(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "environments are read-only (no DB)"})
		return
	}
	id := c.Param("id")
	if err := h.store.DeleteEnvironment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ---------- 专家角色 CRUD（需 store 非 nil）----------
type expertReq struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SystemPrompt   string   `json:"system_prompt"`
	SkillID        string   `json:"skill_id"`        // 关联技能（如 k8s-install），非空时前置注入
	AllowedRoleIDs []string `json:"allowed_role_ids"` // 允许使用该专家的平台角色ID列表，空表示所有角色可用
}

func (h *Handler) addExpert(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "experts are read-only (no DB)"})
		return
	}
	var req expertReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.SystemPrompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and system_prompt required"})
		return
	}
	e := &Expert{
		Name:           req.Name,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		SkillID:        strings.TrimSpace(req.SkillID),
		AllowedRoleIDs: req.AllowedRoleIDs,
	}
	if err := h.store.CreateExpert(e); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": e.ID})
}

func (h *Handler) updateExpert(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "experts are read-only (no DB)"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	var req expertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	e := &Expert{
		ID:             id,
		Name:           req.Name,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		SkillID:        strings.TrimSpace(req.SkillID),
		AllowedRoleIDs: req.AllowedRoleIDs,
	}
	if err := h.store.UpdateExpert(e); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) deleteExpert(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "experts are read-only (no DB)"})
		return
	}
	id := c.Param("id")
	if err := h.store.DeleteExpert(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// tryStartAgent 保证同一 session 只启动一次 runAgent
func (h *Handler) tryStartAgent(sessionID string) {
	if _, loaded := h.agentStarted.LoadOrStore(sessionID, struct{}{}); loaded {
		return
	}
	go h.runAgent(sessionID)
}

// runAgent 在后台执行 agent 并推送步骤到 session 与 SSE（供 tryStartAgent / handleStream 调用）
func (h *Handler) runAgent(sessionID string) {
	h.runAgentCore(sessionID, "")
}

// runAgentCore 同步执行 agent，推送步骤到 session 与 SSE。taskOverride 非空时用于续聊（多轮对话）
func (h *Handler) runAgentCore(sessionID string, taskOverride string) {
	s, err := h.sessionMgr.GetSession(sessionID)
	if err != nil || s == nil || s.Status != "running" {
		return
	}
	task := s.Task
	if taskOverride != "" {
		task = taskOverride
	}
	env := h.getEnvironment(s.EnvID)
	if env == nil {
		_ = h.sessionMgr.AddStep(sessionID, "error", map[string]interface{}{"message": "environment not found"})
		h.streamHub.broadcast(sessionID, "error", map[string]interface{}{"message": "environment not found"})
		return
	}
	// 优先使用数据库模型配置：session 指定 model_id > 数据库首个模型 > settings/config 回退
	var llmConfig *LLMConfig
	if h.store != nil {
		modelID := s.ModelID
		if modelID == "" {
			// 未传 model_id 时，若数据库有模型配置则用第一个（避免前端未选择导致走错误回退）
			list, _ := h.store.ListModels()
			if len(list) > 0 {
				modelID = list[0].ID
			}
		}
		if modelID != "" {
			mc, _ := h.store.GetModel(modelID)
			if mc != nil && mc.APIKey != "" {
				llmConfig = &LLMConfig{
					APIKey:   mc.APIKey,
					BaseURL:  mc.BaseURL,
					Model:    mc.Model,
					MaxSteps: mc.MaxSteps,
					ProxyURL: mc.ProxyURL,
				}
				if llmConfig.MaxSteps <= 0 {
					llmConfig.MaxSteps = 30
				}
			}
		}
	}
	if llmConfig == nil {
		// 不再回退 config/settings，统一使用数据库模型配置
		_ = h.sessionMgr.AddStep(sessionID, "error", map[string]interface{}{"message": "请先在模型配置中添加至少一个模型"})
		h.streamHub.broadcast(sessionID, "error", map[string]interface{}{"message": "请先在模型配置中添加至少一个模型"})
		return
	}
	engine, err := NewEngine(env, h.runners, llmConfig)
	if err != nil {
		_ = h.sessionMgr.AddStep(sessionID, "error", map[string]interface{}{"message": err.Error()})
		h.streamHub.broadcast(sessionID, "error", map[string]interface{}{"message": err.Error()})
		return
	}
	callback := func(stepType string, data map[string]interface{}) {
		_ = h.sessionMgr.AddStep(sessionID, stepType, data)
		h.streamHub.broadcast(sessionID, stepType, data)
	}
	systemPrompt := ""
	if h.store != nil {
		if ex, _ := h.store.GetExpert(s.Role); ex != nil {
			// 技能知识库在前，数据库 system_prompt 在后（可覆盖）。由 expert.skill_id 驱动，无需硬编码角色
			if skill := GetSkillContent(ex.SkillID); skill != "" {
				systemPrompt = skill + "\n\n---\n\n" + ex.SystemPrompt
			} else {
				systemPrompt = ex.SystemPrompt
			}
			// 运行时统一增强：巡检/排障类专家在给出 RCA 时需附可执行修复命令。
			// 用户明确授权且存在可执行类工具时，可执行低风险修复并反馈结果。
			if s.Role == "sre" || s.Role == "inspector" || s.Role == "k8s-expert" {
				systemPrompt += "\n\n## 运行时增强约束（必须遵守）：\n" +
					"1. 你的结论必须结合当前目标环境与 Observation 指标，给出可执行的修复命令（按系统/集群环境区分）。\n" +
					"2. 当用户明确授权“请直接执行/帮我处理”且当前环境存在可执行类工具时，可执行低风险修复动作；执行前先说明动作与风险，执行后回报结果与验证。\n" +
					"3. 若当前环境无可执行类工具或动作风险过高，必须明确说明限制，并提供人工执行命令、回滚命令与验证命令。"
			}
		}
	}
	engine.Run(context.Background(), task, s.Role, systemPrompt, callback)
	h.streamHub.broadcast(sessionID, "system", map[string]interface{}{"message": "Task thread completed"})
}

// sendReportIfNeeded 若该会话属于定时任务且配置了告警渠道，则发送巡检报告
func (h *Handler) sendReportIfNeeded(sessionID string) {
	if h.reportSender == nil {
		return
	}
	s, err := h.sessionMgr.GetSession(sessionID)
	if err != nil || s == nil || s.ScheduleID == "" {
		return
	}
	schedule, err := h.scheduleMgr.GetSchedule(s.ScheduleID)
	if err != nil || schedule == nil || len(schedule.NotificationChannelIDs) == 0 {
		return
	}
	summary := h.buildSessionSummary(s)
	_ = h.reportSender.SendInspectionReport(schedule.NotificationChannelIDs, schedule.Name, s.SessionID, s.Status, summary)
}

// buildSessionSummary 从会话生成报告摘要（结论 + 步骤数）
func (h *Handler) buildSessionSummary(s *Session) string {
	if s.FinalAnswer != "" {
		return "结论：\n" + s.FinalAnswer + "\n\n步骤数：" + fmt.Sprintf("%d", len(s.Steps))
	}
	return "步骤数：" + fmt.Sprintf("%d", len(s.Steps))
}

// handleStream SSE：订阅会话步骤流，使用 Authorization header 鉴权
func (h *Handler) handleStream(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !h.sessionMgr.IsValidSessionID(sessionID) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	ch, unregister := h.streamHub.subscribe(sessionID)
	defer unregister()

	s, _ := h.sessionMgr.GetSession(sessionID)
	if s != nil && s.Status == "running" {
		h.tryStartAgent(sessionID)
	}
	// 回放已有步骤
	if s != nil && len(s.Steps) > 0 {
		for _, step := range s.Steps {
			evt := streamEvent{Type: step.Type, Data: step.Data}
			if step.Data == nil {
				evt.Data = map[string]interface{}{}
			}
			writeSSE(c, evt)
			c.Writer.Flush()
		}
	}
	// 已结束的会话只回放不等待新事件
	if s != nil && s.Status != "running" {
		return
	}

	ctx := c.Request.Context()
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(c, evt)
			c.Writer.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func writeSSE(c *gin.Context, evt streamEvent) {
	data, _ := json.Marshal(evt)
	c.SSEvent(evt.Type, string(data))
}

// streamHub 按 session_id 向 SSE 订阅者广播
type streamHub struct {
	mu      sync.RWMutex
	clients map[string]map[chan streamEvent]struct{}
}

func newStreamHub() *streamHub {
	return &streamHub{clients: make(map[string]map[chan streamEvent]struct{})}
}

func (hub *streamHub) subscribe(sessionID string) (ch chan streamEvent, unregister func()) {
	ch = make(chan streamEvent, 64)
	hub.mu.Lock()
	if hub.clients[sessionID] == nil {
		hub.clients[sessionID] = make(map[chan streamEvent]struct{})
	}
	hub.clients[sessionID][ch] = struct{}{}
	hub.mu.Unlock()
	unregister = func() {
		hub.mu.Lock()
		if m := hub.clients[sessionID]; m != nil {
			delete(m, ch)
			if len(m) == 0 {
				delete(hub.clients, sessionID)
			}
		}
		hub.mu.Unlock()
		close(ch)
	}
	return ch, unregister
}

func (hub *streamHub) broadcast(sessionID, stepType string, data map[string]interface{}) {
	evt := streamEvent{Type: stepType, Data: data}
	if data == nil {
		evt.Data = map[string]interface{}{}
	}
	hub.mu.RLock()
	subs := hub.clients[sessionID]
	if subs == nil {
		hub.mu.RUnlock()
		return
	}
	chans := make([]chan streamEvent, 0, len(subs))
	for ch := range subs {
		chans = append(chans, ch)
	}
	hub.mu.RUnlock()
	for _, ch := range chans {
		select {
		case ch <- evt:
		default:
			logger.Warnf("aiassistant sse: drop event for session %s (channel full)", sessionID)
		}
	}
}
