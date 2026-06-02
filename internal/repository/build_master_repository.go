package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type BuildMasterRepository interface {
	// List
	Create(list *model.BuildMasterList) error
	GetByID(id string) (*model.BuildMasterList, error)
	Update(list *model.BuildMasterList) error
	ListByDateAndType(publishDate string, site string, typeFilter *int) ([]model.BuildMasterList, error)
	ListByDateRange(from, to string, site string, typeFilter *int) ([]model.BuildMasterList, error)
	CountBySite() (map[string]int64, error)
	LatestPublishAtBySite() (map[string]string, error)
	MaxOrderForDateAndType(publishDate string, site string, typ int) (int, error)
	// OperationLog
	CreateOperationLog(log *model.BuildMasterOperationLog) error
	ListOperationLogsByListID(listID string) ([]model.BuildMasterOperationLog, error)
	// Item（工作项分类）
	CreateItem(item *model.BuildMasterItem) error
	ListItemsByListID(listID string) ([]model.BuildMasterItem, error)
	DeleteItem(id uint) error
	ListCheckItems() ([]model.BuildMasterCheckItem, error)
	// ItemDetail（条目详情）
	CreateDetail(detail *model.BuildMasterItemDetail) error
	GetDetailByID(id uint) (*model.BuildMasterItemDetail, error)
	UpdateDetail(detail *model.BuildMasterItemDetail) error
	DeleteDetail(id uint) error
	ListDetailsByItemID(itemID uint) ([]model.BuildMasterItemDetail, error)
	ListDetailsByListID(listID string) ([]model.BuildMasterItemDetail, error)
	UpdateDetailsStatusByItemID(itemID uint, status int) error
	// Approval（审批）
	CreateApproval(approval *model.BuildMasterApproval) error
	UpdateApproval(approval *model.BuildMasterApproval) error
	ListApprovalsByListID(listID string) ([]model.BuildMasterApproval, error)
	GetPendingApprovalsByListID(listID string) ([]model.BuildMasterApproval, error)
	FindByApprovalInstance(instanceCode string) (*model.BuildMasterList, error)
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

func (r *buildMasterRepository) ListByDateAndType(publishDate string, site string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date = ?", publishDate)
	if site != "" {
		q = q.Where("site = ?", site)
	}
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("order_num ASC, created_at ASC").Find(&list).Error
	return list, err
}

func (r *buildMasterRepository) ListByDateRange(from, to string, site string, typeFilter *int) ([]model.BuildMasterList, error) {
	var list []model.BuildMasterList
	q := r.db.Where("publish_date >= ? AND publish_date <= ?", from, to)
	if site != "" {
		q = q.Where("site = ?", site)
	}
	if typeFilter != nil {
		q = q.Where("type = ?", *typeFilter)
	}
	err := q.Order("publish_date DESC, order_num ASC").Find(&list).Error
	return list, err
}

func (r *buildMasterRepository) CountBySite() (map[string]int64, error) {
	var rows []struct {
		Site  string `gorm:"column:site"`
		Count int64  `gorm:"column:count"`
	}
	err := r.db.Model(&model.BuildMasterList{}).
		Select("site, COUNT(*) as count").
		Where("site <> ''").
		Group("site").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Site] = row.Count
	}
	return result, nil
}

func (r *buildMasterRepository) LatestPublishAtBySite() (map[string]string, error) {
	var rows []struct {
		Site     string `gorm:"column:site"`
		LatestAt string `gorm:"column:latest_at"`
	}
	err := r.db.Model(&model.BuildMasterList{}).
		Select("site, MAX(updated_at) as latest_at").
		Where("site <> ''").
		Group("site").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.Site] = row.LatestAt
	}
	return result, nil
}

func (r *buildMasterRepository) MaxOrderForDateAndType(publishDate string, site string, typ int) (int, error) {
	var max *int
	err := r.db.Model(&model.BuildMasterList{}).
		Where("publish_date = ? AND site = ? AND type = ?", publishDate, site, typ).
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

func (r *buildMasterRepository) CreateItem(item *model.BuildMasterItem) error {
	return r.db.Create(item).Error
}

func (r *buildMasterRepository) ListItemsByListID(listID string) ([]model.BuildMasterItem, error) {
	var items []model.BuildMasterItem
	err := r.db.Where("list_id = ?", listID).Order("order_num ASC, created_at ASC").Find(&items).Error
	return items, err
}

func (r *buildMasterRepository) DeleteItem(id uint) error {
	return r.db.Delete(&model.BuildMasterItem{}, id).Error
}

func (r *buildMasterRepository) ListCheckItems() ([]model.BuildMasterCheckItem, error) {
	var items []model.BuildMasterCheckItem
	err := r.db.Table("checkList_checkitem").Order("id ASC").Find(&items).Error
	return items, err
}

func (r *buildMasterRepository) CreateDetail(detail *model.BuildMasterItemDetail) error {
	return r.db.Create(detail).Error
}

func (r *buildMasterRepository) GetDetailByID(id uint) (*model.BuildMasterItemDetail, error) {
	var detail model.BuildMasterItemDetail
	err := r.db.First(&detail, id).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (r *buildMasterRepository) UpdateDetail(detail *model.BuildMasterItemDetail) error {
	return r.db.Save(detail).Error
}

func (r *buildMasterRepository) DeleteDetail(id uint) error {
	return r.db.Delete(&model.BuildMasterItemDetail{}, id).Error
}

func (r *buildMasterRepository) ListDetailsByItemID(itemID uint) ([]model.BuildMasterItemDetail, error) {
	var details []model.BuildMasterItemDetail
	err := r.db.Where("item_id = ?", itemID).Order("order_num ASC, created_at ASC").Find(&details).Error
	return details, err
}

func (r *buildMasterRepository) ListDetailsByListID(listID string) ([]model.BuildMasterItemDetail, error) {
	var details []model.BuildMasterItemDetail
	err := r.db.Where("list_id = ?", listID).Order("order_num ASC, created_at ASC").Find(&details).Error
	return details, err
}

func (r *buildMasterRepository) UpdateDetailsStatusByItemID(itemID uint, status int) error {
	return r.db.Model(&model.BuildMasterItemDetail{}).Where("item_id = ?", itemID).Update("status", status).Error
}

func (r *buildMasterRepository) CreateApproval(approval *model.BuildMasterApproval) error {
	return r.db.Create(approval).Error
}

func (r *buildMasterRepository) UpdateApproval(approval *model.BuildMasterApproval) error {
	return r.db.Save(approval).Error
}

func (r *buildMasterRepository) ListApprovalsByListID(listID string) ([]model.BuildMasterApproval, error) {
	var approvals []model.BuildMasterApproval
	err := r.db.Where("list_id = ?", listID).Order("order_num ASC, created_at ASC").Find(&approvals).Error
	return approvals, err
}

func (r *buildMasterRepository) FindByApprovalInstance(instanceCode string) (*model.BuildMasterList, error) {
	var list model.BuildMasterList
	err := r.db.Where("approval_instance = ?", instanceCode).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *buildMasterRepository) GetPendingApprovalsByListID(listID string) ([]model.BuildMasterApproval, error) {
	var approvals []model.BuildMasterApproval
	err := r.db.Where("list_id = ? AND result = ?", listID, model.BuildMasterApprovalPending).
		Order("order_num ASC, created_at ASC").Find(&approvals).Error
	return approvals, err
}
