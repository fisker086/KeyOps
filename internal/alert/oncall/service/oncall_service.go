package service

import (
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// OnCallService 值班排班服务
type OnCallService struct {
	scheduleRepo   *repository.OnCallScheduleRepository
	shiftRepo      *repository.OnCallShiftRepository
	assignmentRepo *repository.OnCallAssignmentRepository
}

func NewOnCallService(
	scheduleRepo *repository.OnCallScheduleRepository,
	shiftRepo *repository.OnCallShiftRepository,
	assignmentRepo *repository.OnCallAssignmentRepository,
) *OnCallService {
	return &OnCallService{
		scheduleRepo:   scheduleRepo,
		shiftRepo:      shiftRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ==================== 排班管理 ====================

func (s *OnCallService) CreateSchedule(schedule *model.OnCallSchedule) (*model.OnCallSchedule, error) {
	if err := s.scheduleRepo.Create(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *OnCallService) UpdateSchedule(id uint, schedule *model.OnCallSchedule) (*model.OnCallSchedule, error) {
	schedule.ID = id
	if err := s.scheduleRepo.Update(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *OnCallService) DeleteSchedule(id uint) error {
	return s.scheduleRepo.Delete(id)
}

func (s *OnCallService) GetSchedule(id uint) (*model.OnCallSchedule, error) {
	return s.scheduleRepo.FindByID(id)
}

func (s *OnCallService) ListSchedules(departmentID string, page, pageSize int) (int64, []model.OnCallSchedule, error) {
	return s.scheduleRepo.List(departmentID, page, pageSize)
}

// ==================== 班次管理 ====================

func (s *OnCallService) CreateShift(shift *model.OnCallShift) (*model.OnCallShift, error) {
	if err := s.shiftRepo.Create(shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (s *OnCallService) UpdateShift(id uint, shift *model.OnCallShift) (*model.OnCallShift, error) {
	shift.ID = id
	if err := s.shiftRepo.Update(shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (s *OnCallService) DeleteShift(id uint) error {
	return s.shiftRepo.Delete(id)
}

func (s *OnCallService) GetShift(id uint) (*model.OnCallShift, error) {
	return s.shiftRepo.FindByID(id)
}

func (s *OnCallService) ListShiftsBySchedule(scheduleID uint) ([]model.OnCallShift, error) {
	return s.shiftRepo.ListBySchedule(scheduleID)
}

func (s *OnCallService) ListShiftsByScheduleWithUser(scheduleID uint) ([]model.OnCallShiftWithUser, error) {
	return s.shiftRepo.ListByScheduleWithUser(scheduleID)
}

// ==================== 值班查询 ====================

// GetCurrentOnCallUsers 获取当前值班的用户列表
func (s *OnCallService) GetCurrentOnCallUsers(departmentID string) ([]string, error) {
	return s.shiftRepo.GetCurrentOnCallUsers(departmentID, time.Now())
}

// GetOnCallUserForSchedule 获取指定排班的当前值班用户
func (s *OnCallService) GetOnCallUserForSchedule(scheduleID uint) (string, error) {
	return s.shiftRepo.GetOnCallUserForSchedule(scheduleID, time.Now())
}

// GetOnCallScheduleForTime 获取指定时间段的排班信息
func (s *OnCallService) GetOnCallScheduleForTime(departmentID string, startTime, endTime time.Time) ([]model.OnCallShift, error) {
	// 查找所有启用的排班
	_, schedules, err := s.scheduleRepo.List(departmentID, 1, 1000)
	if err != nil {
		return nil, err
	}

	var allShifts []model.OnCallShift
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		shifts, err := s.shiftRepo.ListBySchedule(schedule.ID)
		if err != nil {
			continue
		}
		// 过滤时间范围
		for _, shift := range shifts {
			if shift.Status == "active" &&
				shift.StartTime.Before(endTime) &&
				shift.EndTime.After(startTime) {
				allShifts = append(allShifts, shift)
			}
		}
	}

	return allShifts, nil
}

// ==================== 告警分配 ====================

// AutoAssignAlert 自动分配告警给当前值班人员
func (s *OnCallService) AutoAssignAlert(alertID uint64, departmentID string) error {
	// 获取当前值班用户
	userIDs, err := s.GetCurrentOnCallUsers(departmentID)
	if err != nil {
		return err
	}

	if len(userIDs) == 0 {
		return fmt.Errorf("no on-call users found for department %s", departmentID)
	}

	// 分配给第一个值班用户（可以扩展为轮询分配）
	userID := userIDs[0]

	assignment := &model.OnCallAssignment{
		AlertID:      alertID,
		UserID:       userID,
		AssignedAt:   time.Now(),
		AutoAssigned: true,
	}

	return s.assignmentRepo.Create(assignment)
}

// ManualAssignAlert 手动分配告警
func (s *OnCallService) ManualAssignAlert(alertID uint64, userID string, assignedBy string, shiftID *uint) error {
	assignment := &model.OnCallAssignment{
		AlertID:      alertID,
		UserID:       userID,
		ShiftID:       shiftID,
		AssignedAt:   time.Now(),
		AssignedBy:   assignedBy,
		AutoAssigned: false,
	}

	return s.assignmentRepo.Create(assignment)
}

// GetAssignmentByAlert 获取告警的分配信息
func (s *OnCallService) GetAssignmentByAlert(alertID uint64) (*model.OnCallAssignment, error) {
	return s.assignmentRepo.FindByAlertID(alertID)
}

// ListAssignmentsByUser 获取用户的告警分配列表
func (s *OnCallService) ListAssignmentsByUser(userID string, page, pageSize int) (int64, []model.OnCallAssignment, error) {
	return s.assignmentRepo.ListByUser(userID, page, pageSize)
}

// ListAssignmentsByAlert 获取告警的所有分配记录
func (s *OnCallService) ListAssignmentsByAlert(alertID uint64) ([]model.OnCallAssignment, error) {
	return s.assignmentRepo.ListByAlert(alertID)
}

