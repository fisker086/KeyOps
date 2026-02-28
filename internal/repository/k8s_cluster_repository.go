package repository

import (
	"errors"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type K8sClusterRepository struct {
	db *gorm.DB
}

func NewK8sClusterRepository(db *gorm.DB) *K8sClusterRepository {
	return &K8sClusterRepository{db: db}
}

// Create 创建集群
func (r *K8sClusterRepository) Create(cluster *model.K8sCluster) error {
	return r.db.Create(cluster).Error
}

// Update 更新集群
func (r *K8sClusterRepository) Update(cluster *model.K8sCluster) error {
	return r.db.Model(&model.K8sCluster{}).
		Where("id = ?", cluster.ID).
		Omit("created_at").
		Updates(cluster).Error
}

// Delete 删除集群
func (r *K8sClusterRepository) Delete(id string) error {
	return r.db.Delete(&model.K8sCluster{}, "id = ?", id).Error
}

// FindByID 根据ID查找集群
func (r *K8sClusterRepository) FindByID(id string) (*model.K8sCluster, error) {
	var cluster model.K8sCluster
	err := r.db.Where("id = ?", id).First(&cluster).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cluster, nil
}

// FindByName 根据名称查找集群
func (r *K8sClusterRepository) FindByName(name string) (*model.K8sCluster, error) {
	var cluster model.K8sCluster
	err := r.db.Where("name = ?", name).First(&cluster).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cluster, nil
}

// FindByAPIServer 根据API Server地址查找集群
func (r *K8sClusterRepository) FindByAPIServer(apiServer string) (*model.K8sCluster, error) {
	var cluster model.K8sCluster
	err := r.db.Where("api_server = ?", apiServer).First(&cluster).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cluster, nil
}

// FindAll 查找所有集群
func (r *K8sClusterRepository) FindAll() ([]model.K8sCluster, error) {
	var clusters []model.K8sCluster
	err := r.db.Where("status = ?", "active").Order("name ASC").Find(&clusters).Error
	return clusters, err
}

// FindByStatus 根据状态查找集群
func (r *K8sClusterRepository) FindByStatus(status string) ([]model.K8sCluster, error) {
	var clusters []model.K8sCluster
	err := r.db.Where("status = ?", status).Order("name ASC").Find(&clusters).Error
	return clusters, err
}

// FindByEnvironment 根据环境查找集群
func (r *K8sClusterRepository) FindByEnvironment(environment string) ([]model.K8sCluster, error) {
	var clusters []model.K8sCluster
	err := r.db.Where("environment = ? AND status = ?", environment, "active").Order("name ASC").Find(&clusters).Error
	return clusters, err
}
