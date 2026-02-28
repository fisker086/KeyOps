package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type BuildMasterRepository struct {
	db *gorm.DB
}

func NewBuildMasterRepository(db *gorm.DB) *BuildMasterRepository {
	return &BuildMasterRepository{db: db}
}

func (r *BuildMasterRepository) Create(list *model.BuildMasterList) error {
	return r.db.Create(list).Error
}

func (r *BuildMasterRepository) GetByID(id string) (*model.BuildMasterList, error) {
	var list model.BuildMasterList
	err := r.db.Where("id = ?", id).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *BuildMasterRepository) Update(list *model.BuildMasterList) error {
	return r.db.Save(list).Error
}

// ListByDateAndType 按发版日期和类型查询列表（同一天同类型的多弹按 order 排序）
func (r *BuildMasterRepository) ListByDateAndType(publishDate string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date = ?", publishDate)
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("order_num ASC, created_at ASC").Find(&list).Error
	return list, err
}

// ListByDateRange 按日期范围查询（用于入口页「按日期查看」）
func (r *BuildMasterRepository) ListByDateRange(from, to string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date >= ? AND publish_date <= ?", from, to)
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("publish_date DESC, order_num ASC").Find(&list).Error
	return list, err
}

// MaxOrderForDateAndType 获取某日某类型的最大 order，新建时 order = max+1
func (r *BuildMasterRepository) MaxOrderForDateAndType(publishDate string, typ int) (int, error) {
	var max *int
	err := r.db.Model(&model.BuildMasterList{}).
		Where("publish_date = ? AND type = ?", publishDate, typ).
		Select("COALESCE(MAX(order_num), 0)").Scan(&max).Error
	if err != nil || max == nil {
		return 1, err
	}
	return *max + 1, nil
}

// CreateOperationLog 写入操作记录（GORM 无 Django 式 signal，在业务层显式调用）
func (r *BuildMasterRepository) CreateOperationLog(log *model.BuildMasterOperationLog) error {
	return r.db.Create(log).Error
}

// ListOperationLogsByListID 按 list_id 查操作记录，倒序
func (r *BuildMasterRepository) ListOperationLogsByListID(listID string) ([]model.BuildMasterOperationLog, error) {
	var logs []model.BuildMasterOperationLog
	err := r.db.Where("list_id = ?", listID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}
