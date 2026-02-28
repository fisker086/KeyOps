package auth

import (
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
)

type HostService struct {
	repo *repository.HostRepository
}

func NewHostService(repo *repository.HostRepository) *HostService {
	return &HostService{repo: repo}
}

func (s *HostService) CreateHost(host *model.Host) error {
	host.ID = uuid.New().String()
	host.CreatedAt = time.Now()
	host.UpdatedAt = time.Now()
	return s.repo.Create(host)
}

func (s *HostService) UpdateHost(id string, host *model.Host) error {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	// 更新字段
	if host.Name != "" {
		existing.Name = host.Name
	}
	if host.IP != "" {
		existing.IP = host.IP
	}
	if host.Port != 0 {
		existing.Port = host.Port
	}
	if host.Status != "" {
		existing.Status = host.Status
	}
	if host.OS != "" {
		existing.OS = host.OS
	}
	if host.CPU != "" {
		existing.CPU = host.CPU
	}
	if host.Memory != "" {
		existing.Memory = host.Memory
	}
	if host.Tags != "" {
		existing.Tags = host.Tags
	}
	if host.Description != "" {
		existing.Description = host.Description
	}
	if host.DeviceType != "" {
		existing.DeviceType = host.DeviceType
	}
	if host.ConnectionMode != "" {
		existing.ConnectionMode = host.ConnectionMode
	}
	if host.ProxyID != "" {
		existing.ProxyID = host.ProxyID
	}
	if host.NetworkZone != "" {
		existing.NetworkZone = host.NetworkZone
	}

	existing.UpdatedAt = time.Now()
	return s.repo.Update(existing)
}

func (s *HostService) DeleteHost(id string) error {
	return s.repo.Delete(id)
}

func (s *HostService) GetHost(id string) (*model.Host, error) {
	return s.repo.FindByID(id)
}

func (s *HostService) ListHosts(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 10000 {
		pageSize = 10
	}
	return s.repo.FindAll(page, pageSize, search, tags)
}

// ListHostsByUser 获取用户登录过的主机列表
func (s *HostService) ListHostsByUser(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 10000 {
		pageSize = 10
	}
	return s.repo.FindByUser(page, pageSize, search, tags, userID)
}

// TestConnection 测试主机连接（需要指定系统用户）
// 注意：由于认证信息已从Host移至SystemUser，必须提供systemUserID
func (s *HostService) TestConnection(id string) (bool, string) {
	return false, "请使用 TestConnectionWithSystemUser 方法，并提供系统用户ID进行连接测试"
}

// TestConnectionWithSystemUser 使用指定的系统用户测试主机连接
func (s *HostService) TestConnectionWithSystemUser(hostID string, systemUserID string) (bool, string) {
	_, err := s.repo.FindByID(hostID)
	if err != nil {
		return false, "主机不存在"
	}

	// 获取系统用户信息（需要注入SystemUserRepository）
	// 这里返回提示信息，实际实现需要在handler层完成
	return false, "测试连接功能需要在handler层实现，请使用完整的系统用户认证信息"
}

func (s *HostService) GetDashboardStats() (*model.DashboardStats, error) {
	total, online, offline, err := s.repo.CountByStatus()
	if err != nil {
		return nil, err
	}

	// 这里可以添加获取最近登录数的逻辑
	// 暂时返回 0
	return &model.DashboardStats{
		TotalHosts:   int(total),
		OnlineHosts:  int(online),
		OfflineHosts: int(offline),
		RecentLogins: 0,
	}, nil
}

// GetUserDashboardStats 获取用户自己的仪表盘统计（只统计该用户登录过的主机）
func (s *HostService) GetUserDashboardStats(userID string) (*model.DashboardStats, error) {
	total, online, offline, err := s.repo.CountByStatusForUser(userID)
	if err != nil {
		return nil, err
	}

	return &model.DashboardStats{
		TotalHosts:   int(total),
		OnlineHosts:  int(online),
		OfflineHosts: int(offline),
		RecentLogins: 0,
	}, nil
}

func (s *HostService) RecordLogin(id string) error {
	if err := s.repo.IncrementLoginCount(id); err != nil {
		return err
	}
	return s.repo.UpdateLastLoginTime(id)
}

// GetHostsWithUserLoginCount 获取主机列表，附带指定用户的登录统计
func (s *HostService) GetHostsWithUserLoginCount(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 10000 {
		pageSize = 10
	}
	return s.repo.GetHostsWithUserLoginCount(page, pageSize, search, tags, userID)
}

// GetUserFrequentHosts 获取用户最常用的主机
func (s *HostService) GetUserFrequentHosts(userID string, limit int) ([]model.Host, error) {
	if limit < 1 {
		limit = 5
	}
	return s.repo.GetUserFrequentHosts(userID, limit)
}

// ListHostsByPermissions 获取用户有权限访问的主机列表（基于权限分配）
func (s *HostService) ListHostsByPermissions(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 10000 {
		pageSize = 10
	}
	return s.repo.FindByUserPermissions(page, pageSize, search, tags, userID)
}

// CheckIPDuplicate 检查IP地址是否重复（已废弃，保留用于向后兼容）
// excludeID: 更新时排除自己的ID（创建时传空字符串）
func (s *HostService) CheckIPDuplicate(ip string, excludeID string) error {
	existingHost, err := s.repo.FindByIP(ip)
	if err != nil {
		// 如果找不到记录（gorm.ErrRecordNotFound），说明IP不重复
		return nil
	}

	// 如果找到了记录，检查是否是要排除的主机（更新场景）
	if existingHost != nil && existingHost.ID != excludeID {
		return fmt.Errorf("IP地址 %s 已被主机 '%s' 使用，请使用其他IP地址", ip, existingHost.Name)
	}

	return nil
}

// CheckIPAndPortDuplicate 检查IP地址和端口组合是否重复
// excludeID: 更新时排除自己的ID（创建时传空字符串）
func (s *HostService) CheckIPAndPortDuplicate(ip string, port int, excludeID string) error {
	existingHost, err := s.repo.FindByIPAndPort(ip, port)
	if err != nil {
		// 如果找不到记录（gorm.ErrRecordNotFound），说明IP+端口组合不重复
		return nil
	}

	// 如果找到了记录，检查是否是要排除的主机（更新场景）
	if existingHost != nil && existingHost.ID != excludeID {
		return fmt.Errorf("IP地址 %s 和端口 %d 的组合已被主机 '%s' 使用，请使用其他IP地址或端口", ip, port, existingHost.Name)
	}

	return nil
}
