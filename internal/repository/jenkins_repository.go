package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type JenkinsRepository struct {
	db *gorm.DB
}

func NewJenkinsRepository(db *gorm.DB) *JenkinsRepository {
	return &JenkinsRepository{db: db}
}

// Create 创建Jenkins服务器
func (r *JenkinsRepository) Create(server *model.JenkinsServer) error {
	return r.db.Create(server).Error
}

// GetByID 根据ID获取Jenkins服务器
func (r *JenkinsRepository) GetByID(id uint) (*model.JenkinsServer, error) {
	var server model.JenkinsServer
	err := r.db.Where("id = ?", id).First(&server).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// Update 更新Jenkins服务器
func (r *JenkinsRepository) Update(server *model.JenkinsServer) error {
	return r.db.Save(server).Error
}

// Delete 删除Jenkins服务器
func (r *JenkinsRepository) Delete(id uint) error {
	return r.db.Delete(&model.JenkinsServer{}, id).Error
}

// List 获取Jenkins服务器列表
func (r *JenkinsRepository) List(page, pageSize int) ([]model.JenkinsServer, int64, error) {
	var servers []model.JenkinsServer
	var total int64

	query := r.db.Model(&model.JenkinsServer{})

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&servers).Error; err != nil {
		return nil, 0, err
	}

	return servers, total, nil
}

// ListAll 获取所有启用的Jenkins服务器
func (r *JenkinsRepository) ListAll() ([]model.JenkinsServer, error) {
	var servers []model.JenkinsServer
	err := r.db.Where("enabled = ?", true).Order("created_at DESC").Find(&servers).Error
	return servers, err
}

