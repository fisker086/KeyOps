package workflow

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/fisker086/keyops/internal/model"
)

// WorkflowHandler 精简版工作流/工单处理
type WorkflowHandler struct {
	db *gorm.DB
}

func NewWorkflowHandler(db *gorm.DB) *WorkflowHandler {
	return &WorkflowHandler{db: db}
}

// Generate jobid: wf-YYYYMMDDHHMMSS-xx
func generateJobID() string {
	ts := time.Now().Format("20060102150405")
	const letters = "0123456789abcdefghijklmnopqrstuvwxyz"
	rand.Seed(time.Now().UnixNano())
	suffix := []byte{letters[rand.Intn(len(letters))], letters[rand.Intn(len(letters))]}
	return fmt.Sprintf("wf-%s-%s", ts, suffix)
}

// dispatch GET: list or detail/meta/generate_*
func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	method := c.Query("method")
	switch method {
	case "detail":
		h.getDetail(c)
	case "meta":
		h.getMeta(c)
	case "generate_deploy":
		h.generateDeploy(c)
	case "generate_rollback":
		h.generateRollback(c)
	default:
		h.list(c)
	}
}

// dispatch PUT: actions
func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	var req struct {
		Method       string          `json:"method"`
		JobID        string          `json:"jobid"`
		StepID       string          `json:"step_id"`
		Comment      string          `json:"comment"`
		AssignUserID string          `json:"assign_user_id"`
		TmpArgs      json.RawMessage `json:"tmp_args"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}
	switch req.Method {
	case "commit":
		h.stepAction(c, req.JobID, req.StepID, 1, req.Comment, req.TmpArgs)
	case "reject":
		h.stepAction(c, req.JobID, req.StepID, 4, req.Comment, req.TmpArgs)
	case "rollback":
		h.stepAction(c, req.JobID, req.StepID, 7, req.Comment, req.TmpArgs)
	case "skip":
		h.stepAction(c, req.JobID, req.StepID, 5, req.Comment, req.TmpArgs)
	case "change":
		h.changeStep(c, req.JobID, req.StepID, req.TmpArgs)
	case "reassign":
		h.reassignStep(c, req.JobID, req.StepID, req.AssignUserID)
	default:
		h.updateBase(c, req)
	}
}

// 创建工作流
func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	var payload struct {
		JobID        string              `json:"jobid"`
		Title        string              `json:"title"`
		WorkflowType string              `json:"workflow_type"`
		Status       string              `json:"status"`
		Comment      string              `json:"comment"`
		Labels       datatypes.JSON      `json:"labels"`
		Steps        []model.WorkflowStep `json:"steps"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}
	if payload.Title == "" || payload.WorkflowType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "标题和类型必填"})
		return
	}
	if len(payload.Steps) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "至少包含一个步骤"})
		return
	}

	jobid := payload.JobID
	if jobid == "" {
		jobid = generateJobID()
	}
	status := payload.Status
	if status == "" {
		status = "draft"
	}

	applicantID, _ := c.Get("user_id")
	applicantName, _ := c.Get("username")

	wf := model.Workflow{
		JobID:         jobid,
		Title:         payload.Title,
		WorkflowType:  payload.WorkflowType,
		Status:        status,
		Comment:       payload.Comment,
		Labels:        payload.Labels,
		ApplicantID:   fmt.Sprintf("%v", applicantID),
		ApplicantName: fmt.Sprintf("%v", applicantName),
	}

	for i := range payload.Steps {
		if payload.Steps[i].StepID == "" {
			payload.Steps[i].StepID = fmt.Sprintf("step-%d", i+1)
		}
		payload.Steps[i].JobID = jobid
		payload.Steps[i].StepOrder = i
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&wf).Error; err != nil {
			return err
		}
		if len(payload.Steps) > 0 {
			if err := tx.Create(&payload.Steps).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建工单失败", "error": err.Error()})
		return
	}

	h.db.Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order asc") }).First(&wf, "job_id = ?", jobid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": wf})
}

// list workflows
func (h *WorkflowHandler) list(c *gin.Context) {
	var items []model.Workflow
	query := h.db.Model(&model.Workflow{})

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if wt := c.Query("workflow_type"); wt != "" {
		query = query.Where("workflow_type = ?", wt)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		query = query.Where("title LIKE ? OR job_id LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}

	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取工单列表失败", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  items,
		"total": total,
	})
}

// detail
func (h *WorkflowHandler) getDetail(c *gin.Context) {
	jobid := c.Query("jobid")
	if jobid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 必填"})
		return
	}
	var wf model.Workflow
	if err := h.db.Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order asc") }).
		First(&wf, "job_id = ?", jobid).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "工单不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取工单失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wf})
}

// meta / clone
func (h *WorkflowHandler) getMeta(c *gin.Context) {
	jobid := c.Query("jobid")
	if jobid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 必填"})
		return
	}
	var wf model.Workflow
	if err := h.db.Preload("Steps", func(db *gorm.DB) *gorm.DB { return db.Order("step_order asc") }).
		First(&wf, "job_id = ?", jobid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取工单失败", "error": err.Error()})
		return
	}
	// 构造 meta，重置步骤状态
	meta := map[string]interface{}{
		"title":         wf.Title,
		"workflow_type": wf.WorkflowType,
		"comment":       wf.Comment,
		"labels":        wf.Labels,
		"steps":         []model.WorkflowStep{},
	}
	for _, s := range wf.Steps {
		s.StepStatus = 0
		s.JobID = ""
		meta["steps"] = append(meta["steps"].([]model.WorkflowStep), s)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"job_meta": meta}})
}

func (h *WorkflowHandler) generateDeploy(c *gin.Context) {
	// 精简版直接复用 meta 逻辑
	h.getMeta(c)
}

func (h *WorkflowHandler) generateRollback(c *gin.Context) {
	// 精简版直接复用 meta 逻辑
	h.getMeta(c)
}

func (h *WorkflowHandler) stepAction(c *gin.Context, jobid, stepID string, status int, comment string, tmpArgs json.RawMessage) {
	if jobid == "" || stepID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 和 step_id 必填"})
		return
	}
	var step model.WorkflowStep
	if err := h.db.First(&step, "job_id = ? AND step_id = ?", jobid, stepID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "步骤不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取步骤失败", "error": err.Error()})
		return
	}

	// 变更 func_kwargs_json（如果有）
	if len(tmpArgs) > 0 {
		step.FuncKwargsJSON = datatypes.JSON(tmpArgs)
	}
	step.StepStatus = status

	if err := h.db.Save(&step).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新步骤失败", "error": err.Error()})
		return
	}
	// 记录评论
	if comment != "" {
		userID, _ := c.Get("user_id")
		userName, _ := c.Get("username")
		h.db.Create(&model.WorkflowComment{
			JobID:    jobid,
			StepID:   stepID,
			UserID:   fmt.Sprintf("%v", userID),
			UserName: fmt.Sprintf("%v", userName),
			Action:   map[int]string{1: "commit", 4: "reject", 7: "rollback", 5: "skip"}[status],
			Comment:  comment,
		})
	}
	updateWorkflowStatus(h.db, jobid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *WorkflowHandler) changeStep(c *gin.Context, jobid, stepID string, tmpArgs json.RawMessage) {
	if jobid == "" || stepID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 和 step_id 必填"})
		return
	}
	var step model.WorkflowStep
	if err := h.db.First(&step, "job_id = ? AND step_id = ?", jobid, stepID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取步骤失败", "error": err.Error()})
		return
	}
	if len(tmpArgs) > 0 {
		step.FuncKwargsJSON = datatypes.JSON(tmpArgs)
	}
	if err := h.db.Save(&step).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新步骤失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *WorkflowHandler) reassignStep(c *gin.Context, jobid, stepID, userID string) {
	if jobid == "" || stepID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid/step_id/assign_user_id 必填"})
		return
	}
	var step model.WorkflowStep
	if err := h.db.First(&step, "job_id = ? AND step_id = ?", jobid, stepID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取步骤失败", "error": err.Error()})
		return
	}
	var ids []string
	if len(step.WhoHasPermission) > 0 {
		_ = json.Unmarshal(step.WhoHasPermission, &ids)
	}
	ids = []string{userID}
	buf, _ := json.Marshal(ids)
	step.WhoHasPermission = datatypes.JSON(buf)
	if err := h.db.Save(&step).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新步骤失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// 更新基础信息（非 method 情况）
func (h *WorkflowHandler) updateBase(c *gin.Context, req interface{}) {
	var payload struct {
		JobID string `json:"jobid"`
		Title string `json:"title"`
		Comment string `json:"comment"`
		Status string `json:"status"`
	}
	_ = mapToStruct(req, &payload)
	if payload.JobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 必填"})
		return
	}
	var wf model.Workflow
	if err := h.db.First(&wf, "job_id = ?", payload.JobID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "工单不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取工单失败", "error": err.Error()})
		return
	}
	if payload.Title != "" {
		wf.Title = payload.Title
	}
	if payload.Comment != "" {
		wf.Comment = payload.Comment
	}
	if payload.Status != "" {
		wf.Status = payload.Status
	}
	if err := h.db.Save(&wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": wf})
}

// 草稿：共表，status=draft
func (h *WorkflowHandler) ListDrafts(c *gin.Context) {
	var drafts []model.Workflow
	query := h.db.Model(&model.Workflow{}).Where("status = ?", "draft")
	if userID, ok := c.Get("user_id"); ok {
		query = query.Where("applicant_id = ?", fmt.Sprintf("%v", userID))
	}
	if err := query.Order("updated_at DESC").Find(&drafts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取草稿失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": drafts})
}

func (h *WorkflowHandler) SaveDraft(c *gin.Context) {
	var payload struct {
		JobID        string              `json:"jobid"`
		Title        string              `json:"title"`
		WorkflowType string              `json:"workflow_type"`
		Comment      string              `json:"comment"`
		Labels       datatypes.JSON      `json:"labels"`
		Steps        []model.WorkflowStep `json:"steps"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}
	jobid := payload.JobID
	if jobid == "" {
		jobid = generateJobID()
	}
	applicantID, _ := c.Get("user_id")
	applicantName, _ := c.Get("username")

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var wf model.Workflow
		err := tx.Preload("Steps").First(&wf, "job_id = ?", jobid).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		wf.JobID = jobid
		wf.Title = payload.Title
		wf.WorkflowType = payload.WorkflowType
		wf.Comment = payload.Comment
		wf.Labels = payload.Labels
		wf.Status = "draft"
		wf.ApplicantID = fmt.Sprintf("%v", applicantID)
		wf.ApplicantName = fmt.Sprintf("%v", applicantName)

		if wf.ID == 0 {
			if err := tx.Create(&wf).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(&wf).Error; err != nil {
				return err
			}
			// 先删旧步骤
			if err := tx.Where("job_id = ?", jobid).Delete(&model.WorkflowStep{}).Error; err != nil {
				return err
			}
		}
		for i := range payload.Steps {
			if payload.Steps[i].StepID == "" {
				payload.Steps[i].StepID = fmt.Sprintf("step-%d", i+1)
			}
			payload.Steps[i].JobID = jobid
			payload.Steps[i].StepOrder = i
		}
		if len(payload.Steps) > 0 {
			if err := tx.Create(&payload.Steps).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存草稿失败", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (h *WorkflowHandler) DeleteDraft(c *gin.Context) {
	jobid := c.Query("jobid")
	if jobid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "jobid 必填"})
		return
	}
	var wf model.Workflow
	if err := h.db.First(&wf, "job_id = ?", jobid).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "草稿不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取草稿失败", "error": err.Error()})
		return
	}
	if wf.Status != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "只能删除草稿"})
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", jobid).Delete(&model.WorkflowStep{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&wf).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// 通知占位
func (h *WorkflowHandler) ListStepNotify(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": []interface{}{}})
}

// 辅助函数
func parseIntDefault(val string, def int) int {
	if val == "" {
		return def
	}
	var i int
	_, err := fmt.Sscanf(val, "%d", &i)
	if err != nil {
		return def
	}
	return i
}

func updateWorkflowStatus(db *gorm.DB, jobid string) {
	var steps []model.WorkflowStep
	if err := db.Where("job_id = ?", jobid).Find(&steps).Error; err != nil {
		return
	}
	status := "finished"
	for _, s := range steps {
		if s.StepStatus == 2 || s.StepStatus == 4 {
			status = "rejected"
			break
		}
		if s.StepStatus == 0 || s.StepStatus == 3 || s.StepStatus == 7 {
			status = "running"
		}
	}
	db.Model(&model.Workflow{}).Where("job_id = ?", jobid).Update("status", status)
}

// mapToStruct: 将通用结构映射到目标结构
func mapToStruct(src interface{}, dst interface{}) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

