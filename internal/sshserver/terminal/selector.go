package terminal

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/sshserver/types"
)

// HostSelector 主机选择器
type HostSelector struct {
	hostRepo       *repository.HostRepository
	groupRepo      *repository.HostGroupRepository
	userRepo       *repository.UserRepository
	systemUserRepo *repository.SystemUserRepository // 新增：系统用户仓库（用于新权限架构）
}

// NewHostSelector 创建主机选择器
func NewHostSelector(hostRepo *repository.HostRepository) types.HostSelector {
	return &HostSelector{
		hostRepo:  hostRepo,
		groupRepo: nil, // 兼容性：可选
		userRepo:  nil,
	}
}

// NewHostSelectorWithGroup 创建带分组支持的主机选择器
func NewHostSelectorWithGroup(hostRepo *repository.HostRepository, groupRepo *repository.HostGroupRepository) types.HostSelector {
	return &HostSelector{
		hostRepo:  hostRepo,
		groupRepo: groupRepo,
		userRepo:  nil,
	}
}

// NewHostSelectorWithPermissions 创建带权限控制的主机选择器
func NewHostSelectorWithPermissions(hostRepo *repository.HostRepository, groupRepo *repository.HostGroupRepository, userRepo *repository.UserRepository) types.HostSelector {
	return &HostSelector{
		hostRepo:       hostRepo,
		groupRepo:      groupRepo,
		userRepo:       userRepo,
		systemUserRepo: nil, // 旧接口兼容性
	}
}

// NewHostSelectorV2 创建使用新权限架构的主机选择器
func NewHostSelectorV2(hostRepo *repository.HostRepository, groupRepo *repository.HostGroupRepository, userRepo *repository.UserRepository, systemUserRepo *repository.SystemUserRepository) types.HostSelector {
	return &HostSelector{
		hostRepo:       hostRepo,
		groupRepo:      groupRepo,
		userRepo:       userRepo,
		systemUserRepo: systemUserRepo,
	}
}

// ListAvailableHosts 列出可用主机（根据用户权限过滤）
func (s *HostSelector) ListAvailableHosts(userID string) ([]types.HostInfo, error) {
	logger.Infof("[HostSelector] Listing available hosts for user: %s", userID)

	// 检查是否为管理员
	var isAdmin bool
	if s.userRepo != nil {
		user, err := s.userRepo.FindUserByID(userID)
		if err == nil && user.Role == "admin" {
			isAdmin = true
		}
	}

	// 管理员可以看到所有主机
	if isAdmin {
		hosts, _, err := s.hostRepo.FindAll(1, 1000, "", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list hosts: %w", err)
		}

		hostInfos := make([]types.HostInfo, 0, len(hosts))
		for _, host := range hosts {
			hostInfo := s.convertToHostInfo(&host)
			hostInfos = append(hostInfos, hostInfo)
		}

		logger.Infof("[HostSelector] Admin user, found %d hosts", len(hostInfos))
		return hostInfos, nil
	}

	// 普通用户：使用新权限架构获取有权限的主机
	// 新架构：User → UserGroup → PermissionRule → (SystemUsers + HostGroups) → Hosts

	accessibleHostIDs := make(map[string]bool)

	// 优先使用新权限架构（如果 systemUserRepo 可用）
	if s.systemUserRepo != nil && s.groupRepo != nil {
		logger.Infof("[HostSelector] Using NEW permission architecture for user %s", userID)

		// 通过新权限架构获取主机：查询所有可用系统用户对应的主机
		// 步骤：
		// 1. 获取所有主机
		// 2. 对每个主机，检查用户是否有权限（通过任意一个系统用户）

		allHosts, _, err := s.hostRepo.FindAll(1, 10000, "", nil)
		if err != nil {
			logger.Infof("[HostSelector] Failed to fetch hosts: %v", err)
		} else {
			for _, host := range allHosts {
				// 获取用户对该主机可用的系统用户
				systemUsers, err := s.systemUserRepo.GetAvailableSystemUsersForUser(userID, host.ID)
				if err == nil && len(systemUsers) > 0 {
					// 有可用的系统用户，说明用户有权限访问此主机
					accessibleHostIDs[host.ID] = true
				}
			}
		}

		logger.Infof("[HostSelector] NEW architecture: User %s can access %d hosts", userID, len(accessibleHostIDs))

	} else {
		// 降级使用旧权限架构（兼容性）
		logger.Infof("[HostSelector] Falling back to OLD permission architecture for user %s", userID)

		// 获取用户分组权限对应的主机（旧方式）
		if s.userRepo != nil && s.groupRepo != nil {
			roleIDs, err := s.userRepo.GetUserRoles(userID)
			if err == nil && len(roleIDs) > 0 {
				for _, roleID := range roleIDs {
					hosts, _, err := s.groupRepo.GetHostsByGroupIDWithPagination(roleID, 1, 1000)
					if err == nil {
						for _, host := range hosts {
							accessibleHostIDs[host.ID] = true
						}
					}
				}
			}
		}

		// 获取单独授权的主机（旧方式）
		if s.userRepo != nil {
			hostIDs, err := s.userRepo.GetUserHosts(userID)
			if err == nil {
				for _, hostID := range hostIDs {
					accessibleHostIDs[hostID] = true
				}
			}
		}

		logger.Infof("[HostSelector] OLD architecture: User %s can access %d hosts", userID, len(accessibleHostIDs))
	}

	// 根据主机ID列表查询主机详情
	hostInfos := make([]types.HostInfo, 0)
	for hostID := range accessibleHostIDs {
		host, err := s.hostRepo.FindByID(hostID)
		if err == nil {
			hostInfo := s.convertToHostInfo(host)
			hostInfos = append(hostInfos, hostInfo)
		}
	}

	logger.Infof("[HostSelector] User %s can access %d hosts (group+individual)", userID, len(hostInfos))
	return hostInfos, nil
}

// GetHostInfo 获取主机信息
func (s *HostSelector) GetHostInfo(hostID string) (*types.HostInfo, error) {
	logger.Infof("[HostSelector] Getting host info for: %s", hostID)

	host, err := s.hostRepo.FindByID(hostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	hostInfo := s.convertToHostInfo(host)
	return &hostInfo, nil
}

// convertToHostInfo 转换为HostInfo
func (s *HostSelector) convertToHostInfo(host *model.Host) types.HostInfo {
	return types.HostInfo{
		ID:         host.ID,
		Name:       host.Name,
		IP:         host.IP,
		Port:       host.Port,
		Username:   "",         // TODO: 从 SystemUser 获取
		Password:   "",         // TODO: 从 SystemUser 获取
		Tags:       []string{}, // TODO: 添加标签支持
		DeviceType: host.DeviceType,
		Status:     host.Status, // 主机状态
	}
}

// ============================================================================
// 分组相关方法
// ============================================================================

// ListGroups 获取所有分组（带统计信息）
// 注意：此方法已废弃，建议使用 ListGroupsForUser
func (s *HostSelector) ListGroups() ([]types.HostGroupInfo, error) {
	if s.groupRepo == nil {
		return nil, fmt.Errorf("group repository not available")
	}

	groups, err := s.groupRepo.FindAllWithStats()
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// 转换为 HostGroupInfo
	groupInfos := make([]types.HostGroupInfo, 0, len(groups))
	for _, group := range groups {
		groupInfos = append(groupInfos, types.HostGroupInfo{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			HostCount:   group.HostCount,
			OnlineCount: group.OnlineCount,
		})
	}

	return groupInfos, nil
}

// ListGroupsForUser 根据用户权限获取分组列表
func (s *HostSelector) ListGroupsForUser(userID string) ([]types.HostGroupInfo, error) {
	if s.groupRepo == nil {
		return nil, fmt.Errorf("group repository not available")
	}

	// 获取用户信息以检查是否为管理员
	var isAdmin bool
	if s.userRepo != nil {
		user, err := s.userRepo.FindUserByID(userID)
		if err == nil && user.Role == "admin" {
			isAdmin = true
			logger.Infof("[HostSelector] User %s is admin, showing all groups", userID)
		}
	}

	// 管理员可以看到所有分组
	if isAdmin {
		return s.ListGroups()
	}

	// 普通用户只能看到有权限的分组
	if s.userRepo == nil {
		// 没有用户仓库，降级为显示所有分组（兼容性）
		logger.Infof("[HostSelector] UserRepo not available, showing all groups")
		return s.ListGroups()
	}

	// 获取用户有权限访问的主机组ID列表（通过授权规则）
	hostGroupIDs, err := s.userRepo.GetUserHostGroupIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user host groups: %w", err)
	}

	if len(hostGroupIDs) == 0 {
		logger.Infof("[HostSelector] User %s has no host group permissions", userID)
		return []types.HostGroupInfo{}, nil
	}

	logger.Infof("[HostSelector] User %s has access to %d host groups: %v", userID, len(hostGroupIDs), hostGroupIDs)

	// 获取所有分组的统计信息
	allGroups, err := s.groupRepo.FindAllWithStats()
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// 过滤出用户有权限的主机组
	groupInfos := make([]types.HostGroupInfo, 0)
	for _, group := range allGroups {
		// 检查该主机组是否在用户权限列表中
		hasPermission := false
		for _, gid := range hostGroupIDs {
			if group.ID == gid {
				hasPermission = true
				break
			}
		}

		if hasPermission {
			groupInfos = append(groupInfos, types.HostGroupInfo{
				ID:          group.ID,
				Name:        group.Name,
				Description: group.Description,
				HostCount:   group.HostCount,
				OnlineCount: group.OnlineCount,
			})
		}
	}

	logger.Infof("[HostSelector] User %s can access %d host groups", userID, len(groupInfos))
	return groupInfos, nil
}

// ListHostsByGroup 列出指定分组的主机（支持分页）
func (s *HostSelector) ListHostsByGroup(groupID string, page, pageSize int) ([]types.HostInfo, int, error) {
	if s.groupRepo == nil {
		return nil, 0, fmt.Errorf("group repository not available")
	}

	hosts, total, err := s.groupRepo.GetHostsByGroupIDWithPagination(groupID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list hosts in group: %w", err)
	}

	// 转换为 HostInfo
	hostInfos := make([]types.HostInfo, 0, len(hosts))
	for _, host := range hosts {
		hostInfo := s.convertToHostInfo(&host)
		hostInfos = append(hostInfos, hostInfo)
	}

	return hostInfos, int(total), nil
}

// SearchHosts 搜索主机（按名称或IP）
func (s *HostSelector) SearchHosts(keyword string) ([]types.HostInfo, error) {
	if keyword == "" {
		return s.ListAvailableHosts("")
	}

	// 使用 hostRepo 的查询功能搜索
	hosts, _, err := s.hostRepo.FindAll(1, 100, keyword, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search hosts: %w", err)
	}

	// 转换为 HostInfo
	hostInfos := make([]types.HostInfo, 0, len(hosts))
	for _, host := range hosts {
		hostInfo := s.convertToHostInfo(&host)
		hostInfos = append(hostInfos, hostInfo)
	}

	return hostInfos, nil
}
