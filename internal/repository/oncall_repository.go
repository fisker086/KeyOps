package repository

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

// OnCallScheduleRepository 值班排班仓库
type OnCallScheduleRepository struct {
	db *gorm.DB
}

func NewOnCallScheduleRepository(db *gorm.DB) *OnCallScheduleRepository {
	return &OnCallScheduleRepository{db: db}
}

func (r *OnCallScheduleRepository) Create(schedule *model.OnCallSchedule) error {
	return r.db.Create(schedule).Error
}

func (r *OnCallScheduleRepository) Update(schedule *model.OnCallSchedule) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at 和 uid
	// updated_at 会由数据库自动更新（如果设置了 ON UPDATE CURRENT_TIMESTAMP）
	return r.db.Model(schedule).
		Select("schedule_name", "description", "department_id", "enabled").
		Updates(map[string]interface{}{
			"schedule_name": schedule.ScheduleName,
			"description":   schedule.Description,
			"department_id": schedule.DepartmentID,
			"enabled":       schedule.Enabled,
		}).Error
}

func (r *OnCallScheduleRepository) Delete(id uint) error {
	return r.db.Delete(&model.OnCallSchedule{}, "id = ?", id).Error
}

func (r *OnCallScheduleRepository) FindByID(id uint) (*model.OnCallSchedule, error) {
	var schedule model.OnCallSchedule
	err := r.db.Where("id = ?", id).First(&schedule).Error
	return &schedule, err
}

func (r *OnCallScheduleRepository) List(departmentID string, page, pageSize int) (int64, []model.OnCallSchedule, error) {
	var schedules []model.OnCallSchedule
	var total int64

	query := r.db.Model(&model.OnCallSchedule{})
	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&schedules).Error
	return total, schedules, err
}

// OnCallShiftRepository 值班班次仓库
type OnCallShiftRepository struct {
	db *gorm.DB
}

func NewOnCallShiftRepository(db *gorm.DB) *OnCallShiftRepository {
	return &OnCallShiftRepository{db: db}
}

func (r *OnCallShiftRepository) Create(shift *model.OnCallShift) error {
	return r.db.Create(shift).Error
}

func (r *OnCallShiftRepository) Update(shift *model.OnCallShift) error {
	// 使用 Omit 明确排除 created_at 和 updated_at 字段，然后使用 Updates 更新
	// 这样可以确保不会更新这些字段，即使它们被包含在结构体中
	return r.db.Model(shift).
		Omit("created_at", "updated_at").
		Updates(map[string]interface{}{
			"schedule_id": shift.ScheduleID,
			"user_id":     shift.UserID,
			"start_time":  shift.StartTime,
			"end_time":    shift.EndTime,
			"shift_type":  shift.ShiftType,
			"repeat_rule": shift.RepeatRule,
			"status":      shift.Status,
		}).Error
}

func (r *OnCallShiftRepository) Delete(id uint) error {
	return r.db.Delete(&model.OnCallShift{}, "id = ?", id).Error
}

func (r *OnCallShiftRepository) FindByID(id uint) (*model.OnCallShift, error) {
	var shift model.OnCallShift
	err := r.db.Where("id = ?", id).First(&shift).Error
	return &shift, err
}

func (r *OnCallShiftRepository) ListBySchedule(scheduleID uint) ([]model.OnCallShift, error) {
	var shifts []model.OnCallShift
	err := r.db.Where("schedule_id = ? AND status = ?", scheduleID, "active").
		Order("start_time ASC").Find(&shifts).Error
	return shifts, err
}

// ListByScheduleWithUser 获取排班的班次列表（包含用户名，返回所有状态的班次）
func (r *OnCallShiftRepository) ListByScheduleWithUser(scheduleID uint) ([]model.OnCallShiftWithUser, error) {
	var results []model.OnCallShiftWithUser
	err := r.db.Table("on_call_shifts").
		Select("on_call_shifts.*, users.username as username").
		Joins("LEFT JOIN users ON on_call_shifts.user_id = users.id").
		Where("on_call_shifts.schedule_id = ?", scheduleID).
		Order("on_call_shifts.start_time ASC").
		Scan(&results).Error
	return results, err
}

// GetCurrentOnCallUsers 获取当前值班的用户列表
func (r *OnCallShiftRepository) GetCurrentOnCallUsers(departmentID string, atTime time.Time) ([]string, error) {
	var userIDs []string

	// 查找所有启用的排班
	var schedules []model.OnCallSchedule
	query := r.db.Where("enabled = ?", true)
	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}
	if err := query.Find(&schedules).Error; err != nil {
		return nil, err
	}

	scheduleIDs := make([]uint, 0, len(schedules))
	for _, s := range schedules {
		scheduleIDs = append(scheduleIDs, s.ID)
	}

	if len(scheduleIDs) == 0 {
		return []string{}, nil
	}

	// 查找当前时间范围内的班次
	err := r.db.Model(&model.OnCallShift{}).
		Where("schedule_id IN ? AND status = ? AND start_time <= ? AND end_time >= ?",
			scheduleIDs, "active", atTime, atTime).
		Pluck("user_id", &userIDs).Error

	// 去重
	uniqueMap := make(map[string]bool)
	var uniqueUserIDs []string
	for _, id := range userIDs {
		if !uniqueMap[id] {
			uniqueMap[id] = true
			uniqueUserIDs = append(uniqueUserIDs, id)
		}
	}

	return uniqueUserIDs, err
}

// GetOnCallUserForSchedule 获取指定排班的当前值班用户
func (r *OnCallShiftRepository) GetOnCallUserForSchedule(scheduleID uint, atTime time.Time) (string, error) {
	var userID string
	err := r.db.Model(&model.OnCallShift{}).
		Where("schedule_id = ? AND status = ? AND start_time <= ? AND end_time >= ?",
			scheduleID, "active", atTime, atTime).
		Order("created_at ASC").
		Limit(1).
		Pluck("user_id", &userID).Error
	return userID, err
}

// OnCallAssignmentRepository 告警分配仓库
type OnCallAssignmentRepository struct {
	db *gorm.DB
}

func NewOnCallAssignmentRepository(db *gorm.DB) *OnCallAssignmentRepository {
	return &OnCallAssignmentRepository{db: db}
}

func (r *OnCallAssignmentRepository) Create(assignment *model.OnCallAssignment) error {
	return r.db.Create(assignment).Error
}

func (r *OnCallAssignmentRepository) FindByAlertID(alertID uint64) (*model.OnCallAssignment, error) {
	var assignment model.OnCallAssignment
	err := r.db.Where("alert_id = ?", alertID).Order("assigned_at DESC").First(&assignment).Error
	return &assignment, err
}

func (r *OnCallAssignmentRepository) ListByUser(userID string, page, pageSize int) (int64, []model.OnCallAssignment, error) {
	var assignments []model.OnCallAssignment
	var total int64

	if err := r.db.Model(&model.OnCallAssignment{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, nil, err
	}

	offset := (page - 1) * pageSize
	err := r.db.Where("user_id = ?", userID).
		Offset(offset).Limit(pageSize).
		Order("assigned_at DESC").Find(&assignments).Error
	return total, assignments, err
}

func (r *OnCallAssignmentRepository) ListByAlert(alertID uint64) ([]model.OnCallAssignment, error) {
	var assignments []model.OnCallAssignment
	err := r.db.Where("alert_id = ?", alertID).Order("assigned_at DESC").Find(&assignments).Error
	return assignments, err
}
