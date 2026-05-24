package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type BuildMasterRepository interface {
	Create(list *model.BuildMasterList) error
	GetByID(id string) (*model.BuildMasterList, error)
	Update(list *model.BuildMasterList) error
	ListByDateAndType(publishDate string, typeFilter *int) ([]model.BuildMasterList, error)
	ListByDateRange(from, to string, typeFilter *int) ([]model.BuildMasterList, error)
	MaxOrderForDateAndType(publishDate string, typ int) (int, error)
	CreateOperationLog(log *model.BuildMasterOperationLog) error
	ListOperationLogsByListID(listID string) ([]model.BuildMasterOperationLog, error)
}

type buildMasterRepository struct {
	db *gorm.DB
}

func NewBuildMasterRepository(db *gorm.DB) BuildMasterRepository {
	return &buildMasterRepository{db: db}
}

func (r *buildMasterRepository) Create(list *model.BuildMasterList) error {
	return r.db.Create(list).Error
}

func (r *buildMasterRepository) GetByID(id string) (*model.BuildMasterList, error) {
	var list model.BuildMasterList
	err := r.db.Where("id = ?", id).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *buildMasterRepository) Update(list *model.BuildMasterList) error {
	return r.db.Save(list).Error
}

func (r *buildMasterRepository) ListByDateAndType(publishDate string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date = ?", publishDate)
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("order_num ASC, created_at ASC").Find(&list).Error
	return list, err
}

func (r *buildMasterRepository) ListByDateRange(from, to string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date >= ? AND publish_date <= ?", from, to)
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("publish_date DESC, order_num ASC").Find(&list).Error
	return list, err
}

func (r *buildMasterRepository) MaxOrderForDateAndType(publishDate string, typ int) (int, error) {
	var max *int
	err := r.db.Model(&model.BuildMasterList{}).
		Where("publish_date = ? AND type = ?", publishDate, typ).
		Select("COALESCE(MAX(order_num), 0)").Scan(&max).Error
	if err != nil || max == nil {
		return 1, err
	}
	return *max + 1, nil
}

func (r *buildMasterRepository) CreateOperationLog(log *model.BuildMasterOperationLog) error {
	return r.db.Create(log).Error
}

func (r *buildMasterRepository) ListOperationLogsByListID(listID string) ([]model.BuildMasterOperationLog, error) {
	var logs []model.BuildMasterOperationLog
	err := r.db.Where("list_id = ?", listID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}
