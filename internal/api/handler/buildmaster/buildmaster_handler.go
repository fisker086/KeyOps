package buildmaster

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BuildMasterHandler struct {
	repo *repository.BuildMasterRepository
}

func NewBuildMasterHandler(repo *repository.BuildMasterRepository) *BuildMasterHandler {
	return &BuildMasterHandler{repo: repo}
}

// List 按发版日期、类型查询列表
// GET /api/build-master/lists?publish_date=2025-02-12&type=0
func (h *BuildMasterHandler) List(c *gin.Context) {
	publishDate := c.Query("publish_date")
	if publishDate == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "publish_date required"))
		return
	}
	var typeFilter *int
	if t := c.Query("type"); t != "" {
		v, err := strconv.Atoi(t)
		if err != nil || (v != 0 && v != 1) {
			c.JSON(http.StatusBadRequest, model.Error(400, "type must be 0 or 1"))
			return
		}
		typeFilter = &v
	}
	lists, err := h.repo.ListByDateAndType(publishDate, typeFilter)
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
// POST /api/build-master/lists  body: { "publish_date": "2025-02-12", "type": 0 }
func (h *BuildMasterHandler) Create(c *gin.Context) {
	var body struct {
		PublishDate string `json:"publish_date" binding:"required"`
		Type        int    `json:"type"` // 0 常规 1 紧急
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

	orderNum, err := h.repo.MaxOrderForDateAndType(body.PublishDate, body.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "max order: "+err.Error()))
		return
	}

	list := &model.BuildMasterList{
		ID:          uuid.New().String(),
		PublishDate: body.PublishDate,
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
	// 操作记录：GORM 无 Django 式 signal，在 handler 层显式写入
	bodyBytes, _ := json.Marshal([]map[string]string{
		{"name": "type", "old": "", "new": strconv.Itoa(list.Type)},
		{"name": "publish_date", "old": "", "new": list.PublishDate},
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
// PATCH /api/build-master/lists/:id  body: { "status"?: 0-4, "order_describe"?: "", "hurried"?: 0-3 }
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
	// 操作记录：记录变更字段
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

// RecordsByQuery 获取某发版任务的操作记录（query list_id，避免 /lists/:id 与带子路径冲突）
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

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
