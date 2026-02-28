package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type QueryLogRepository struct {
	db *gorm.DB
}

func NewQueryLogRepository(db *gorm.DB) *QueryLogRepository {
	return &QueryLogRepository{db: db}
}

// Create 创建查询日志
func (r *QueryLogRepository) Create(log *model.QueryLog) error {
	return r.db.Create(log).Error
}

// GetByID 根据ID获取日志
func (r *QueryLogRepository) GetByID(id uint) (*model.QueryLog, error) {
	var log model.QueryLog
	err := r.db.Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// List 获取日志列表
func (r *QueryLogRepository) List(offset, limit int, filters map[string]interface{}) ([]model.QueryLog, int64, error) {
	var logs []model.QueryLog
	var total int64

	query := r.db.Model(&model.QueryLog{})

	// 应用过滤条件
	if userID, ok := filters["user_id"].(string); ok && userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if instanceID, ok := filters["instance_id"].(uint); ok && instanceID > 0 {
		query = query.Where("instance_id = ?", instanceID)
	}
	if dbType, ok := filters["db_type"].(string); ok && dbType != "" {
		query = query.Where("db_type = ?", dbType)
	}
	if queryType, ok := filters["query_type"].(string); ok && queryType != "" {
		query = query.Where("query_type = ?", queryType)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if startTime, ok := filters["start_time"].(string); ok && startTime != "" {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime, ok := filters["end_time"].(string); ok && endTime != "" {
		query = query.Where("created_at <= ?", endTime)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("query_content LIKE ? OR username LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取列表
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
