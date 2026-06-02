package buildmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/approval"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	releaseSvc "github.com/fisker086/keyops/internal/service/release"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BuildMasterHandler struct {
	repo            repository.BuildMasterRepository
	db              *gorm.DB
	approvalFactory *approval.Factory
	releaseSvc      *releaseSvc.Service
}

func NewBuildMasterHandler(repo repository.BuildMasterRepository, db *gorm.DB, af *approval.Factory) *BuildMasterHandler {
	return &BuildMasterHandler{repo: repo, db: db, approvalFactory: af}
}

func (h *BuildMasterHandler) SetReleaseService(svc *releaseSvc.Service) {
	h.releaseSvc = svc
}

// List 按发版日期、站点、类型查询列表
// GET /api/build-master/lists?publish_date=2025-02-12&site=香港&type=0
func (h *BuildMasterHandler) List(c *gin.Context) {
	publishDate := c.Query("publish_date")
	if publishDate == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "publish_date required"))
		return
	}
	site := c.Query("site")
	var typeFilter *int
	if t := c.Query("type"); t != "" {
		v, err := strconv.Atoi(t)
		if err != nil || (v != 0 && v != 1) {
			c.JSON(http.StatusBadRequest, model.Error(400, "type must be 0 or 1"))
			return
		}
		typeFilter = &v
	}
	lists, err := h.repo.ListByDateAndType(publishDate, site, typeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(lists))
}

// Get 获取单条
// GET /api/build-master/lists/:id
func (h *BuildMasterHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	list, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "not found"))
		return
	}
	c.JSON(http.StatusOK, model.Success(list))
}

// Create 新建一发版任务（常规/紧急）
// POST /api/build-master/lists  body: { "publish_date": "2025-02-12", "site": "香港", "type": 0 }
func (h *BuildMasterHandler) Create(c *gin.Context) {
	var body struct {
		PublishDate string `json:"publish_date" binding:"required"`
		Site        string `json:"site"`
		Type        int    `json:"type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	if body.Type != model.BuildMasterTypeNormal && body.Type != model.BuildMasterTypeUrgent {
		c.JSON(http.StatusBadRequest, model.Error(400, "type must be 0 or 1"))
		return
	}
	ownerID, _ := c.Get("user_id")
	ownerName, _ := c.Get("username")
	ownerIDStr := toString(ownerID)
	ownerNameStr := toString(ownerName)

	orderNum, err := h.repo.MaxOrderForDateAndType(body.PublishDate, body.Site, body.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "max order: "+err.Error()))
		return
	}

	list := &model.BuildMasterList{
		ID:          uuid.New().String(),
		PublishDate: body.PublishDate,
		Site:        body.Site,
		Type:        body.Type,
		Status:      model.BuildMasterStatusCreated,
		OrderNum:    orderNum,
		OwnerID:     ownerIDStr,
		OwnerName:   ownerNameStr,
	}
	if err := h.repo.Create(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create: "+err.Error()))
		return
	}
	bodyBytes, _ := json.Marshal([]map[string]string{
		{"name": "status", "old": "", "new": strconv.Itoa(list.Status)},
		{"name": "type", "old": "", "new": strconv.Itoa(list.Type)},
		{"name": "publish_date", "old": "", "new": list.PublishDate},
		{"name": "site", "old": "", "new": list.Site},
		{"name": "order", "old": "", "new": strconv.Itoa(list.OrderNum)},
	})
	_ = h.repo.CreateOperationLog(&model.BuildMasterOperationLog{
		ListID:       list.ID,
		OperatorID:   ownerIDStr,
		OperatorName: ownerNameStr,
		Method:       "create",
		Body:         string(bodyBytes),
	})
	c.JSON(http.StatusOK, model.Success(list))
}

// Update 更新状态、自定义弹名、催一下
// PATCH /api/build-master/lists/:id
func (h *BuildMasterHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "id required"))
		return
	}
	list, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "not found"))
		return
	}
	oldStatus := list.Status
	oldOrderDescribe := list.OrderDescribe
	oldHurried := list.Hurried

	var body struct {
		Status        *int    `json:"status"`
		OrderDescribe *string `json:"order_describe"`
		Hurried       *int    `json:"hurried"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body"))
		return
	}
	if body.Status != nil {
		if *body.Status < 0 || *body.Status > 4 {
			c.JSON(http.StatusBadRequest, model.Error(400, "status must be 0-4"))
			return
		}
		list.Status = *body.Status
	}
	if body.OrderDescribe != nil {
		list.OrderDescribe = *body.OrderDescribe
	}
	if body.Hurried != nil {
		if *body.Hurried < 0 || *body.Hurried > 3 {
			c.JSON(http.StatusBadRequest, model.Error(400, "hurried must be 0-3"))
			return
		}
		list.Hurried = *body.Hurried
	}
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update: "+err.Error()))
		return
	}
	var changes []map[string]string
	if body.Status != nil && *body.Status != oldStatus {
		changes = append(changes, map[string]string{"name": "status", "old": strconv.Itoa(oldStatus), "new": strconv.Itoa(list.Status)})
	}
	if body.OrderDescribe != nil && (oldOrderDescribe != list.OrderDescribe) {
		changes = append(changes, map[string]string{"name": "order_describe", "old": oldOrderDescribe, "new": list.OrderDescribe})
	}
	if body.Hurried != nil && *body.Hurried != oldHurried {
		changes = append(changes, map[string]string{"name": "hurried", "old": strconv.Itoa(oldHurried), "new": strconv.Itoa(list.Hurried)})
	}
	if len(changes) > 0 {
		bodyBytes, _ := json.Marshal(changes)
		operatorID, _ := c.Get("user_id")
		operatorName, _ := c.Get("username")
		_ = h.repo.CreateOperationLog(&model.BuildMasterOperationLog{
			ListID:       id,
			OperatorID:   toString(operatorID),
			OperatorName: toString(operatorName),
			Method:       "update",
			Body:         string(bodyBytes),
		})
	}
	c.JSON(http.StatusOK, model.Success(list))
}

// RecordsByQuery 获取某发版任务的操作记录
// GET /api/build-master/records?list_id=xxx
func (h *BuildMasterHandler) RecordsByQuery(c *gin.Context) {
	listID := c.Query("list_id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list_id required"))
		return
	}
	logs, err := h.repo.ListOperationLogsByListID(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "records: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(logs))
}

// SiteStats 按站点统计发版总次数与最近发布时间
// GET /api/build-master/stats/sites
func (h *BuildMasterHandler) SiteStats(c *gin.Context) {
	counts, err := h.repo.CountBySite()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "count by site: "+err.Error()))
		return
	}
	latest, err := h.repo.LatestPublishAtBySite()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "latest by site: "+err.Error()))
		return
	}

	type siteStat struct {
		Site            string `json:"site"`
		PublishCount    int64  `json:"publish_count"`
		RecentPublishAt string `json:"recent_publish_at,omitempty"`
	}
	resp := make([]siteStat, 0, len(counts))
	for site, cnt := range counts {
		item := siteStat{Site: site, PublishCount: cnt}
		if raw := latest[site]; raw != "" {
			if t, parseErr := time.Parse(time.RFC3339Nano, raw); parseErr == nil {
				item.RecentPublishAt = t.Format("2006-01-02 15:04")
			} else {
				item.RecentPublishAt = raw
			}
		}
		resp = append(resp, item)
	}
	c.JSON(http.StatusOK, model.Success(resp))
}

// ──────────────────────────────────────────────
// Item（工作项分类）CRUD
// ──────────────────────────────────────────────

// ListItems 获取某发版任务下的所有工作项及条目详情
// GET /api/build-master/lists/:id/items
func (h *BuildMasterHandler) ListItems(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	items, err := h.repo.ListItemsByListID(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list items: "+err.Error()))
		return
	}
	details, err := h.repo.ListDetailsByListID(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list details: "+err.Error()))
		return
	}

	type ItemWithDetails struct {
		model.BuildMasterItem
		Details []model.BuildMasterItemDetail `json:"details"`
	}

	result := make([]ItemWithDetails, 0, len(items))
	for _, item := range items {
		itemDetails := make([]model.BuildMasterItemDetail, 0)
		for _, d := range details {
			if d.ItemID == item.ID {
				itemDetails = append(itemDetails, d)
			}
		}
		result = append(result, ItemWithDetails{
			BuildMasterItem: item,
			Details:         itemDetails,
		})
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// CreateItem 创建工作项分类
// POST /api/build-master/items  body: {"list_id": "...", "name": "..."}
func (h *BuildMasterHandler) CreateItem(c *gin.Context) {
	var body struct {
		ListID string `json:"list_id" binding:"required"`
		Name   string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	item := &model.BuildMasterItem{
		ListID:   body.ListID,
		Name:     body.Name,
		OrderNum: 0,
	}
	if err := h.repo.CreateItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create item: "+err.Error()))
		return
	}
	logOperation(c, h.repo, body.ListID, "create_item", []map[string]string{
		{"name": "item_name", "old": "", "new": item.Name},
	})
	c.JSON(http.StatusOK, model.Success(item))
}

// ListCheckItems 获取工作项模板
// GET /api/build-master/check-items
func (h *BuildMasterHandler) ListCheckItems(c *gin.Context) {
	items, err := h.repo.ListCheckItems()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "list check items: "+err.Error()))
		return
	}
	// BMS 只保留指定模板项
	site := strings.TrimSpace(c.Query("site"))
	if strings.EqualFold(site, "BMS") {
		filtered := make([]model.BuildMasterCheckItem, 0, len(items))
		for _, it := range items {
			name := strings.ToLower(strings.TrimSpace(it.Name))
			if strings.Contains(name, "前端") ||
				strings.Contains(name, "后端") ||
				strings.Contains(name, "nacos") ||
				strings.Contains(name, "中间件") ||
				strings.Contains(name, "mysql") ||
				strings.Contains(name, "数据库") {
				filtered = append(filtered, it)
			}
		}
		// 若规则过严导致空列表，回退原始模板，避免前端无数据可选
		if len(filtered) > 0 {
			items = filtered
		}
	}
	c.JSON(http.StatusOK, model.Success(items))
}

// DeleteItem 删除工作项分类（同时删除其下所有详情）
// DELETE /api/build-master/items/:id
func (h *BuildMasterHandler) DeleteItem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid item id"))
		return
	}
	// 先查出 item 获取 list_id
	items, _ := h.repo.ListItemsByListID("")
	if err := h.repo.DeleteItem(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "delete item: "+err.Error()))
		return
	}
	// 删除对应的 details
	_ = h.repo.UpdateDetailsStatusByItemID(uint(id), 4) // 标记为已删除
	_ = h.repo.DeleteDetail(uint(id))
	// 记录操作日志
	for _, it := range items {
		if it.ID == uint(id) {
			logOperation(c, h.repo, it.ListID, "delete_item", []map[string]string{
				{"name": "item_name", "old": it.Name, "new": ""},
			})
			break
		}
	}
	c.JSON(http.StatusOK, model.Success(nil))
}

// ──────────────────────────────────────────────
// Detail（条目详情）CRUD
// ──────────────────────────────────────────────

// CreateDetail 创建条目详情
// POST /api/build-master/details
func (h *BuildMasterHandler) CreateDetail(c *gin.Context) {
	var body struct {
		ListID  string `json:"list_id" binding:"required"`
		ItemID  uint   `json:"item_id" binding:"required"`
		AppName string `json:"app_name"`
		Operate string `json:"operate"`
		SubType string `json:"sub_type"`
		Tag     string `json:"tag"`
		Content string `json:"content"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	detail := &model.BuildMasterItemDetail{
		ListID:   body.ListID,
		ItemID:   body.ItemID,
		AppName:  body.AppName,
		Operate:  body.Operate,
		SubType:  body.SubType,
		Tag:      body.Tag,
		Content:  body.Content,
		Note:     body.Note,
		Status:   model.BuildMasterItemStatusUndone,
		OrderNum: 0,
	}
	if err := h.repo.CreateDetail(detail); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "create detail: "+err.Error()))
		return
	}
	if list, err := h.repo.GetByID(body.ListID); err == nil && list.Status == model.BuildMasterStatusCreated {
		list.Status = model.BuildMasterStatusFilling
		_ = h.repo.Update(list)
	}
	logOperation(c, h.repo, body.ListID, "create_detail", []map[string]string{
		{"name": "app_name", "old": "", "new": detail.AppName},
		{"name": "operate", "old": "", "new": detail.Operate},
	})
	c.JSON(http.StatusOK, model.Success(detail))
}

// UpdateDetail 更新条目详情
// PATCH /api/build-master/details/:id
func (h *BuildMasterHandler) UpdateDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid detail id"))
		return
	}
	detail, err := h.repo.GetDetailByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "detail not found"))
		return
	}

	var body struct {
		AppName     *string `json:"app_name"`
		Operate     *string `json:"operate"`
		SubType     *string `json:"sub_type"`
		Tag         *string `json:"tag"`
		Content     *string `json:"content"`
		Note        *string `json:"note"`
		RollbackStr *string `json:"rollback"`
		Record      *string `json:"record"`
		Status      *int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	if body.AppName != nil {
		detail.AppName = *body.AppName
	}
	if body.Operate != nil {
		detail.Operate = *body.Operate
	}
	if body.SubType != nil {
		detail.SubType = *body.SubType
	}
	if body.Tag != nil {
		detail.Tag = *body.Tag
	}
	if body.Content != nil {
		detail.Content = *body.Content
	}
	if body.Note != nil {
		detail.Note = *body.Note
	}
	if body.RollbackStr != nil {
		detail.Rollback = *body.RollbackStr
	}
	if body.Record != nil {
		detail.Record = *body.Record
	}
	if body.Status != nil {
		detail.Status = *body.Status
	}

	if err := h.repo.UpdateDetail(detail); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update detail: "+err.Error()))
		return
	}
	logOperation(c, h.repo, detail.ListID, "update_detail", []map[string]string{
		{"name": "app_name", "old": "", "new": detail.AppName},
	})
	c.JSON(http.StatusOK, model.Success(detail))
}

// DeleteDetail 删除条目详情
// DELETE /api/build-master/details/:id
func (h *BuildMasterHandler) DeleteDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid detail id"))
		return
	}
	detail, err := h.repo.GetDetailByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "detail not found"))
		return
	}
	if err := h.repo.DeleteDetail(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "delete detail: "+err.Error()))
		return
	}
	logOperation(c, h.repo, detail.ListID, "delete_detail", []map[string]string{
		{"name": "detail", "old": detail.AppName + "/" + detail.Operate, "new": ""},
	})
	c.JSON(http.StatusOK, model.Success(nil))
}

// ──────────────────────────────────────────────
// Submit / Approve / Execute / Complete / Rollback
// ──────────────────────────────────────────────

// SubmitForApproval 提交审批
// POST /api/build-master/lists/:id/submit
func (h *BuildMasterHandler) SubmitForApproval(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}

	// 更新状态为审批中
	list.Status = model.BuildMasterStatusApproving
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update status: "+err.Error()))
		return
	}

	// 创建审批记录（当前用户为审批人）
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	approval := &model.BuildMasterApproval{
		ListID:       listID,
		ApproverID:   toString(userID),
		ApproverName: toString(userName),
		Result:       model.BuildMasterApprovalPending,
		OrderNum:     1,
	}
	// 如果有第二个审批人
	var body struct {
		ApproverID   string `json:"approver_id"`
		ApproverName string `json:"approver_name"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.ApproverID != "" {
		// Two approvals needed: owner and additional approver
		_ = h.repo.CreateApproval(approval)
		approval2 := &model.BuildMasterApproval{
			ListID:       listID,
			ApproverID:   body.ApproverID,
			ApproverName: body.ApproverName,
			Result:       model.BuildMasterApprovalPending,
			OrderNum:     2,
		}
		_ = h.repo.CreateApproval(approval2)
	} else {
		_ = h.repo.CreateApproval(approval)
	}

	// 同步创建审批总表记录，确保 /approvals 页面可见
	now := time.Now()
	approvalRecord := &model.Approval{
		ID:            uuid.New().String(),
		Title:         "发版审批: " + list.PublishDate + " " + list.Site,
		Description:   "build_master_list_id:" + listID,
		Type:          model.ApprovalTypeDeployment,
		Status:        model.ApprovalStatusPending,
		Platform:      model.ApprovalPlatformInternal,
		ApplicantID:   toString(userID),
		ApplicantName: toString(userName),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := h.db.Create(approvalRecord).Error; err != nil {
		logger.Errorf("同步创建审批记录失败: %v", err)
	}

	logOperation(c, h.repo, listID, "submit", []map[string]string{
		{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusFilling), "new": strconv.Itoa(model.BuildMasterStatusApproving)},
	})
	c.JSON(http.StatusOK, model.Success(list))
}

// Approve 审批（通过/拒绝）
// POST /api/build-master/lists/:id/approve  body: {"result": "approved"/"rejected", "comment": "..."}
func (h *BuildMasterHandler) Approve(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}
	if list.Status != model.BuildMasterStatusApproving {
		c.JSON(http.StatusBadRequest, model.Error(400, "list is not in approving status"))
		return
	}

	var body struct {
		Result  string `json:"result" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	if body.Result != model.BuildMasterApprovalApproved && body.Result != model.BuildMasterApprovalRejected {
		c.JSON(http.StatusBadRequest, model.Error(400, "result must be approved or rejected"))
		return
	}

	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")

	// 查找当前用户的待审批记录
	approvals, err := h.repo.GetPendingApprovalsByListID(listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "get approvals: "+err.Error()))
		return
	}

	found := false
	for i := range approvals {
		if approvals[i].ApproverID == toString(userID) {
			approvals[i].Result = body.Result
			approvals[i].Comment = body.Comment
			if err := h.repo.UpdateApproval(&approvals[i]); err != nil {
				c.JSON(http.StatusInternalServerError, model.Error(500, "update approval: "+err.Error()))
				return
			}
			found = true
			break
		}
	}

	if !found {
		// If no existing approval, create one
		orderNum := len(approvals) + 1
		approval := &model.BuildMasterApproval{
			ListID:       listID,
			ApproverID:   toString(userID),
			ApproverName: toString(userName),
			Result:       body.Result,
			Comment:      body.Comment,
			OrderNum:     orderNum,
		}
		if err := h.repo.CreateApproval(approval); err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "create approval: "+err.Error()))
			return
		}
	}

	// 检查是否所有审批都通过了
	allApprovals, _ := h.repo.ListApprovalsByListID(listID)
	allApproved := true
	hasRejected := false
	for _, a := range allApprovals {
		if a.Result == model.BuildMasterApprovalRejected {
			hasRejected = true
			allApproved = false
			break
		}
		if a.Result != model.BuildMasterApprovalApproved {
			allApproved = false
		}
	}

	if allApproved && len(allApprovals) > 0 {
		list.Status = model.BuildMasterStatusReleasing
		list.DeployStatus = model.BuildMasterDeployPending
		// 第一版闭环：审批通过后将明细标记为"待发布执行"，便于发布页直接接续操作
		if details, derr := h.repo.ListDetailsByListID(listID); derr == nil {
			for i := range details {
				if details[i].Record == "" {
					details[i].Record = "审批通过，待发布执行"
					_ = h.repo.UpdateDetail(&details[i])
				}
			}
		}
		// 同步更新审批总表状态为已通过
		h.db.Model(&model.Approval{}).
			Where("description = ? AND status = ?", "build_master_list_id:"+listID, model.ApprovalStatusPending).
			Update("status", model.ApprovalStatusApproved)
	} else if hasRejected {
		list.Status = model.BuildMasterStatusFilling
		// 同步更新审批总表状态为已拒绝
		h.db.Model(&model.Approval{}).
			Where("description = ? AND status = ?", "build_master_list_id:"+listID, model.ApprovalStatusPending).
			Update("status", model.ApprovalStatusRejected)
	}
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update list: "+err.Error()))
		return
	}

	if allApproved && len(allApprovals) > 0 {
		logOperation(c, h.repo, listID, "approve", []map[string]string{
			{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusApproving), "new": strconv.Itoa(model.BuildMasterStatusReleasing)},
			{"name": "approve_result", "old": "", "new": body.Result},
		})
		logOperation(c, h.repo, listID, "auto_release_ready", []map[string]string{
			{"name": "release_status", "old": "approving", "new": "ready_to_release"},
		})
	} else if hasRejected {
		logOperation(c, h.repo, listID, "approve", []map[string]string{
			{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusApproving), "new": strconv.Itoa(model.BuildMasterStatusFilling)},
			{"name": "approve_result", "old": "", "new": body.Result},
		})
	} else {
		logOperation(c, h.repo, listID, "approve", []map[string]string{
			{"name": "approve_result", "old": "", "new": body.Result},
		})
	}
	c.JSON(http.StatusOK, model.Success(list))
}

// ExecuteDetail 执行条目（标记完成/取消/回滚）
// POST /api/build-master/details/:id/execute  body: {"status": 1, "record": "..."}
func (h *BuildMasterHandler) ExecuteDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid detail id"))
		return
	}
	detail, err := h.repo.GetDetailByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "detail not found"))
		return
	}

	var body struct {
		Status *int   `json:"status"`
		Record string `json:"record"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid body: "+err.Error()))
		return
	}
	if body.Status != nil {
		if *body.Status < 0 || *body.Status > 3 {
			c.JSON(http.StatusBadRequest, model.Error(400, "status must be 0-3"))
			return
		}
		detail.Status = *body.Status
	}
	if body.Record != "" {
		detail.Record = body.Record
	}

	if err := h.repo.UpdateDetail(detail); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "execute detail: "+err.Error()))
		return
	}
	logOperation(c, h.repo, detail.ListID, "execute_detail", []map[string]string{
		{"name": "status", "old": "", "new": strconv.Itoa(detail.Status)},
	})

	allDetails, err := h.repo.ListDetailsByListID(detail.ListID)
	if err == nil {
		allDone := true
		for _, d := range allDetails {
			if d.Status == model.BuildMasterItemStatusUndone {
				allDone = false
				break
			}
		}
		if allDone {
			list, err := h.repo.GetByID(detail.ListID)
			if err == nil && list.Status == model.BuildMasterStatusReleasing {
				logOperation(c, h.repo, detail.ListID, "deploy", []map[string]string{
					{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusReleasing), "new": strconv.Itoa(model.BuildMasterStatusCompleted)},
				})
				list.Status = model.BuildMasterStatusCompleted
				if err := h.repo.Update(list); err == nil {
					logOperation(c, h.repo, detail.ListID, "complete", []map[string]string{
						{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusReleasing), "new": strconv.Itoa(model.BuildMasterStatusCompleted)},
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, model.Success(detail))
}

// CompleteList 完成发版
// POST /api/build-master/lists/:id/complete
func (h *BuildMasterHandler) CompleteList(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}
	list.Status = model.BuildMasterStatusCompleted
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "complete: "+err.Error()))
		return
	}
	logOperation(c, h.repo, listID, "complete", []map[string]string{
		{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusReleasing), "new": strconv.Itoa(model.BuildMasterStatusCompleted)},
	})
	c.JSON(http.StatusOK, model.Success(list))
}

// RollbackList 回滚发版
// POST /api/build-master/lists/:id/rollback
func (h *BuildMasterHandler) RollbackList(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}
	if list.Status != model.BuildMasterStatusCompleted {
		c.JSON(http.StatusBadRequest, model.Error(400, "only completed list can be rolled back"))
		return
	}

	// 创建回滚 ReleaseRun
	if h.releaseSvc != nil {
		rollbackRun := &model.ReleaseRun{
			ID:                  uuid.New().String(),
			Source:              model.ReleaseRunSourceRollback,
			Status:              model.ReleaseRunStatusRunning,
			DeployedEnvironment: "prod",
			RollbackFromRunID:   list.ReleaseRunID,
		}
		if err := h.releaseSvc.CreateRun(rollbackRun); err != nil {
			c.JSON(http.StatusInternalServerError, model.Error(500, "创建回滚记录失败: "+err.Error()))
			return
		}
		list.ReleaseRunID = rollbackRun.ID
	}

	// 回滚：状态回到发版中，标记所有详情为已回滚
	list.Status = model.BuildMasterStatusReleasing
	list.DeployStatus = model.BuildMasterDeployRollback
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "rollback: "+err.Error()))
		return
	}
	details, _ := h.repo.ListDetailsByListID(listID)
	for _, d := range details {
		if d.Status == model.BuildMasterItemStatusDone {
			d.Status = model.BuildMasterItemStatusRollback
			_ = h.repo.UpdateDetail(&d)
		}
	}
	logOperation(c, h.repo, listID, "rollback", []map[string]string{
		{"name": "status", "old": strconv.Itoa(model.BuildMasterStatusCompleted), "new": strconv.Itoa(model.BuildMasterStatusReleasing)},
		{"name": "deploy_status", "old": "", "new": model.BuildMasterDeployRollback},
	})
	c.JSON(http.StatusOK, model.Success(list))
}

// DeployList 触发发版单部署：创建 ReleaseRun 并执行
// POST /api/build-master/lists/:id/deploy
func (h *BuildMasterHandler) DeployList(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}
	if list.Status != model.BuildMasterStatusReleasing {
		c.JSON(http.StatusBadRequest, model.Error(400, "只有发版中状态才能触发部署"))
		return
	}
	if list.DeployStatus == model.BuildMasterDeployDeploying {
		c.JSON(http.StatusBadRequest, model.Error(400, "部署正在进行中"))
		return
	}
	if h.releaseSvc == nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "发布服务未配置"))
		return
	}

	// 创建 ReleaseRun
	now := time.Now()
	run := &model.ReleaseRun{
		ID:                  uuid.New().String(),
		Source:              model.ReleaseRunSourceBuildMaster,
		Status:              model.ReleaseRunStatusRunning,
		DeployedEnvironment: "prod",
		StartedAt:           &now,
		CreatedBy:           toString(c.MustGet("user_id")),
	}
	if err := h.releaseSvc.CreateRun(run); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "创建部署记录失败: "+err.Error()))
		return
	}

	// 更新发版单
	list.ReleaseRunID = run.ID
	list.DeployStatus = model.BuildMasterDeployDeploying
	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update list: "+err.Error()))
		return
	}

	logOperation(c, h.repo, listID, "deploy", []map[string]string{
		{"name": "deploy_status", "old": "", "new": model.BuildMasterDeployDeploying},
		{"name": "release_run_id", "old": "", "new": run.ID},
	})

	c.JSON(http.StatusOK, model.Success(gin.H{
		"release_run_id": run.ID,
		"deploy_status":  list.DeployStatus,
	}))
}

// UpdateDeployStatus 更新发版单部署状态
// PATCH /api/build-master/lists/:id/deploy-status
func (h *BuildMasterHandler) UpdateDeployStatus(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "list id required"))
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "status required (success/failed)"))
		return
	}
	if body.Status != model.BuildMasterDeploySuccess && body.Status != model.BuildMasterDeployFailed {
		c.JSON(http.StatusBadRequest, model.Error(400, "status must be success or failed"))
		return
	}

	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "list not found"))
		return
	}
	if list.DeployStatus != model.BuildMasterDeployDeploying {
		c.JSON(http.StatusBadRequest, model.Error(400, "当前状态不是部署中"))
		return
	}

	oldStatus := list.DeployStatus
	list.DeployStatus = body.Status

	// 部署成功时自动完成发版单
	if body.Status == model.BuildMasterDeploySuccess {
		list.Status = model.BuildMasterStatusCompleted
	}

	if h.releaseSvc != nil && list.ReleaseRunID != "" {
		completedAt := time.Now()
		runStatus := model.ReleaseRunStatusSuccess
		if body.Status == model.BuildMasterDeployFailed {
			runStatus = model.ReleaseRunStatusFailed
		}
		_ = h.releaseSvc.UpdateRunStatus(list.ReleaseRunID, runStatus, &completedAt)
	}

	if err := h.repo.Update(list); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "update list: "+err.Error()))
		return
	}

	logOperation(c, h.repo, listID, "update_deploy_status", []map[string]string{
		{"name": "deploy_status", "old": oldStatus, "new": body.Status},
	})

	c.JSON(http.StatusOK, model.Success(list))
}

// ──────────────────────────────────────────────
// 第三方审批集成（Feishu / DingTalk / WeChat）
// ──────────────────────────────────────────────

// approvalFormData 构建审批表单数据（通用格式，各 provider 自行转换）
func (h *BuildMasterHandler) approvalFormData(list *model.BuildMasterList) []map[string]interface{} {
	typeVal := "常规发版"
	if list.Type == model.BuildMasterTypeUrgent {
		typeVal = "紧急发版"
	}
	orderStr := list.OrderDescribe
	if orderStr == "" {
		orderStr = "第" + strconv.Itoa(list.OrderNum) + "弹"
	}
	return []map[string]interface{}{
		{"id": "发版日期", "type": "input", "value": list.PublishDate},
		{"id": "站点", "type": "input", "value": list.Site},
		{"id": "发版类型", "type": "input", "value": typeVal},
		{"id": "弹数", "type": "input", "value": orderStr},
		{"id": "负责人", "type": "input", "value": list.OwnerName},
	}
}

// widgetFormData 根据审批配置中保存的三方 widget 结构构建表单数据
// settings 中下载的三方 widget scheme 存储在 ApprovalConfig.FormFields 中，
// 每个 widget 字段可配置 keyword 指定数据来源：
//
//	auto_id         – 自动生成工单编号（WF{timestamp}-{listId}）
//	auto_url        – 自动生成工单链接
//	fixed:<值>      – 固定值
//	option:<值>     – 从 option 匹配的值（无需 keyword，自动匹配）
//	list.<field>    – BuildMasterList 字段，如 list.site / list.type_label
//	detail.<field>  – TABLE 类型子字段，如 detail.app_name / detail.tag / detail.operate
//
// 示例：在 settings 页面的 form_fields JSON 中为字段添加 keyword：
//
//	{"id":"xxx","type":"input","name":"工单编号","keyword":"auto_id"}
//	{"id":"xxx","type":"select","name":"站点","keyword":"list.site"}
//	{"id":"xxx","type":"textarea","name":"明细","keyword":"details","children":[
//	  {"id":"c1","type":"input","name":"项目类","keyword":"detail.app_name"},
//	  {"id":"c2","type":"input","name":"版本号","keyword":"detail.tag"}
//	]}
func (h *BuildMasterHandler) widgetFormData(list *model.BuildMasterList, widgetFields []map[string]interface{}, baseURL string) []map[string]interface{} {
	now := time.Now()
	typeVal := "常规发版"
	if list.Type == model.BuildMasterTypeUrgent {
		typeVal = "紧急发版"
	}
	orderStr := list.OrderDescribe
	if orderStr == "" {
		orderStr = "第" + strconv.Itoa(list.OrderNum) + "弹"
	}

	// 加载明细数据
	details, _ := h.repo.ListDetailsByListID(list.ID)

	var formData []map[string]interface{}
	for _, field := range widgetFields {
		value := resolveFieldValue(field, list, typeVal, orderStr, now, details, baseURL)
		if !shouldSubmitWidgetValue(field, value) {
			continue
		}
		formData = append(formData, map[string]interface{}{
			"id":    field["id"],
			"type":  field["type"],
			"value": value,
		})
	}
	return formData
}

// shouldSubmitWidgetValue 与 hashcheck 一致：未解析出的空字段不提交，避免飞书校验失败
func shouldSubmitWidgetValue(field map[string]interface{}, value interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []interface{}:
		return true
	default:
		return true
	}
}

// isTypeWidgetField 发版类型单选
func isTypeWidgetField(field map[string]interface{}) bool {
	keyword, _ := field["keyword"].(string)
	if keyword == "list.type_label" {
		return true
	}
	name, _ := field["name"].(string)
	return strings.Contains(strings.TrimSpace(name), "发版类型")
}

// isVatpWidgetField 日月报
func isVatpWidgetField(field map[string]interface{}) bool {
	name, _ := field["name"].(string)
	return strings.Contains(strings.TrimSpace(name), "日月报")
}

// isSqlWidgetField 含有SQL
func isSqlWidgetField(field map[string]interface{}) bool {
	name, _ := field["name"].(string)
	return strings.Contains(strings.TrimSpace(name), "含有SQL") || strings.Contains(strings.TrimSpace(name), "SQL")
}

// isDetailsWidgetField 明细表格（含飞书 fieldList）
func isDetailsWidgetField(field map[string]interface{}) bool {
	keyword, _ := field["keyword"].(string)
	if keyword == "details" || strings.HasPrefix(keyword, "details") {
		return true
	}
	name, _ := field["name"].(string)
	if strings.HasPrefix(strings.TrimSpace(name), "明细") {
		return true
	}
	wtype, _ := field["type"].(string)
	return strings.EqualFold(wtype, "fieldlist")
}

// isOrderWidgetField 弹次/弹数
func isOrderWidgetField(field map[string]interface{}) bool {
	keyword, _ := field["keyword"].(string)
	if keyword == "list.order" {
		return true
	}
	name, _ := field["name"].(string)
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, "弹数") || strings.HasPrefix(name, "弹次")
}

// isOwnerWidgetField 负责人
func isOwnerWidgetField(field map[string]interface{}) bool {
	keyword, _ := field["keyword"].(string)
	if keyword == "list.owner" {
		return true
	}
	name, _ := field["name"].(string)
	return strings.Contains(strings.TrimSpace(name), "负责人")
}

// inferFieldValueByName 按飞书控件名称推断（hashcheck feishuApp.create_instance 兼容）
func inferFieldValueByName(
	field map[string]interface{},
	list *model.BuildMasterList,
	typeVal, orderStr string,
	now time.Time,
	details []model.BuildMasterItemDetail,
	baseURL string,
) (interface{}, bool) {
	name, _ := field["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}

	switch {
	case strings.HasPrefix(name, "工单编号"):
		return fmt.Sprintf("WF%d-%s", now.Unix(), list.ID[:8]), true
	case strings.HasPrefix(name, "工单站点"), strings.HasPrefix(name, "发版站点"):
		return matchOptionValue(field, list.Site), true
	case strings.HasPrefix(name, "发版类型"):
		return matchOptionValueForType(field, typeVal), true
	case strings.HasPrefix(name, "日月报"):
		return matchYesNoOptionValue(field, "否"), true
	case strings.HasPrefix(name, "含有SQL"):
		return matchYesNoOptionValue(field, "否"), true
	case strings.HasPrefix(name, "明细"):
		return buildDetailsTableValue(field, details), true
	case strings.HasPrefix(name, "工单链接"):
		return fmt.Sprintf("%s/build-master/list/%s", baseURL, list.ID), true
	case strings.HasPrefix(name, "发版日期"):
		return list.PublishDate, true
	case strings.HasPrefix(name, "弹数"), strings.HasPrefix(name, "弹次"):
		return orderStr, true
	case strings.HasPrefix(name, "负责人"):
		return list.OwnerName, true
	}
	return nil, false
}

func buildDetailsTableValue(field map[string]interface{}, details []model.BuildMasterItemDetail) []interface{} {
	childrenRaw, _ := field["children"].([]interface{})
	var tableData []interface{}
	for _, detail := range details {
		tableData = append(tableData, buildDetailRowByKeyword(childrenRaw, detail))
	}
	if tableData == nil {
		tableData = []interface{}{}
	}
	return tableData
}

// isSiteWidgetField 判断是否为发版站点类控件（飞书 radio/select）
func isSiteWidgetField(field map[string]interface{}) bool {
	keyword, _ := field["keyword"].(string)
	if keyword == "list.site" {
		return true
	}
	name, _ := field["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	nameLower := strings.ToLower(name)
	return strings.Contains(name, "站点") ||
		strings.Contains(nameLower, "site") ||
		strings.HasPrefix(name, "工单站点") ||
		strings.HasPrefix(name, "发版站点")
}

// isOptionWidgetType 是否为需要 option value 的单选/下拉控件
func isOptionWidgetType(field map[string]interface{}) bool {
	wtype, _ := field["type"].(string)
	switch strings.ToLower(wtype) {
	case "radio", "radiov2", "select", "checkbox", "checkboxv2":
		return true
	default:
		return false
	}
}

// validateWidgetFormData 提交前校验关键控件（避免飞书返回难懂的 widget 错误）
func validateWidgetFormData(formData []map[string]interface{}, widgetFields []map[string]interface{}, list *model.BuildMasterList) error {
	fieldByID := make(map[string]map[string]interface{}, len(widgetFields))
	for _, f := range widgetFields {
		if id, _ := f["id"].(string); id != "" {
			fieldByID[id] = f
		}
	}
	submitted := make(map[string]map[string]interface{}, len(formData))
	for _, item := range formData {
		if id, _ := item["id"].(string); id != "" {
			submitted[id] = item
		}
	}

	for _, field := range widgetFields {
		id, _ := field["id"].(string)
		if id == "" {
			continue
		}
		name, _ := field["name"].(string)
		required, _ := field["required"].(bool)

		item, ok := submitted[id]
		if !ok {
			if required || isOptionWidgetType(field) {
				return fmt.Errorf("审批表单必填字段「%s」(id=%s) 未生成提交值，请在 form_fields 中配置 keyword", name, id)
			}
			continue
		}

		val := strings.TrimSpace(fmt.Sprintf("%v", item["value"]))
		if val == "" || val == "<nil>" {
			if isSiteWidgetField(field) {
				site := strings.TrimSpace(list.Site)
				if site == "" {
					return fmt.Errorf("发版站点为空，请先在发版清单中选择站点")
				}
				return fmt.Errorf("审批表单站点字段「%s」未匹配到选项，当前站点=%s，请检查飞书 option text 与 config.yaml sites 一致", name, site)
			}
			if isTypeWidgetField(field) {
				typeLabel := "常规发版"
				if list.Type == model.BuildMasterTypeUrgent {
					typeLabel = "紧急发版"
				}
				return fmt.Errorf("审批表单发版类型字段「%s」未匹配到选项，当前类型=%s，请检查飞书 option text", name, typeLabel)
			}
			if isOptionWidgetType(field) || required {
				return fmt.Errorf("审批表单字段「%s」(id=%s) 选项值为空，请配置 keyword 或检查飞书选项与系统数据是否一致", name, id)
			}
		}
	}
	return nil
}

// resolveFieldValue 根据 widget 字段的 keyword 解析值
func resolveFieldValue(field map[string]interface{}, list *model.BuildMasterList, typeVal, orderStr string, now time.Time, details []model.BuildMasterItemDetail, baseURL string) interface{} {
	keyword, _ := field["keyword"].(string)

	// 1) 优先使用 keyword
	if keyword != "" {
		switch {
		case keyword == "auto_id":
			return fmt.Sprintf("WF%d-%s", now.Unix(), list.ID[:8])

		case keyword == "auto_url":
			return fmt.Sprintf("%s/build-master/list/%s", baseURL, list.ID)

		case strings.HasPrefix(keyword, "fixed:"):
			return strings.TrimPrefix(keyword, "fixed:")

		case keyword == "list.site":
			return matchOptionValue(field, list.Site)

		case keyword == "list.type_label":
			return matchOptionValueForType(field, typeVal)

		case keyword == "list.order":
			return orderStr

		case keyword == "list.owner":
			return list.OwnerName

		case keyword == "list.publish_date":
			return list.PublishDate

		case keyword == "details" || strings.HasPrefix(keyword, "details"):
			return buildDetailsTableValue(field, details)

		default:
			if strings.HasPrefix(keyword, "list.") {
				fieldName := strings.TrimPrefix(keyword, "list.")
				raw := listFieldValue(list, fieldName)
				if fieldName == "site" || fieldName == "type_label" {
					if fieldName == "type_label" {
						return matchOptionValueForType(field, raw)
					}
					return matchOptionValue(field, raw)
				}
				return raw
			}
			return listFieldValue(list, keyword)
		}
	}

	// 2) 无 keyword，按 name 前缀兼容（hashcheck feishuApp）
	if v, ok := inferFieldValueByName(field, list, typeVal, orderStr, now, details, baseURL); ok {
		return v
	}

	// 3) 按名称/类型推断常见控件
	if isSiteWidgetField(field) {
		return matchOptionValue(field, list.Site)
	}
	if isTypeWidgetField(field) {
		return matchOptionValueForType(field, typeVal)
	}
	if isVatpWidgetField(field) {
		return matchYesNoOptionValue(field, "否")
	}
	if isSqlWidgetField(field) {
		return matchYesNoOptionValue(field, "否")
	}
	if isDetailsWidgetField(field) {
		return buildDetailsTableValue(field, details)
	}
	if isOrderWidgetField(field) {
		return orderStr
	}
	if isOwnerWidgetField(field) {
		return list.OwnerName
	}

	return ""
}

// matchOptionValue 从 widget option 列表中匹配显示文本/值，返回飞书要求的 option value
func matchOptionValue(field map[string]interface{}, raw string) string {
	raw = strings.TrimSpace(raw)
	optsRaw, ok := field["option"].([]interface{})
	if !ok || len(optsRaw) == 0 {
		return raw
	}
	if raw == "" {
		return ""
	}

	for _, o := range optsRaw {
		opt, ok := o.(map[string]interface{})
		if !ok {
			continue
		}
		text, _ := opt["text"].(string)
		val, _ := opt["value"].(string)
		text = strings.TrimSpace(text)
		val = strings.TrimSpace(val)
		if text == raw || val == raw || strings.EqualFold(text, raw) || strings.EqualFold(val, raw) {
			if val != "" {
				return val
			}
			return text
		}
	}

	// 站点别名：config.yaml 与飞书 option text 不完全一致时尝试包含匹配
	if isSiteWidgetField(field) {
		rawLower := strings.ToLower(raw)
		for _, o := range optsRaw {
			opt, ok := o.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := opt["text"].(string)
			val, _ := opt["value"].(string)
			text = strings.TrimSpace(text)
			val = strings.TrimSpace(val)
			textLower := strings.ToLower(text)
			if text != "" && (strings.Contains(textLower, rawLower) || strings.Contains(rawLower, textLower)) {
				if val != "" {
					return val
				}
				return text
			}
		}
	}

	// 单选/下拉：不能把显示文本直接提交给飞书
	if isOptionWidgetType(field) {
		return ""
	}
	return raw
}

// matchYesNoOptionValue 是/否 类单选（日月报、含有SQL）
func matchYesNoOptionValue(field map[string]interface{}, raw string) string {
	if v := matchOptionValue(field, raw); v != "" {
		return v
	}
	switch raw {
	case "否":
		if v := matchOptionValue(field, "no"); v != "" {
			return v
		}
	case "是":
		if v := matchOptionValue(field, "yes"); v != "" {
			return v
		}
	}
	return ""
}

// matchOptionValueForType 发版类型选项匹配（支持 常规/紧急 等别名）
func matchOptionValueForType(field map[string]interface{}, typeVal string) string {
	if v := matchOptionValue(field, typeVal); v != "" {
		return v
	}
	short := typeVal
	if strings.HasPrefix(typeVal, "常规") {
		short = "常规"
	} else if strings.HasPrefix(typeVal, "紧急") {
		short = "紧急"
	}
	if v := matchOptionValue(field, short); v != "" {
		return v
	}
	// 飞书选项可能为「常规发版」「紧急发版」的英文或其它文案，按 option text 包含关系兜底
	optsRaw, ok := field["option"].([]interface{})
	if !ok {
		return ""
	}
	wantUrgent := strings.Contains(typeVal, "紧急")
	for _, o := range optsRaw {
		opt, ok := o.(map[string]interface{})
		if !ok {
			continue
		}
		text, _ := opt["text"].(string)
		val, _ := opt["value"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if wantUrgent && strings.Contains(text, "紧急") {
			if val != "" {
				return val
			}
			return text
		}
		if !wantUrgent && strings.Contains(text, "常规") {
			if val != "" {
				return val
			}
			return text
		}
	}
	return ""
}

// listFieldValue 通过字段名获取 BuildMasterList 的值
func listFieldValue(list *model.BuildMasterList, fieldName string) string {
	switch fieldName {
	case "site":
		return list.Site
	case "type_label":
		if list.Type == model.BuildMasterTypeUrgent {
			return "紧急发版"
		}
		return "常规发版"
	case "order":
		if list.OrderDescribe != "" {
			return list.OrderDescribe
		}
		return "第" + strconv.Itoa(list.OrderNum) + "弹"
	case "owner":
		return list.OwnerName
	case "publish_date":
		return list.PublishDate
	case "id":
		return list.ID
	default:
		return ""
	}
}

// buildDetailRowByKeyword 根据 keyword 从 BuildMasterItemDetail 中取值
func buildDetailRowByKeyword(childrenRaw []interface{}, detail model.BuildMasterItemDetail) []map[string]interface{} {
	var row []map[string]interface{}
	for _, c := range childrenRaw {
		child, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		childKeyword, _ := child["keyword"].(string)
		childName, _ := child["name"].(string)

		val := resolveDetailValue(childKeyword, childName, detail)
		if val == "" {
			val = "暂无"
		}

		row = append(row, map[string]interface{}{
			"id":    child["id"],
			"type":  child["type"],
			"value": val,
		})
	}
	return row
}

// resolveDetailValue 根据 keyword 或 name 从明细对象取值
func resolveDetailValue(keyword, name string, detail model.BuildMasterItemDetail) string {
	// 优先使用 keyword
	if keyword != "" {
		if strings.HasPrefix(keyword, "detail.") {
			fieldName := strings.TrimPrefix(keyword, "detail.")
			return getDetailFieldValue(fieldName, detail)
		}
		if strings.HasPrefix(keyword, "fixed:") {
			return strings.TrimPrefix(keyword, "fixed:")
		}
		return getDetailFieldValue(keyword, detail)
	}

	// 无 keyword，按 name 前缀兼容（与飞书拉取的明细子字段名对齐）
	if name != "" {
		switch {
		case strings.HasPrefix(name, "项目类"), strings.HasPrefix(name, "操作项"):
			return detail.AppName
		case strings.HasPrefix(name, "版本号"), name == "版本", strings.HasPrefix(name, "版本"):
			return detail.Tag
		case strings.HasPrefix(name, "摘要"):
			return detail.Content
		case strings.HasPrefix(name, "操作类型"):
			return detail.SubType
		}
	}

	return ""
}

// getDetailFieldValue 反射获取 BuildMasterItemDetail 的字段值
func getDetailFieldValue(fieldName string, detail model.BuildMasterItemDetail) string {
	switch fieldName {
	case "app_name":
		return detail.AppName
	case "tag":
		return detail.Tag
	case "sub_type":
		return detail.SubType
	default:
		return ""
	}
}

// GetApprovalConfigs 获取已启用的审批平台配置
// GET /api/build-master/approval-configs
func (h *BuildMasterHandler) GetApprovalConfigs(c *gin.Context) {
	var configs []model.ApprovalConfig
	if err := h.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		c.JSON(500, model.Error(500, "query approval configs failed: "+err.Error()))
		return
	}
	type item struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Type          string   `json:"type"`
		ApproverIDs   []string `json:"approver_ids"`
		ApproverNames []string `json:"approver_names"`
	}
	out := make([]item, 0, len(configs))
	for _, cfg := range configs {
		var ids []string
		if err := json.Unmarshal([]byte(cfg.ApproverUserIDs), &ids); err != nil {
			ids = nil
		}
		names := make([]string, 0, len(ids))
		for _, id := range ids {
			var user model.User
			if err := h.db.Where("email = ?", id).First(&user).Error; err == nil {
				name := user.FullName
				if name == "" {
					name = user.Username
				}
				names = append(names, name)
			} else {
				names = append(names, id)
			}
		}
		out = append(out, item{ID: cfg.ID, Name: cfg.Name, Type: cfg.Type, ApproverIDs: ids, ApproverNames: names})
	}
	c.JSON(200, model.Success(out))
}

// SubmitForPlatformApproval 提交第三方审批（Feishu / DingTalk / WeChat）
// POST /api/build-master/lists/:id/submit-platform  body: {"config_id": "..."}
func (h *BuildMasterHandler) SubmitForPlatformApproval(c *gin.Context) {
	listID := c.Param("id")
	if listID == "" {
		c.JSON(400, model.Error(400, "list id required"))
		return
	}
	list, err := h.repo.GetByID(listID)
	if err != nil {
		c.JSON(404, model.Error(404, "list not found"))
		return
	}
	if list.Status != model.BuildMasterStatusFilling {
		c.JSON(400, model.Error(400, "只有填写中的状态才能提交审批"))
		return
	}

	var body struct {
		ConfigID string `json:"config_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ConfigID == "" {
		c.JSON(400, model.Error(400, "config_id required"))
		return
	}

	var cfg model.ApprovalConfig
	if err := h.db.Where("id = ? AND enabled = ?", body.ConfigID, true).First(&cfg).Error; err != nil {
		c.JSON(400, model.Error(400, "审批配置不存在或未启用"))
		return
	}

	platform := model.ApprovalPlatform(cfg.Type)
	provider, ok := h.approvalFactory.GetProvider(platform)
	if !ok {
		c.JSON(500, model.Error(500, "未找到审批平台提供者: "+cfg.Type))
		return
	}

	// 优先使用 settings 中下载的三方 widget scheme 构建表单数据
	scheme := "http"
	if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	var formData []map[string]interface{}
	var widgetFields []map[string]interface{}
	if cfg.FormFields != "" {
		if err := json.Unmarshal([]byte(cfg.FormFields), &widgetFields); err == nil && len(widgetFields) > 0 {
			formData = h.widgetFormData(list, widgetFields, baseURL)
		}
	}
	if formData == nil {
		formData = h.approvalFormData(list)
	} else if err := validateWidgetFormData(formData, widgetFields, list); err != nil {
		c.JSON(400, model.Error(400, err.Error()))
		return
	}
	formJSON, _ := json.Marshal(formData)
	logger.Infof("BuildMaster approval form: list=%s field_count=%d", list.ID, len(formData))

	applicantID := toString(c.MustGet("username"))
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok && uidStr != "" {
			var user model.User
			if err := h.db.Where("id = ?", uidStr).First(&user).Error; err == nil && user.Email != "" {
				applicantID = user.Email
			}
		}
	}

	fakeApproval := &model.Approval{
		Title:         "发版审批: " + list.PublishDate + " " + list.Site,
		ApplicantID:   applicantID,
		ApplicantName: applicantID,
	}

	// 各平台使用不同的 code 字段
	code := cfg.ApprovalCode
	if platform == model.ApprovalPlatformDingTalk {
		code = cfg.ProcessCode
	} else if platform == model.ApprovalPlatformWeChat {
		code = cfg.TemplateID
	}

	instanceCode, err := provider.CreateApprovalWithFormData(context.Background(), code, string(formJSON), fakeApproval)
	if err != nil {
		c.JSON(500, model.Error(500, "创建审批失败: "+err.Error()))
		return
	}

	list.ApprovalConfigID = cfg.ID
	list.ApprovalPlatform = cfg.Type
	list.ApprovalInstance = instanceCode
	list.Status = model.BuildMasterStatusApproving
	if err := h.repo.Update(list); err != nil {
		c.JSON(500, model.Error(500, "update list: "+err.Error()))
		return
	}

	// 同步创建审批总表记录，确保 /approvals 页面可见
	now := time.Now()
	realApplicantID := toString(c.MustGet("user_id"))
	approvalRecord := &model.Approval{
		ID:          uuid.New().String(),
		Title:       "发版审批: " + list.PublishDate + " " + list.Site,
		Description: "build_master_list_id:" + listID,
		Type:        model.ApprovalTypeDeployment,
		Status:      model.ApprovalStatusPending,
		Platform:    platform,
		ApplicantID: realApplicantID,
		ExternalID:  instanceCode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.db.Create(approvalRecord).Error; err != nil {
		logger.Errorf("同步创建审批记录失败: %v", err)
	}

	userName, _ := c.Get("username")
	_ = h.repo.CreateOperationLog(&model.BuildMasterOperationLog{
		ListID:       listID,
		OperatorID:   toString(c.MustGet("user_id")),
		OperatorName: toString(userName),
		Method:       "approval_submit",
		Body:         `[{"name":"status","old":"1","new":"2"},{"name":"approval_platform","old":"","new":"` + cfg.Type + `"}]`,
	})

	c.JSON(200, model.Success(map[string]string{
		"instance_code": instanceCode,
		"platform":      cfg.Type,
	}))
}

// ApprovalCallback 通用审批回调入口（Feishu / DingTalk / WeChat）
// 各平台需注册独立路由，此方法根据 platform 参数分发
func (h *BuildMasterHandler) ApprovalCallback(c *gin.Context) {
	platform := c.Param("platform")

	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(400, model.Error(400, "read body failed"))
		return
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		c.JSON(400, model.Error(400, "invalid json"))
		return
	}

	// Feishu URL verification
	if typ, _ := rawMap["type"].(string); typ == "url_verification" {
		challenge, _ := rawMap["challenge"].(string)
		c.JSON(200, gin.H{"challenge": challenge})
		return
	}

	// 使用 provider 解析回调
	prov := model.ApprovalPlatform(platform)
	provider, ok := h.approvalFactory.GetProvider(prov)
	if !ok {
		logger.Errorf("ApprovalCallback: no provider for platform %s", platform)
		c.JSON(400, model.Error(400, "unsupported platform"))
		return
	}

	result, err := provider.HandleCallback(context.Background(), rawMap)
	if err != nil {
		logger.Errorf("ApprovalCallback: handle callback error: %v", err)
		c.JSON(200, gin.H{"message": "ignored"})
		return
	}

	list, err := h.repo.FindByApprovalInstance(result.ApprovalID)
	if err != nil {
		logger.Infof("ApprovalCallback: no build master found for instance=%s", result.ApprovalID)
		c.JSON(200, gin.H{"message": "ignored"})
		return
	}

	method := "approval_" + string(result.Status)
	switch result.Status {
	case model.ApprovalStatusApproved:
		list.Status = model.BuildMasterStatusReleasing
		list.DeployStatus = model.BuildMasterDeployPending
		h.db.Model(&model.Approval{}).Where("external_id = ?", result.ApprovalID).Update("status", model.ApprovalStatusApproved)
	case model.ApprovalStatusRejected:
		list.Status = model.BuildMasterStatusFilling
		list.DeployStatus = ""
		h.db.Model(&model.Approval{}).Where("external_id = ?", result.ApprovalID).Update("status", model.ApprovalStatusRejected)
	case model.ApprovalStatusCanceled:
		list.Status = model.BuildMasterStatusFilling
		list.DeployStatus = ""
		list.ApprovalInstance = ""
		h.db.Model(&model.Approval{}).Where("external_id = ?", result.ApprovalID).Update("status", model.ApprovalStatusCanceled)
	default:
		c.JSON(200, gin.H{"message": "ignored"})
		return
	}

	if err := h.repo.Update(list); err != nil {
		logger.Errorf("ApprovalCallback: update list failed: %v", err)
		c.JSON(500, model.Error(500, "update failed"))
		return
	}

	_ = h.repo.CreateOperationLog(&model.BuildMasterOperationLog{
		ListID:       list.ID,
		OperatorID:   "",
		OperatorName: result.ApproverName,
		Method:       method,
		Body:         `[{"name":"status","old":"2","new":"` + strconv.Itoa(list.Status) + `"}]`,
	})

	c.JSON(200, gin.H{"message": "ok"})
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func logOperation(c *gin.Context, repo repository.BuildMasterRepository, listID, method string, changes []map[string]string) {
	bodyBytes, _ := json.Marshal(changes)
	operatorID, _ := c.Get("user_id")
	operatorName, _ := c.Get("username")
	_ = repo.CreateOperationLog(&model.BuildMasterOperationLog{
		ListID:       listID,
		OperatorID:   toString(operatorID),
		OperatorName: toString(operatorName),
		Method:       method,
		Body:         string(bodyBytes),
	})
}
