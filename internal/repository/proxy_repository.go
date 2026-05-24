package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ProxyRepository interface {
	FindByID(proxyID string) (*model.Proxy, error)
	FindProxyInfoByID(proxyID string) (*model.ProxyInfo, error)
	FindOnlineProxies() ([]model.ProxyInfo, error)
	FindOnlineProxiesByZone(zone string) ([]model.ProxyInfo, error)
	FindAll() ([]model.ProxyInfo, error)
	UpdateStatus(proxyID string, status string) error
	UpdateNetworkZone(proxyID string, zone string) error
}

type proxyRepository struct {
	db *gorm.DB
}

func NewProxyRepository(db *gorm.DB) ProxyRepository {
	return &proxyRepository{db: db}
}

// FindByID 根据ID查找代理
func (r *proxyRepository) FindByID(proxyID string) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.Where("proxy_id = ?", proxyID).First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

// FindByProxyID 根据ProxyID查找代理（查询proxy_registrations表）
func (r *proxyRepository) FindProxyInfoByID(proxyID string) (*model.ProxyInfo, error) {
	var proxy model.ProxyInfo
	err := r.db.Where("proxy_id = ?", proxyID).First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

// FindOnlineProxies 查找所有在线的代理
func (r *proxyRepository) FindOnlineProxies() ([]model.ProxyInfo, error) {
	var proxies []model.ProxyInfo
	err := r.db.Where("status = ?", "online").
		Order("last_heartbeat DESC").
		Find(&proxies).Error
	return proxies, err
}

// FindOnlineProxiesByZone 根据网络区域查找在线代理
func (r *proxyRepository) FindOnlineProxiesByZone(zone string) ([]model.ProxyInfo, error) {
	var proxies []model.ProxyInfo
	err := r.db.Where("status = ? AND network_zone = ?", "online", zone).
		Order("last_heartbeat DESC").
		Find(&proxies).Error
	return proxies, err
}

// FindAll 查找所有代理
func (r *proxyRepository) FindAll() ([]model.ProxyInfo, error) {
	var proxies []model.ProxyInfo
	err := r.db.Order("created_at DESC").Find(&proxies).Error
	return proxies, err
}

// UpdateStatus 更新代理状态
func (r *proxyRepository) UpdateStatus(proxyID string, status string) error {
	return r.db.Model(&model.ProxyInfo{}).
		Where("proxy_id = ?", proxyID).
		Update("status", status).Error
}

// UpdateNetworkZone 更新代理网络区域
func (r *proxyRepository) UpdateNetworkZone(proxyID string, zone string) error {
	return r.db.Model(&model.ProxyInfo{}).
		Where("proxy_id = ?", proxyID).
		Update("network_zone", zone).Error
}
