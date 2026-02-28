package release

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/service/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReleaseHandler struct {
	service      *release.Service
	pipelineRepo *repository.ReleasePipelineDefinitionRepository
	bindingRepo  *repository.ApplicationDeployBindingRepository
}

func NewReleaseHandler(service *release.Service, pipelineRepo *repository.ReleasePipelineDefinitionRepository, bindingRepo *repository.ApplicationDeployBindingRepository) *ReleaseHandler {
	return &ReleaseHandler{service: service, pipelineRepo: pipelineRepo, bindingRepo: bindingRepo}
}

// HandleWebhook 接收 Git Webhook（GitHub / GitLab），无需认证
// POST /api/release/webhook
// Header: X-GitHub-Event: push 或 X-Gitlab-Event: Push
func (h *ReleaseHandler) HandleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "read body: "+err.Error()))
		return
	}

	var repoURL, branch, commitSHA, commitMessage, ref, author string
	event := c.GetHeader("X-GitHub-Event")
	if event == "push" {
		repoURL, branch, commitSHA, commitMessage, ref, author, err = release.ParseGitHubPush(body)
	} else if c.GetHeader("X-Gitlab-Event") == "Push Hook" {
		repoURL, branch, commitSHA, commitMessage, ref, author, err = release.ParseGitLabPush(body)
	} else {
		c.JSON(http.StatusBadRequest, model.Error(400, "unsupported webhook event (expect X-GitHub-Event: push or X-Gitlab-Event: Push Hook)"))
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "parse webhook: "+err.Error()))
		return
	}

	run, err := h.service.CreateFromWebhook(repoURL, branch, commitSHA, commitMessage, ref, author)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create release run: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{
		"id":       run.ID,
		"status":   run.Status,
		"repo_url": run.RepoURL,
		"branch":   run.Branch,
		"commit":   run.CommitSHA,
	}))
}

// HandleWebhookPush 接收带 token 的 Git 推送（动态 URL），根据 token 确定应用并创建发布记录
// POST /api/release/webhook/push?token=xxx
// Header: X-GitHub-Event: push 或 X-Gitlab-Event: Push
func (h *ReleaseHandler) HandleWebhookPush(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, model.Error(401, "token required"))
		return
	}
	binding, err := h.bindingRepo.FindByWebhookToken(token)
	if err != nil || binding == nil {
		c.JSON(http.StatusUnauthorized, model.Error(401, "invalid or disabled token"))
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "read body: "+err.Error()))
		return
	}

	var repoURL, branch, commitSHA, commitMessage, ref, author string
	event := c.GetHeader("X-GitHub-Event")
	if event == "push" {
		repoURL, branch, commitSHA, commitMessage, ref, author, err = release.ParseGitHubPush(body)
	} else if c.GetHeader("X-Gitlab-Event") == "Push Hook" {
		repoURL, branch, commitSHA, commitMessage, ref, author, err = release.ParseGitLabPush(body)
	} else {
		c.JSON(http.StatusBadRequest, model.Error(400, "unsupported webhook event (expect X-GitHub-Event: push or X-Gitlab-Event: Push Hook)"))
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "parse webhook: "+err.Error()))
		return
	}

	run, err := h.service.CreateFromWebhookWithApplication(binding.ApplicationID, repoURL, branch, commitSHA, commitMessage, ref, author)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create release run: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{
		"id":             run.ID,
		"application_id": binding.ApplicationID,
		"status":         run.Status,
		"repo_url":       run.RepoURL,
		"branch":        run.Branch,
		"commit":        run.CommitSHA,
	}))
}

// ListRuns 发布记录列表（需认证）
// GET /api/release/runs
func (h *ReleaseHandler) ListRuns(c *gin.Context) {
	var req struct {
		RepoURL   string `form:"repo_url"`
		Branch    string `form:"branch"`
		Status    string `form:"status"`
		Page      int    `form:"page"`
		PageSize  int    `form:"pageSize"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	list, total, err := h.service.List(req.RepoURL, req.Branch, req.Status, req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list release runs: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	}))
}

// GetRun 单条发布记录详情（需认证）
// GET /api/release/runs/:id
func (h *ReleaseHandler) GetRun(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	run, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "release run not found"))
		return
	}
	c.JSON(http.StatusOK, model.Success(run))
}

// CreateRun 手动创建一条发布记录（需认证）
// POST /api/release/runs
func (h *ReleaseHandler) CreateRun(c *gin.Context) {
	var req struct {
		RepoURL       string `json:"repo_url" binding:"required"`
		Branch        string `json:"branch" binding:"required"`
		CommitSHA     string `json:"commit_sha"`
		CommitMessage string `json:"commit_message"`
		ApplicationID string `json:"application_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)
	if req.CommitSHA == "" {
		req.CommitSHA = "manual"
	}
	if req.CommitMessage == "" {
		req.CommitMessage = "manual trigger"
	}
	run, err := h.service.CreateManual(req.RepoURL, req.Branch, req.CommitSHA, req.CommitMessage, req.ApplicationID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create release run: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(run))
}

// ExecuteRun 执行一条发布记录：按环境触发 Jenkins，或提交 prod 工单
// POST /api/release/runs/:id/execute  body: { "environment": "dev"|"test"|"qa"|"staging"|"prod" }
func (h *ReleaseHandler) ExecuteRun(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	var req struct {
		Environment string `json:"environment"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Environment == "" {
		req.Environment = "dev"
	}
	userID, _ := c.Get("userID")
	userName, _ := c.Get("userName")
	applicantID, _ := userID.(string)
	applicantName, _ := userName.(string)

	prodCreated, approvalID, err := h.service.ExecuteRun(id, req.Environment, applicantID, applicantName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, model.Error(404, "release run not found"))
			return
		}
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	if prodCreated {
		c.JSON(http.StatusOK, model.Success(gin.H{
			"need_approval": true,
			"approval_id":   approvalID,
			"message":       "已提交生产发布审批，请到工单/审批中查看",
		}))
		return
	}
	run, _ := h.service.GetByID(id)
	c.JSON(http.StatusOK, model.Success(run))
}

// GetLastProdRun 查询某应用最近一次 prod 部署成功的 run（用于回滚入口、展示当前线上版本）
// GET /api/release/applications/:id/last-prod-run
func (h *ReleaseHandler) GetLastProdRun(c *gin.Context) {
	applicationID := c.Param("id")
	if applicationID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "application id required"))
		return
	}
	run, err := h.service.GetLastSuccessfulProdRun(applicationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, model.Success(nil)) // 无记录返回 null
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(run))
}

// UpdateRunStatus 更新发布记录状态（Jenkins 完成后或人工标记 success/failed）
// POST /api/release/runs/:id/status  body: { "status": "success"|"failed" }
func (h *ReleaseHandler) UpdateRunStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=success failed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	run, err := h.service.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, model.Error(404, "release run not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	if run.Status != model.ReleaseRunStatusRunning {
		c.JSON(http.StatusBadRequest, model.Error(400, "only running run can be updated to success/failed"))
		return
	}
	now := time.Now()
	if err := h.service.UpdateRunStatus(id, req.Status, &now); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	updated, _ := h.service.GetByID(id)
	c.JSON(http.StatusOK, model.Success(updated))
}

// RollbackProd 提交生产回滚工单（基于上一版 prod 成功 run 创建新 run + 审批单）
// POST /api/release/rollback  body: { "application_id": "uuid" }
func (h *ReleaseHandler) RollbackProd(c *gin.Context) {
	var req struct {
		ApplicationID string `json:"application_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	userID, _ := c.Get("userID")
	userName, _ := c.Get("userName")
	applicantID, _ := userID.(string)
	applicantName, _ := userName.(string)

	created, approvalID, runID, err := h.service.RollbackProd(req.ApplicationID, applicantID, applicantName)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	if !created {
		c.JSON(http.StatusOK, model.Success(gin.H{"need_approval": false, "run_id": runID}))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{
		"need_approval": true,
		"approval_id":   approvalID,
		"run_id":        runID,
		"message":       "已提交生产回滚审批，请到工单/审批中查看",
	}))
}

// ListPipelines 获取流水线定义列表（需认证）
// GET /api/release/pipelines
func (h *ReleaseHandler) ListPipelines(c *gin.Context) {
	list, err := h.pipelineRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list pipelines: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(list))
}

// GetPipeline 获取单条发布流水线定义（需认证）
// GET /api/release/pipeline?id=xxx
func (h *ReleaseHandler) GetPipeline(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	def, err := h.pipelineRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "get pipeline: "+err.Error()))
		return
	}
	if def == nil {
		c.JSON(http.StatusOK, model.Success(gin.H{"id": id, "name": "", "nodes": []interface{}{}, "edges": []interface{}{}}))
		return
	}
	var contentMap gin.H
	if err := json.Unmarshal([]byte(def.Content), &contentMap); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "invalid pipeline content"))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"id": def.ID, "name": def.Name, "nodes": contentMap["nodes"], "edges": contentMap["edges"]}))
}

// SavePipeline 保存发布流水线定义（需认证）
// PUT /api/release/pipeline  body: { "id": "", "name": "", "nodes": [], "edges": [] }  id 为空则新建
func (h *ReleaseHandler) SavePipeline(c *gin.Context) {
	var req struct {
		ID    string        `json:"id"`
		Name  string        `json:"name"`
		Nodes []interface{} `json:"nodes" binding:"required"`
		Edges []interface{} `json:"edges"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	content, err := json.Marshal(gin.H{"nodes": req.Nodes, "edges": req.Edges})
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid nodes/edges"))
		return
	}
	userID, _ := c.Get("userID")
	userIDStr, _ := userID.(string)

	var def *model.ReleasePipelineDefinition
	if req.ID != "" {
		def, _ = h.pipelineRepo.GetByID(req.ID)
	}
	if def == nil {
		newID := uuid.New().String()
		def = &model.ReleasePipelineDefinition{
			ID:        newID,
			Name:      req.Name,
			Content:   string(content),
			UpdatedBy: userIDStr,
		}
	} else {
		def.Content = string(content)
		def.UpdatedBy = userIDStr
		if req.Name != "" {
			def.Name = req.Name
		}
	}
	if err := h.pipelineRepo.Save(def); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "save pipeline: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"id": def.ID, "name": def.Name, "updated_at": def.UpdatedAt}))
}

// DeletePipeline 删除发布流水线定义（需认证）
// DELETE /api/release/pipeline/:id
func (h *ReleaseHandler) DeletePipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	if id == model.ReleasePipelineDefinitionIDDefault {
		c.JSON(http.StatusBadRequest, model.Error(400, "cannot delete default pipeline"))
		return
	}
	if err := h.pipelineRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "delete pipeline: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"id": id}))
}
