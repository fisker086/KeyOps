package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type HostRepository struct {
	db *gorm.DB
}

func NewHostRepository(db *gorm.DB) *HostRepository {
	return &HostRepository{db: db}
}

func (r *HostRepository) Create(host *model.Host) error {
	return r.db.Create(host).Error
}

func (r *HostRepository) Update(host *model.Host) error {
	return r.db.Save(host).Error
}

func (r *HostRepository) Delete(id string) error {
	return r.db.Delete(&model.Host{}, "id = ?", id).Error
}

func (r *HostRepository) FindByID(id string) (*model.Host, error) {
	var host model.Host
	err := r.db.Where("id = ?", id).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (r *HostRepository) FindByIP(ip string) (*model.Host, error) {
	var host model.Host
	err := r.db.Where("ip = ?", ip).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (r *HostRepository) FindByIPAndPort(ip string, port int) (*model.Host, error) {
	var host model.Host
	err := r.db.Where("ip = ? AND port = ?", ip, port).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (r *HostRepository) FindAll(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	var hosts []model.Host
	var total int64

	query := r.db.Model(&model.Host{})

	if search != "" {
		query = query.Where("name LIKE ? OR ip LIKE ? OR os LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&hosts).Error

	return hosts, total, err
}

// FindByUser 查询用户登录过的主机列表
func (r *HostRepository) FindByUser(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	var hosts []model.Host
	var total int64

	// 先查询该用户登录过的主机ID列表
	var hostIDs []string
	if err := r.db.Model(&model.LoginRecord{}).
		Select("DISTINCT host_id").
		Where("user_id = ?", userID).
		Pluck("host_id", &hostIDs).Error; err != nil {
		return nil, 0, err
	}

	if len(hostIDs) == 0 {
		return []model.Host{}, 0, nil
	}

	// 查询这些主机的详细信息
	query := r.db.Model(&model.Host{}).Where("id IN ?", hostIDs)

	if search != "" {
		query = query.Where("name LIKE ? OR ip LIKE ? OR os LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&hosts).Error

	return hosts, total, err
}

func (r *HostRepository) CountByStatus() (total, online, offline int64, err error) {
	if err = r.db.Model(&model.Host{}).Count(&total).Error; err != nil {
		return
	}
	if err = r.db.Model(&model.Host{}).Where("status = ?", "online").Count(&online).Error; err != nil {
		return
	}
	if err = r.db.Model(&model.Host{}).Where("status = ?", "offline").Count(&offline).Error; err != nil {
		return
	}
	return
}

// CountByStatusForUser 统计用户有权限访问的主机状态（使用新权限架构）
func (r *HostRepository) CountByStatusForUser(userID string) (total, online, offline int64, err error) {
	// 使用新权限架构获取用户可访问的主机ID列表
	hostIDs, err := r.GetAccessibleHostIDsForUser(userID)
	if err != nil {
		return 0, 0, 0, err
	}

	if len(hostIDs) == 0 {
		return 0, 0, 0, nil
	}

	// 统计这些主机的状态
	if err = r.db.Model(&model.Host{}).Where("id IN ?", hostIDs).Count(&total).Error; err != nil {
		return
	}
	if err = r.db.Model(&model.Host{}).Where("id IN ? AND status = ?", hostIDs, "online").Count(&online).Error; err != nil {
		return
	}
	if err = r.db.Model(&model.Host{}).Where("id IN ? AND status = ?", hostIDs, "offline").Count(&offline).Error; err != nil {
		return
	}
	return
}

func (r *HostRepository) IncrementLoginCount(id string) error {
	return r.db.Model(&model.Host{}).Where("id = ?", id).
		UpdateColumn("login_count", gorm.Expr("login_count + 1")).Error
}

func (r *HostRepository) UpdateLastLoginTime(id string) error {
	return r.db.Model(&model.Host{}).Where("id = ?", id).
		Update("last_login_time", gorm.Expr("NOW()")).Error
}

// GetHostsWithUserLoginCount 获取主机列表，并附带指定用户的登录次数和最后登录时间
func (r *HostRepository) GetHostsWithUserLoginCount(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	var hosts []model.Host
	var total int64

	// 基础查询
	query := r.db.Model(&model.Host{})

	if search != "" {
		query = query.Where("name LIKE ? OR ip LIKE ? OR os LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&hosts).Error; err != nil {
		return nil, 0, err
	}

	// 为每个主机查询该用户的登录统计
	for i := range hosts {
		var loginCount int64
		var lastLoginTime *string

		// 统计该用户对这个主机的登录次数（只统计成功的登录，排除失败的连接尝试）
		r.db.Model(&model.LoginRecord{}).
			Where("user_id = ? AND host_id = ? AND status IN (?, ?)", userID, hosts[i].ID, "active", "completed").
			Count(&loginCount)
		hosts[i].LoginCount = int(loginCount)

		// 获取该用户对这个主机的最后登录时间（只统计成功的登录）
		var record model.LoginRecord
		if err := r.db.Model(&model.LoginRecord{}).
			Where("user_id = ? AND host_id = ? AND status IN (?, ?)", userID, hosts[i].ID, "active", "completed").
			Order("login_time DESC").
			Limit(1).
			First(&record).Error; err == nil {
			timeStr := record.LoginTime.Format("2006-01-02 15:04:05")
			lastLoginTime = &timeStr
		}

		if lastLoginTime != nil {
			hosts[i].LastLoginTime = &record.LoginTime
		}
	}

	return hosts, total, nil
}

// GetUserFrequentHosts 获取用户最常用的主机（按登录次数排序，仅限用户有权限访问的主机）
func (r *HostRepository) GetUserFrequentHosts(userID string, limit int) ([]model.Host, error) {
	// 首先获取用户有权限访问的主机ID列表（使用新权限架构）
	accessibleHostIDs, err := r.GetAccessibleHostIDsForUser(userID)
	if err != nil {
		return nil, err
	}

	if len(accessibleHostIDs) == 0 {
		return []model.Host{}, nil
	}

	// 先统计用户对每个主机的登录次数和最后登录时间（仅限有权限的主机）
	type HostLoginStat struct {
		HostID        string    `gorm:"column:host_id"`
		LoginCount    int64     `gorm:"column:login_count"`
		LastLoginTime time.Time `gorm:"column:last_login_time"`
	}

	var stats []HostLoginStat
	err = r.db.Model(&model.LoginRecord{}).
		Select("host_id, COUNT(*) as login_count, MAX(login_time) as last_login_time").
		Where("user_id = ? AND host_id IN ? AND status IN (?, ?)", userID, accessibleHostIDs, "active", "completed").
		Group("host_id").
		Order("login_count DESC").
		Limit(limit).
		Scan(&stats).Error

	if err != nil {
		return nil, err
	}

	// 如果用户还没有登录记录，返回用户有权限的前N个主机
	if len(stats) == 0 {
		var hosts []model.Host
		if err := r.db.Where("id IN ?", accessibleHostIDs).
			Order("created_at DESC").
			Limit(limit).
			Find(&hosts).Error; err != nil {
			return nil, err
		}
		return hosts, nil
	}

	// 获取这些主机的详细信息
	hostIDs := make([]string, len(stats))
	statMap := make(map[string]HostLoginStat)
	for i, stat := range stats {
		hostIDs[i] = stat.HostID
		statMap[stat.HostID] = stat
	}

	var hosts []model.Host
	if err := r.db.Where("id IN ?", hostIDs).Find(&hosts).Error; err != nil {
		return nil, err
	}

	// 设置登录次数和最后登录时间，并按登录次数排序
	for i := range hosts {
		if stat, ok := statMap[hosts[i].ID]; ok {
			hosts[i].LoginCount = int(stat.LoginCount)
			hosts[i].LastLoginTime = &stat.LastLoginTime
		}
	}

	// 按登录次数降序排序（保持与SQL查询一致的顺序）
	for i := 0; i < len(hosts)-1; i++ {
		for j := i + 1; j < len(hosts); j++ {
			if hosts[i].LoginCount < hosts[j].LoginCount {
				hosts[i], hosts[j] = hosts[j], hosts[i]
			}
		}
	}

	return hosts, nil
}

// UpdateStatus 更新主机状态
func (r *HostRepository) UpdateStatus(id string, status string) error {
	return r.db.Model(&model.Host{}).Where("id = ?", id).
		Update("status", status).Error
}

// FindAllWithPagination 获取所有主机（支持分页和搜索）
func (r *HostRepository) FindAllWithPagination(page, pageSize int, search string, tags []string) ([]model.Host, int64, error) {
	var hosts []model.Host
	var total int64

	query := r.db.Model(&model.Host{})

	// 搜索过滤
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name LIKE ? OR ip LIKE ? OR description LIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// 标签过滤
	if len(tags) > 0 {
		for _, tag := range tags {
			// 根据数据库类型使用不同的 JSON 查询语法
			if r.db.Dialector.Name() == "postgres" {
				// PostgreSQL: 使用 JSONB @> 操作符
				query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
			} else {
				// MySQL: 使用 JSON_CONTAINS
				query = query.Where("JSON_CONTAINS(tags, ?)", fmt.Sprintf(`"%s"`, tag))
			}
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&hosts).Error

	return hosts, total, err
}

// FindByUserPermissions 查询用户有权限访问的主机（使用新权限架构）
func (r *HostRepository) FindByUserPermissions(page, pageSize int, search string, tags []string, userID string) ([]model.Host, int64, error) {
	var hosts []model.Host
	var total int64

	// 使用新权限架构：通过权限规则获取主机
	hostIDs, err := r.GetAccessibleHostIDsForUser(userID)
	if err != nil {
		return nil, 0, err
	}

	// 如果用户没有任何权限，返回空列表
	if len(hostIDs) == 0 {
		return []model.Host{}, 0, nil
	}

	// 查询这些主机的详细信息
	query := r.db.Model(&model.Host{}).Where("id IN ?", hostIDs)

	if search != "" {
		query = query.Where("name LIKE ? OR ip LIKE ? OR os LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err = query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&hosts).Error

	return hosts, total, err
}

// GetAccessibleHostIDsForUser 获取用户可访问的所有主机ID（新权限架构）
func (r *HostRepository) GetAccessibleHostIDsForUser(userID string) ([]string, error) {
	now := time.Now()
	hostIDMap := make(map[string]bool)

	// 新权限架构查询：
	// 1. 用户 → role_members → roles
	// 2. roles → permission_rules（启用且在有效期内）
	// 3. permission_rules → permission_rule_host_groups → host_group_members → hosts
	// 4. permission_rules → host_ids（JSON字段）

	// 方法1：通过主机组获取主机
	var hostGroupHostIDs []string
	err := r.db.Table("host_group_members").
		Select("DISTINCT host_group_members.host_id").
		Joins("INNER JOIN permission_rule_host_groups ON permission_rule_host_groups.host_group_id = host_group_members.group_id").
		Joins("INNER JOIN permission_rules ON permission_rules.id = permission_rule_host_groups.permission_rule_id").
		Joins("INNER JOIN roles ON roles.id = permission_rules.role_id").
		Joins("INNER JOIN role_members ON role_members.role_id = roles.id").
		Where("role_members.user_id = ?", userID).
		Where("permission_rules.enabled = ?", true).
		Where("roles.status = ?", "active").
		Where("(permission_rules.valid_from IS NULL OR permission_rules.valid_from <= ?)", now).
		Where("(permission_rules.valid_to IS NULL OR permission_rules.valid_to >= ?)", now).
		Pluck("host_group_members.host_id", &hostGroupHostIDs).Error

	if err != nil {
		return nil, err
	}

	for _, id := range hostGroupHostIDs {
		hostIDMap[id] = true
	}

	// 方法2：通过 host_ids JSON字段获取主机（直接指定的主机）
	var rules []model.PermissionRule
	err = r.db.Table("permission_rules").
		Joins("INNER JOIN roles ON roles.id = permission_rules.role_id").
		Joins("INNER JOIN role_members ON role_members.role_id = roles.id").
		Where("role_members.user_id = ?", userID).
		Where("permission_rules.enabled = ?", true).
		Where("roles.status = ?", "active").
		Where("(permission_rules.valid_from IS NULL OR permission_rules.valid_from <= ?)", now).
		Where("(permission_rules.valid_to IS NULL OR permission_rules.valid_to >= ?)", now).
		Where("permission_rules.host_ids IS NOT NULL AND permission_rules.host_ids != ''").
		Find(&rules).Error

	if err == nil {
		for _, rule := range rules {
			// 解析 host_ids JSON字段
			if rule.HostIDs != "" {
				var hostIDs []string
				// 根据数据库类型使用不同的 JSON 提取语法
				var jsonQuery string
				if r.db.Dialector.Name() == "postgres" {
					// PostgreSQL: 使用 jsonb_array_elements_text
					jsonQuery = "SELECT jsonb_array_elements_text(?::jsonb)::text"
				} else {
					// MySQL: 使用 JSON_EXTRACT
					jsonQuery = "SELECT JSON_EXTRACT(?, '$[*]')"
				}
				if err := r.db.Raw(jsonQuery, rule.HostIDs).Scan(&hostIDs).Error; err == nil {
					for _, id := range hostIDs {
						// 清理引号
						cleanID := strings.Trim(id, `"`)
						if cleanID != "" {
							hostIDMap[cleanID] = true
						}
					}
				}
			}
		}
	}

	// 转换为切片
	var hostIDs []string
	for id := range hostIDMap {
		hostIDs = append(hostIDs, id)
	}

	return hostIDs, nil
}
