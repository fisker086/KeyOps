package repository

import (
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// GetDB 返回数据库实例（用于Service层的复杂查询）
func (r *SessionRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *SessionRepository) CreateLoginRecord(record *model.LoginRecord) error {
	return r.db.Create(record).Error
}

func (r *SessionRepository) UpdateLogoutTime(id string) error {
	now := time.Now()
	return r.db.Model(&model.LoginRecord{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"logout_time": now,
			"status":      "completed",
		}).Error
}

func (r *SessionRepository) CalculateDuration(id string) error {
	var record model.LoginRecord
	if err := r.db.Where("id = ?", id).First(&record).Error; err != nil {
		return err
	}

	if record.LogoutTime != nil {
		duration := int(record.LogoutTime.Sub(record.LoginTime).Seconds())
		return r.db.Model(&model.LoginRecord{}).Where("id = ?", id).
			Update("duration", duration).Error
	}

	return nil
}

func (r *SessionRepository) FindLoginRecords(page, pageSize int, hostID string) ([]model.LoginRecord, int64, error) {
	var records []model.LoginRecord
	var total int64

	// 只查询虚拟机登录记录（host_id 不为空且不为空字符串）
	query := r.db.Model(&model.LoginRecord{}).
		Where("host_id IS NOT NULL AND host_id != ''")

	if hostID != "" {
		query = query.Where("host_id = ?", hostID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("login_time DESC").Find(&records).Error

	return records, total, err
}

// FindLoginRecordsByUser 查询登录记录（支持按用户过滤，userID为空则返回所有）
func (r *SessionRepository) FindLoginRecordsByUser(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
	var total int64

	// 只查询虚拟机登录记录（host_id 不为空且不为空字符串）
	query := r.db.Model(&model.LoginRecord{}).
		Where("host_id IS NOT NULL AND host_id != ''")

	if hostID != "" {
		query = query.Where("host_id = ?", hostID)
	}

	// 如果指定了用户ID，则只查询该用户的记录
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	// 使用LEFT JOIN查询连接类型
	var results []model.LoginRecordWithType
	joinQuery := r.db.Table("login_records lr").
		Select("lr.*, COALESCE(sr.connection_type, 'webshell') as connection_type").
		Joins("LEFT JOIN session_recordings sr ON lr.session_id = sr.session_id").
		Where("lr.host_id IS NOT NULL AND lr.host_id != ''")

	if hostID != "" {
		joinQuery = joinQuery.Where("lr.host_id = ?", hostID)
	}

	if userID != "" {
		joinQuery = joinQuery.Where("lr.user_id = ?", userID)
	}

	err := joinQuery.
		Order("lr.login_time DESC").
		Offset(offset).
		Limit(pageSize).
		Scan(&results).Error

	if err != nil {
		return nil, 0, err
	}

	return results, total, err
}

func (r *SessionRepository) GetRecentLogins(limit int) ([]model.LoginRecord, error) {
	var records []model.LoginRecord
	// 只查询虚拟机登录记录（host_id 不为空且不为空字符串）
	err := r.db.Where("host_id IS NOT NULL AND host_id != ''").
		Order("login_time DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetRecentLoginsByUser 获取指定用户的最近登录记录（如果userID为空则返回所有）
func (r *SessionRepository) GetRecentLoginsByUser(limit int, userID string) ([]model.LoginRecord, error) {
	var records []model.LoginRecord
	query := r.db.Where("host_id IS NOT NULL AND host_id != ''")

	// 如果指定了用户ID，则只查询该用户的记录
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	err := query.Order("login_time DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// FindSessionHistories 查询SSH会话历史记录（用于首页展示）
func (r *SessionRepository) FindSessionHistories(page, pageSize int, hostID string) ([]map[string]interface{}, int64, error) {
	var total int64

	// 构建Count查询（不能包含Select和Joins）
	countQuery := r.db.Table("session_histories sh")
	if hostID != "" {
		countQuery = countQuery.Where("sh.host_id = ?", hostID)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 构建数据查询
	query := r.db.Table("session_histories sh").
		Select("sh.session_id, sh.host_id, sh.username, sh.host_ip, sh.start_time, sh.end_time, sh.status, h.name as host_name").
		Joins("LEFT JOIN hosts h ON sh.host_id = h.id")

	if hostID != "" {
		query = query.Where("sh.host_id = ?", hostID)
	}

	// 获取分页数据
	var results []map[string]interface{}
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("sh.start_time DESC").Find(&results).Error

	return results, total, err
}

func (r *SessionRepository) CountRecentLogins(hours int) (int64, error) {
	var count int64
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	err := r.db.Model(&model.LoginRecord{}).
		Where("login_time >= ?", cutoff).
		Count(&count).Error
	return count, err
}

// CountRecentLoginsByUser 统计指定用户的最近登录次数（如果userID为空则统计所有）
// 只统计成功的登录（status IN ('active', 'completed')），排除失败的连接尝试
func (r *SessionRepository) CountRecentLoginsByUser(hours int, userID string) (int64, error) {
	var count int64
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	query := r.db.Model(&model.LoginRecord{}).
		Where("login_time >= ? AND status IN (?, ?)", cutoff, "active", "completed")

	// 如果指定了用户ID，则只统计该用户的记录
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	err := query.Count(&count).Error
	return count, err
}

// CountTodayLoginsByUser 统计指定用户的今日登录次数（从今天0点开始，如果userID为空则统计所有）
// 只统计成功的登录（status IN ('active', 'completed')），排除失败的连接尝试
func (r *SessionRepository) CountTodayLoginsByUser(userID string) (int64, error) {
	var count int64
	// 获取今天0点的时间
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	query := r.db.Model(&model.LoginRecord{}).
		Where("login_time >= ? AND status IN (?, ?)", todayStart, "active", "completed")

	// 如果指定了用户ID，则只统计该用户的记录
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	err := query.Count(&count).Error
	return count, err
}

// SSH Session 相关方法已删除，统一使用 session_recordings 表管理会话

// ===== Session Recording Methods =====

func (r *SessionRepository) CreateSessionRecording(recording *model.SessionRecording) error {
	return r.db.Create(recording).Error
}

func (r *SessionRepository) FindSessionRecordings(page, pageSize int, search string) ([]model.SessionRecording, int64, error) {
	var recordings []model.SessionRecording
	var total int64

	query := r.db.Model(&model.SessionRecording{})

	// 过滤掉失败状态的会话，只显示成功建立的会话
	query = query.Where("status != ?", "failed")

	if search != "" {
		query = query.Where("host_name LIKE ? OR username LIKE ? OR session_id LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("start_time DESC").Find(&recordings).Error

	return recordings, total, err
}

func (r *SessionRepository) FindSessionRecordingByID(sessionID string) (*model.SessionRecording, error) {
	var recording model.SessionRecording
	err := r.db.Where("session_id = ?", sessionID).First(&recording).Error
	if err != nil {
		return nil, err
	}
	return &recording, nil
}

func (r *SessionRepository) UpdateSessionRecording(sessionID string, updates map[string]interface{}) error {
	return r.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", sessionID).
		Updates(updates).Error
}

// ===== Command Record Methods =====

func (r *SessionRepository) CreateCommandRecord(record *model.CommandRecord) error {
	return r.db.Create(record).Error
}

func (r *SessionRepository) FindCommandRecords(page, pageSize int, search, hostFilter string) ([]model.CommandRecord, int64, error) {
	var commands []model.CommandRecord
	var total int64

	query := r.db.Model(&model.CommandRecord{})

	if search != "" {
		query = query.Where("command LIKE ? OR host_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if hostFilter != "" && hostFilter != "all" {
		query = query.Where("host_ip = ?", hostFilter)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("executed_at DESC").Find(&commands).Error

	return commands, total, err
}

func (r *SessionRepository) FindCommandsBySessionID(sessionID string) ([]model.CommandRecord, error) {
	var commands []model.CommandRecord
	err := r.db.Where("session_id = ?", sessionID).Order("executed_at ASC").Find(&commands).Error
	return commands, err
}

func (r *SessionRepository) CountCommandsBySessionID(sessionID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.CommandRecord{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}
