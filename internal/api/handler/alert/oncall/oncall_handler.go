package oncall

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/model"
	oncallService "github.com/fisker086/keyops/internal/alert/oncall/service"
	"github.com/gin-gonic/gin"
)

// OnCallHandler 值班排班处理器
type OnCallHandler struct {
	service *oncallService.OnCallService
}

func NewOnCallHandler(service *oncallService.OnCallService) *OnCallHandler {
	return &OnCallHandler{service: service}
}

// ==================== 排班管理 ====================

// CreateSchedule 创建排班
func (h *OnCallHandler) CreateSchedule(c *gin.Context) {
	var req model.OnCallSchedule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	schedule, err := h.service.CreateSchedule(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(schedule))
}

// UpdateSchedule 更新排班
func (h *OnCallHandler) UpdateSchedule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.OnCallSchedule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	schedule, err := h.service.UpdateSchedule(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(schedule))
}

// DeleteSchedule 删除排班
func (h *OnCallHandler) DeleteSchedule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteSchedule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetSchedule 获取排班详情
func (h *OnCallHandler) GetSchedule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	schedule, err := h.service.GetSchedule(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "排班不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(schedule))
}

// ListSchedules 获取排班列表
func (h *OnCallHandler) ListSchedules(c *gin.Context) {
	departmentID := c.DefaultQuery("department_id", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, schedules, err := h.service.ListSchedules(departmentID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    schedules,
		"total":   total,
	})
}

// ==================== 班次管理 ====================

// CreateShift 创建班次
func (h *OnCallHandler) CreateShift(c *gin.Context) {
	var req model.OnCallShift
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	shift, err := h.service.CreateShift(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(shift))
}

// UpdateShift 更新班次
func (h *OnCallHandler) UpdateShift(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req model.OnCallShift
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 清除 created_at 和 updated_at 字段，避免更新时出错
	req.CreatedAt = time.Time{}
	req.UpdatedAt = time.Time{}

	shift, err := h.service.UpdateShift(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(shift))
}

// DeleteShift 删除班次
func (h *OnCallHandler) DeleteShift(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.DeleteShift(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetShift 获取班次详情
func (h *OnCallHandler) GetShift(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	shift, err := h.service.GetShift(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "班次不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(shift))
}

// ListShiftsBySchedule 获取排班的班次列表
func (h *OnCallHandler) ListShiftsBySchedule(c *gin.Context) {
	scheduleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// 使用包含用户名的查询
	shiftsWithUser, err := h.service.ListShiftsByScheduleWithUser(uint(scheduleID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 转换为 OnCallShift 格式，但包含 username 字段
	shifts := make([]model.OnCallShift, len(shiftsWithUser))
	for i, swu := range shiftsWithUser {
		shifts[i] = swu.OnCallShift
		// 将 username 添加到 JSON 中（通过扩展字段）
		// 注意：由于 OnCallShift 结构体没有 Username 字段，我们需要创建一个包含 username 的响应
	}

	// 返回包含用户名的数据
	type ShiftWithUsername struct {
		model.OnCallShift
		Username string `json:"username"`
	}
	result := make([]ShiftWithUsername, len(shiftsWithUser))
	for i, swu := range shiftsWithUser {
		result[i] = ShiftWithUsername{
			OnCallShift: swu.OnCallShift,
			Username:    swu.Username,
		}
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// ==================== 值班查询 ====================

// GetCurrentOnCallUsers 获取当前值班用户
func (h *OnCallHandler) GetCurrentOnCallUsers(c *gin.Context) {
	departmentID := c.DefaultQuery("department_id", "")
	userIDs, err := h.service.GetCurrentOnCallUsers(departmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(userIDs))
}

// GetOnCallUserForSchedule 获取指定排班的当前值班用户
func (h *OnCallHandler) GetOnCallUserForSchedule(c *gin.Context) {
	scheduleID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID, err := h.service.GetOnCallUserForSchedule(uint(scheduleID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"user_id": userID}))
}

// ==================== 告警分配 ====================

// AutoAssignAlert 自动分配告警
func (h *OnCallHandler) AutoAssignAlert(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alert_id"), 10, 64)
	departmentID := c.DefaultQuery("department_id", "")

	if err := h.service.AutoAssignAlert(alertID, departmentID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ManualAssignAlert 手动分配告警
func (h *OnCallHandler) ManualAssignAlert(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alert_id"), 10, 64)
	var req struct {
		UserID     string `json:"user_id" binding:"required"`
		ShiftID    *uint  `json:"shift_id"`
		AssignedBy string `json:"assigned_by"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.ManualAssignAlert(alertID, req.UserID, req.AssignedBy, req.ShiftID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// GetAssignmentByAlert 获取告警的分配信息
func (h *OnCallHandler) GetAssignmentByAlert(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alert_id"), 10, 64)
	assignment, err := h.service.GetAssignmentByAlert(alertID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "分配记录不存在"))
		return
	}

	c.JSON(http.StatusOK, model.Success(assignment))
}

// ListAssignmentsByUser 获取用户的告警分配列表
func (h *OnCallHandler) ListAssignmentsByUser(c *gin.Context) {
	userID := c.Param("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	total, assignments, err := h.service.ListAssignmentsByUser(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    assignments,
		"total":   total,
	})
}

