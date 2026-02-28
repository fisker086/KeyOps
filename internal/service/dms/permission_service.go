package dms

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/casbin"
	"github.com/fisker086/keyops/pkg/logger"
)

type PermissionService struct {
	permissionRepo *repository.DBPermissionRepository
	instanceRepo   *repository.DBInstanceRepository
}

func NewPermissionService(
	permissionRepo *repository.DBPermissionRepository,
	instanceRepo *repository.DBInstanceRepository,
) *PermissionService {
	return &PermissionService{
		permissionRepo: permissionRepo,
		instanceRepo:   instanceRepo,
	}
}

// CheckPermission 检查权限
// 权限层级：admin > write > read
// - admin 权限包含 write 和 read
// - write 权限包含 read
func (s *PermissionService) CheckPermission(
	userID string,
	instanceID uint,
	databaseName string,
	tableName string,
	action string, // read, write, admin
) (bool, error) {
	// 根据权限层级，确定需要检查的权限类型
	// read: 检查 read, write, admin
	// write: 检查 write, admin
	// admin: 只检查 admin
	actionsToCheck := []string{}
	switch action {
	case "read":
		actionsToCheck = []string{"read", "write", "admin"}
	case "write":
		actionsToCheck = []string{"write", "admin"}
	case "admin":
		actionsToCheck = []string{"admin"}
	default:
		actionsToCheck = []string{action}
	}

	// 1. 构建资源路径
	resourcePath := s.buildResourcePath(instanceID, databaseName, tableName)

	// 2. 检查用户直接权限（表级别）
	if tableName != "" {
		for _, checkAction := range actionsToCheck {
			if hasPermission, _ := casbin.Enforce(userID, resourcePath, checkAction); hasPermission {
				return true, nil
			}
		}
	}

	// 3. 检查数据库级别权限
	if databaseName != "" {
		dbPath := s.buildResourcePath(instanceID, databaseName, "")
		for _, checkAction := range actionsToCheck {
			if hasPermission, _ := casbin.Enforce(userID, dbPath, checkAction); hasPermission {
				return true, nil
			}
		}
		// 检查通配符权限
		wildcardPath := fmt.Sprintf("dms:instance:%d:db:*", instanceID)
		for _, checkAction := range actionsToCheck {
			if hasPermission, _ := casbin.Enforce(userID, wildcardPath, checkAction); hasPermission {
				return true, nil
			}
		}
	}

	// 4. 检查实例级别权限
	instancePath := s.buildResourcePath(instanceID, "", "")
	for _, checkAction := range actionsToCheck {
		if hasPermission, _ := casbin.Enforce(userID, instancePath, checkAction); hasPermission {
			return true, nil
		}
	}

	// 5. 检查角色权限
	roles, err := casbin.GetRolesForUser(userID)
	if err == nil {
		for _, role := range roles {
			// 检查表级别
			if tableName != "" {
				for _, checkAction := range actionsToCheck {
					if hasPermission, _ := casbin.Enforce(role, resourcePath, checkAction); hasPermission {
						return true, nil
					}
				}
			}
			// 检查数据库级别
			if databaseName != "" {
				dbPath := s.buildResourcePath(instanceID, databaseName, "")
				for _, checkAction := range actionsToCheck {
					if hasPermission, _ := casbin.Enforce(role, dbPath, checkAction); hasPermission {
						return true, nil
					}
				}
				wildcardPath := fmt.Sprintf("dms:instance:%d:db:*", instanceID)
				for _, checkAction := range actionsToCheck {
					if hasPermission, _ := casbin.Enforce(role, wildcardPath, checkAction); hasPermission {
						return true, nil
					}
				}
			}
			// 检查实例级别
			for _, checkAction := range actionsToCheck {
				if hasPermission, _ := casbin.Enforce(role, instancePath, checkAction); hasPermission {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// buildResourcePath 构建资源路径
func (s *PermissionService) buildResourcePath(instanceID uint, databaseName, tableName string) string {
	path := fmt.Sprintf("dms:instance:%d", instanceID)
	if databaseName != "" {
		path += fmt.Sprintf(":db:%s", databaseName)
		if tableName != "" {
			path += fmt.Sprintf(":table:%s", tableName)
		}
	}
	return path
}

// GrantPermission 分配权限
func (s *PermissionService) GrantPermission(req *GrantPermissionRequest) error {
	// 1. 构建资源路径
	resourcePath := s.buildResourcePath(req.InstanceID, req.DatabaseName, req.TableName)

	// 2. 检查精确的权限策略是否已存在（使用 GetFilteredPolicy 而不是 Enforce）
	// Enforce 会考虑权限层级（如实例级权限会覆盖数据库级权限），不适合检查精确策略是否存在
	policies, err := casbin.GetFilteredPolicy(0, req.UserID)
	if err == nil {
		// 检查是否有精确匹配的策略
		for _, policy := range policies {
			if len(policy) >= 3 && policy[0] == req.UserID && policy[1] == resourcePath && policy[2] == req.PermissionType {
				return errors.New("权限已存在")
			}
		}
	}

	// 3. 添加到 Casbin
	success, err := casbin.AddPolicy(req.UserID, resourcePath, req.PermissionType)
	if err != nil {
		return fmt.Errorf("添加权限失败: %w", err)
	}
	if !success {
		return errors.New("添加权限失败")
	}

	// 4. 保存元数据
	metadata := &model.DBPermissionMetadata{
		UserID:         req.UserID,
		InstanceID:     req.InstanceID,
		DatabaseName:   req.DatabaseName,
		Table:          req.TableName,
		PermissionType: req.PermissionType,
		GrantedBy:      req.GrantedBy,
		ExpiresAt:      req.ExpiresAt,
		Description:    req.Description,
	}
	if err := s.permissionRepo.Create(metadata); err != nil {
		// 回滚 Casbin 规则
		casbin.RemovePolicy(req.UserID, resourcePath, req.PermissionType)
		return fmt.Errorf("保存权限元数据失败: %w", err)
	}

	// 5. 重新加载策略（Watcher 会自动通知其他机器）
	if err := casbin.ReloadPolicy(); err != nil {
		logger.Errorf("重新加载策略失败: %v", err)
	}

	return nil
}

// BatchGrantPermissions 批量分配权限
func (s *PermissionService) BatchGrantPermissions(reqs []*GrantPermissionRequest) error {
	if len(reqs) == 0 {
		return errors.New("权限列表不能为空")
	}

	var rules [][]string
	var metadatas []*model.DBPermissionMetadata

	// 1. 构建所有权限的资源路径和规则
	for _, req := range reqs {
		resourcePath := s.buildResourcePath(req.InstanceID, req.DatabaseName, req.TableName)
		
		// 检查精确的权限策略是否已存在（使用 GetFilteredPolicy 而不是 Enforce）
		// Enforce 会考虑权限层级（如实例级权限会覆盖数据库级权限），不适合检查精确策略是否存在
		policies, err := casbin.GetFilteredPolicy(0, req.UserID)
		if err == nil {
			// 检查是否有精确匹配的策略
			exactMatch := false
			for _, policy := range policies {
				if len(policy) >= 3 && policy[0] == req.UserID && policy[1] == resourcePath && policy[2] == req.PermissionType {
					exactMatch = true
					break
				}
			}
			if exactMatch {
				continue // 跳过已存在的精确权限策略
			}
		}

		rules = append(rules, []string{req.UserID, resourcePath, req.PermissionType})
		
		metadatas = append(metadatas, &model.DBPermissionMetadata{
			UserID:         req.UserID,
			InstanceID:     req.InstanceID,
			DatabaseName:   req.DatabaseName,
			Table:          req.TableName,
			PermissionType: req.PermissionType,
			GrantedBy:      req.GrantedBy,
			ExpiresAt:      req.ExpiresAt,
			Description:    req.Description,
		})
	}

	if len(rules) == 0 {
		return errors.New("所有权限都已存在")
	}

	// 2. 批量添加到 Casbin
	success, err := casbin.AddPolicies(rules)
	if err != nil {
		return fmt.Errorf("批量添加权限失败: %w", err)
	}
	if !success {
		return errors.New("批量添加权限失败")
	}

	// 3. 批量保存元数据
	if len(metadatas) > 0 {
		if err := s.permissionRepo.BatchCreate(metadatas); err != nil {
			// 回滚 Casbin 规则
			for _, rule := range rules {
				casbin.RemovePolicy(rule[0], rule[1], rule[2])
			}
			return fmt.Errorf("保存权限元数据失败: %w", err)
		}
	}

	// 4. 重新加载策略（Watcher 会自动通知其他机器）
	if err := casbin.ReloadPolicy(); err != nil {
		logger.Errorf("重新加载策略失败: %v", err)
	}

	return nil
}

// UpdatePermission 更新权限（只更新元数据：过期时间、描述）
func (s *PermissionService) UpdatePermission(req *UpdatePermissionRequest) error {
	// 1. 构建资源路径
	resourcePath := s.buildResourcePath(req.InstanceID, req.DatabaseName, req.TableName)
	
	// 2. 检查权限是否存在
	existing, _ := casbin.Enforce(req.UserID, resourcePath, req.PermissionType)
	if !existing {
		return errors.New("权限不存在")
	}
	
	// 3. 更新元数据
	updates := make(map[string]interface{})
	if req.ExpiresAt != nil {
		updates["expires_at"] = req.ExpiresAt
	} else {
		// 如果传入的是空值，表示清除过期时间
		updates["expires_at"] = nil
	}
	if req.Description != "" {
		updates["description"] = req.Description
	} else {
		updates["description"] = ""
	}
	
	if err := s.permissionRepo.Update(req.UserID, req.InstanceID, req.DatabaseName, req.TableName, req.PermissionType, updates); err != nil {
		return fmt.Errorf("更新权限元数据失败: %w", err)
	}
	
	return nil
}

// UpdatePermissionResource 更新权限的资源路径（数据库、权限类型）
// 先添加新权限，成功后再删除旧权限，确保即使删除失败，新权限也已经存在
func (s *PermissionService) UpdatePermissionResource(req *UpdatePermissionResourceRequest) error {
	// 1. 构建新旧资源路径
	oldResourcePath := s.buildResourcePath(req.OldInstanceID, req.OldDatabaseName, req.OldTableName)
	newResourcePath := s.buildResourcePath(req.NewInstanceID, req.NewDatabaseName, req.NewTableName)
	
	// 2. 检查旧权限是否存在（检查精确的策略）
	policies, err := casbin.GetFilteredPolicy(0, req.UserID)
	if err != nil {
		return fmt.Errorf("获取权限策略失败: %w", err)
	}
	
	oldExists := false
	for _, policy := range policies {
		if len(policy) >= 3 && policy[0] == req.UserID && policy[1] == oldResourcePath && policy[2] == req.OldPermissionType {
			oldExists = true
			break
		}
	}
	if !oldExists {
		return errors.New("旧权限不存在")
	}
	
	// 3. 检查新权限是否已存在（检查精确的策略，避免重复）
	// 注意：如果新旧权限路径和类型完全相同，则认为是更新操作，不应该报错
	if oldResourcePath == newResourcePath && req.OldPermissionType == req.NewPermissionType {
		// 新旧权限完全相同，不需要更新
		return nil
	}
	
	newExists := false
	for _, policy := range policies {
		if len(policy) >= 3 && policy[0] == req.UserID && policy[1] == newResourcePath && policy[2] == req.NewPermissionType {
			newExists = true
			break
		}
	}
	if newExists {
		return errors.New("新权限已存在")
	}
	
	// 4. 先添加新权限（确保新权限存在）
	newRule := []string{req.UserID, newResourcePath, req.NewPermissionType}
	success, err := casbin.AddPolicy(newRule[0], newRule[1], newRule[2])
	if err != nil {
		return fmt.Errorf("添加新权限失败: %w", err)
	}
	if !success {
		return errors.New("添加新权限失败")
	}
	
	// 5. 保存新权限元数据
	newMetadata := &model.DBPermissionMetadata{
		UserID:         req.UserID,
		InstanceID:     req.NewInstanceID,
		DatabaseName:   req.NewDatabaseName,
		Table:          req.NewTableName,
		PermissionType: req.NewPermissionType,
		GrantedBy:      req.GrantedBy,
		ExpiresAt:      req.ExpiresAt,
		Description:    req.Description,
	}
	if err := s.permissionRepo.Create(newMetadata); err != nil {
		// 如果保存元数据失败，回滚 Casbin 策略
		casbin.RemovePolicy(newRule[0], newRule[1], newRule[2])
		return fmt.Errorf("保存新权限元数据失败: %w", err)
	}
	
	// 6. 删除旧权限（即使失败，新权限也已经存在）
	oldSuccess, err := casbin.RemovePolicy(req.UserID, oldResourcePath, req.OldPermissionType)
	if err != nil {
		logger.Warnf("删除旧权限失败: %v，但新权限已添加", err)
	} else if oldSuccess {
		// 删除旧权限元数据
		s.permissionRepo.Delete(req.UserID, req.OldInstanceID, req.OldDatabaseName, req.OldTableName, req.OldPermissionType)
	}
	
	// 7. 重新加载策略
	if err := casbin.ReloadPolicy(); err != nil {
		logger.Errorf("重新加载策略失败: %v", err)
	}
	
	return nil
}

// RevokePermission 回收权限
func (s *PermissionService) RevokePermission(req *RevokePermissionRequest) error {
	resourcePath := s.buildResourcePath(req.InstanceID, req.DatabaseName, req.TableName)

	// 从 Casbin 删除
	success, err := casbin.RemovePolicy(req.UserID, resourcePath, req.PermissionType)
	if err != nil {
		return fmt.Errorf("删除权限失败: %w", err)
	}
	if !success {
		return errors.New("权限不存在")
	}

	// 删除元数据
	s.permissionRepo.Delete(req.UserID, req.InstanceID, req.DatabaseName, req.TableName, req.PermissionType)

	// 重新加载策略（Watcher 会自动通知其他机器）
	if err := casbin.ReloadPolicy(); err != nil {
		logger.Errorf("重新加载策略失败: %v", err)
	}

	return nil
}

// GetUserPermissions 获取用户权限列表（兼容旧接口）
func (s *PermissionService) GetUserPermissions(userID string, filters map[string]interface{}) ([]*PermissionInfo, error) {
	permissions, _, err := s.GetUserPermissionsWithPagination(userID, filters, 1, 10000)
	return permissions, err
}

// GetUserPermissionsWithPagination 获取用户权限列表（支持分页）
func (s *PermissionService) GetUserPermissionsWithPagination(userID string, filters map[string]interface{}, page, pageSize int) ([]*PermissionInfo, int64, error) {
	var allPolicies [][]string
	
	if userID != "" {
		// 如果指定了用户ID，只获取该用户的权限
		policies, err := casbin.GetFilteredPolicy(0, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("获取权限策略失败: %w", err)
		}
		allPolicies = policies

		// 获取用户所属的角色
		roles, err := casbin.GetRolesForUser(userID)
		if err == nil {
			// 获取角色的权限
			for _, role := range roles {
				rolePolicies, err := casbin.GetFilteredPolicy(0, role)
				if err == nil {
					allPolicies = append(allPolicies, rolePolicies...)
				}
			}
		}
	} else {
		// 如果userID为空，获取所有用户的权限
		policies, err := casbin.GetFilteredPolicy(0)
		if err != nil {
			return nil, 0, fmt.Errorf("获取权限策略失败: %w", err)
		}
		allPolicies = policies
	}

	var allPermissions []*PermissionInfo
	seen := make(map[string]bool) // 去重

	for _, policy := range allPolicies {
		if len(policy) < 3 {
			continue
		}
		
		policyUserID := policy[0]
		resourcePath := policy[1]
		permissionType := policy[2]

		if strings.HasPrefix(resourcePath, "dms:instance:") {
			perm := s.parsePermission(resourcePath, permissionType)
			if perm != nil {
				perm.UserID = policyUserID

				// 应用过滤
				if filters != nil {
					if instanceID, ok := filters["instance_id"].(uint); ok && instanceID > 0 {
						if perm.InstanceID != instanceID {
							continue
						}
					}
				}

				// 去重（相同用户、资源路径和权限类型）
				key := fmt.Sprintf("%s:%d:%s:%s:%s", policyUserID, perm.InstanceID, perm.DatabaseName, perm.Table, permissionType)
				if seen[key] {
					continue
				}
				seen[key] = true

				// 获取元数据补充信息
				metadata, _ := s.permissionRepo.Get(policyUserID, perm.InstanceID, perm.DatabaseName, perm.Table, permissionType)
				if metadata != nil {
					perm.GrantedBy = metadata.GrantedBy
					perm.GrantedAt = metadata.GrantedAt
					perm.ExpiresAt = metadata.ExpiresAt
					perm.Description = metadata.Description
				}

				// 获取实例名称
				instance, _ := s.instanceRepo.GetByID(perm.InstanceID)
				if instance != nil {
					perm.InstanceName = instance.Name
				}

				allPermissions = append(allPermissions, perm)
			}
		}
	}

	// 分页处理
	total := int64(len(allPermissions))
	start := (page - 1) * pageSize
	end := start + pageSize
	
	if start >= len(allPermissions) {
		return []*PermissionInfo{}, total, nil
	}
	if end > len(allPermissions) {
		end = len(allPermissions)
	}

	return allPermissions[start:end], total, nil
}

// parsePermission 解析权限路径
func (s *PermissionService) parsePermission(resourcePath, permissionType string) *PermissionInfo {
	parts := strings.Split(resourcePath, ":")
	if len(parts) < 3 || parts[0] != "dms" || parts[1] != "instance" {
		return nil
	}

	var instanceID uint
	fmt.Sscanf(parts[2], "%d", &instanceID)

	perm := &PermissionInfo{
		InstanceID:     instanceID,
		PermissionType: permissionType,
	}

	if len(parts) >= 5 && parts[3] == "db" {
		perm.DatabaseName = parts[4]
		if len(parts) >= 7 && parts[5] == "table" {
			perm.Table = parts[6]
		}
	}

	return perm
}

type GrantPermissionRequest struct {
	UserID         string     `json:"userId" binding:"required"`
	InstanceID     uint       `json:"instanceId" binding:"required"`
	DatabaseName   string     `json:"databaseName"`
	TableName      string     `json:"tableName"`
	PermissionType string     `json:"permissionType" binding:"required,oneof=read write admin"`
	GrantedBy      string     `json:"grantedBy"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    string     `json:"description"`
}

type RevokePermissionRequest struct {
	UserID         string `json:"userId" binding:"required"`
	InstanceID     uint   `json:"instanceId" binding:"required"`
	DatabaseName   string `json:"databaseName"`
	TableName      string `json:"tableName"`
	PermissionType string `json:"permissionType" binding:"required,oneof=read write admin"`
}

type UpdatePermissionRequest struct {
	UserID         string     `json:"userId" binding:"required"`
	InstanceID     uint       `json:"instanceId" binding:"required"`
	DatabaseName   string     `json:"databaseName"`
	TableName      string     `json:"tableName"`
	PermissionType string     `json:"permissionType" binding:"required,oneof=read write admin"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    string     `json:"description"`
}

type UpdatePermissionResourceRequest struct {
	UserID            string     `json:"userId" binding:"required"`
	OldInstanceID     uint       `json:"oldInstanceId" binding:"required"`
	OldDatabaseName   string     `json:"oldDatabaseName"`
	OldTableName      string     `json:"oldTableName"`
	OldPermissionType string     `json:"oldPermissionType" binding:"required,oneof=read write admin"`
	NewInstanceID     uint       `json:"newInstanceId" binding:"required"`
	NewDatabaseName   string     `json:"newDatabaseName"`
	NewTableName      string     `json:"newTableName"`
	NewPermissionType string     `json:"newPermissionType" binding:"required,oneof=read write admin"`
	GrantedBy         string     `json:"grantedBy"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	Description       string     `json:"description"`
}

type BatchGrantPermissionRequest struct {
	UserID         string     `json:"userId" binding:"required"`
	InstanceID     uint       `json:"instanceId" binding:"required"`
	PermissionType string     `json:"permissionType" binding:"required,oneof=read write admin"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    string     `json:"description"`
	Permissions    []struct {
		DatabaseName string `json:"databaseName"` // 留空表示实例级别权限
	} `json:"permissions" binding:"required"`
}

type PermissionInfo struct {
	UserID         string     `json:"userId"`
	InstanceID     uint       `json:"instanceId"`
	InstanceName   string     `json:"instanceName"`
	DatabaseName   string     `json:"databaseName"`
	Table          string     `json:"tableName"`
	PermissionType string     `json:"permissionType"`
	GrantedBy      string     `json:"grantedBy"`
	GrantedAt      time.Time  `json:"grantedAt"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    string     `json:"description"`
}
