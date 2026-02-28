package k8s

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fisker086/keyops/pkg/casbin"
	"github.com/fisker086/keyops/pkg/logger"
)

// K8sPermissionService K8s权限服务
type K8sPermissionService struct {
}

// NewK8sPermissionService 创建K8s权限服务
func NewK8sPermissionService() *K8sPermissionService {
	return &K8sPermissionService{}
}

// ResourceType 资源类型
type ResourceType string

const (
	ResourceTypeNamespace   ResourceType = "namespace"
	ResourceTypeDeployment  ResourceType = "deployment"
	ResourceTypeStatefulSet ResourceType = "statefulset"
	ResourceTypeDaemonSet   ResourceType = "daemonset"
	ResourceTypeService     ResourceType = "service"
	ResourceTypePod         ResourceType = "pod"
	ResourceTypeIngress     ResourceType = "ingress"
	ResourceTypePVC         ResourceType = "pvc"
)

// Action 操作类型
type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionAdmin  Action = "admin"
)

// BuildResourcePath 构建资源路径
// 格式: /k8s/cluster/{clusterId}/namespace/{namespace}/deployment/{deploymentName}
// 或者: /k8s/cluster/{clusterId}/namespace/{namespace}
func BuildResourcePath(clusterID string, namespace string, resourceType ResourceType, resourceName string) string {
	path := fmt.Sprintf("/k8s/cluster/%s/namespace/%s", clusterID, namespace)

	if resourceType != ResourceTypeNamespace && resourceName != "" {
		path = fmt.Sprintf("%s/%s/%s", path, resourceType, resourceName)
	}

	return path
}

// CheckPermission 检查权限
// sub: 用户ID或角色ID
// clusterID: 集群ID
// namespace: 命名空间
// resourceType: 资源类型
// resourceName: 资源名称（可选，如果为空则检查命名空间权限）
// action: 操作类型
func (s *K8sPermissionService) CheckPermission(sub string, clusterID string, namespace string, resourceType ResourceType, resourceName string, action Action) (bool, error) {
	// 构建资源路径
	resourcePath := BuildResourcePath(clusterID, namespace, resourceType, resourceName)

	// 使用casbin检查权限
	hasPermission, err := casbin.Enforce(sub, resourcePath, string(action))
	if err != nil {
		logger.Errorf("Casbin权限检查失败: %v", err)
		return false, err
	}

	// 如果检查失败，尝试检查更高级别的权限（命名空间级别）
	if !hasPermission && resourceName != "" {
		namespacePath := BuildResourcePath(clusterID, namespace, ResourceTypeNamespace, "")
		hasPermission, err = casbin.Enforce(sub, namespacePath, string(action))
		if err != nil {
			logger.Errorf("Casbin命名空间权限检查失败: %v", err)
			return false, err
		}
	}

	// 如果还是失败，尝试检查集群级别的权限
	if !hasPermission {
		clusterPath := fmt.Sprintf("/k8s/cluster/%s", clusterID)
		hasPermission, err = casbin.Enforce(sub, clusterPath, string(action))
		if err != nil {
			logger.Errorf("Casbin集群权限检查失败: %v", err)
			return false, err
		}
	}

	return hasPermission, nil
}

// GetAPIPermissionsForResource 根据资源权限获取对应的API权限
// 返回: API路径列表和对应的HTTP方法
func GetAPIPermissionsForResource(resourceType ResourceType, action Action) []struct {
	Path   string
	Method string
} {
	var apiPermissions []struct {
		Path   string
		Method string
	}

	// Pod权限映射
	if resourceType == ResourceTypePod {
		if action == ActionRead {
			// read权限 → 查看日志API
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod/ws/logs", "GET"})
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod/down_logs", "GET"})
		}
		if action == ActionWrite || action == ActionAdmin {
			// write/admin权限 → 终端和exec API
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod/ws/terminal", "GET"})
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod/ws/exec", "GET"})
			// write权限也包含read权限
			if action == ActionWrite {
				apiPermissions = append(apiPermissions, struct {
					Path   string
					Method string
				}{"/api/v1/kube/pod/ws/logs", "GET"})
				apiPermissions = append(apiPermissions, struct {
					Path   string
					Method string
				}{"/api/v1/kube/pod/down_logs", "GET"})
			}
		}
		if action == ActionDelete || action == ActionAdmin {
			// delete权限 → 删除Pod API
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod", "DELETE"})
		}
	}

	// Deployment权限映射
	if resourceType == ResourceTypeDeployment {
		if action == ActionWrite || action == ActionAdmin {
			// write权限 → 扩缩容等操作
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/scale", "POST"})
		}
		if action == ActionDelete || action == ActionAdmin {
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/yaml", "DELETE"})
		}
	}

	// Service/Ingress/PVC/StatefulSet/DaemonSet delete 权限 → 删除资源 API
	if action == ActionDelete || action == ActionAdmin {
		switch resourceType {
		case ResourceTypeService, ResourceTypeIngress, ResourceTypePVC, ResourceTypeStatefulSet, ResourceTypeDaemonSet:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/yaml", "DELETE"})
		}
	}

	// 通用权限：所有资源类型的read权限都包含查看列表的权限
	if action == ActionRead || action == ActionWrite || action == ActionAdmin {
		switch resourceType {
		case ResourceTypePod:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod", "GET"})
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pod/detail", "GET"})
		case ResourceTypeDeployment:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/deployment", "GET"})
		case ResourceTypeStatefulSet:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/statefulset", "GET"})
		case ResourceTypeDaemonSet:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/daemonset", "GET"})
		case ResourceTypeService:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/service", "GET"})
		case ResourceTypeIngress:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/ingress", "GET"})
		case ResourceTypePVC:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/pvc", "GET"})
		case ResourceTypeNamespace:
			apiPermissions = append(apiPermissions, struct {
				Path   string
				Method string
			}{"/api/v1/kube/namespace", "GET"})
		}
	}

	return apiPermissions
}

// AddPermission 添加权限策略（包括自动创建API权限）
func (s *K8sPermissionService) AddPermission(sub string, clusterID string, namespace string, resourceType ResourceType, resourceName string, action Action) (bool, error) {
	// 添加资源权限
	resourcePath := BuildResourcePath(clusterID, namespace, resourceType, resourceName)
	success, err := casbin.AddPolicy(sub, resourcePath, string(action))
	if err != nil {
		return false, err
	}
	if !success {
		// 权限已存在，但仍然需要检查API权限
		logger.Warnf("资源权限已存在: %s %s %s", sub, resourcePath, action)
	}

	// 自动创建对应的API权限（仅在资源权限添加成功时）
	if success {
		apiPermissions := GetAPIPermissionsForResource(resourceType, action)
		for _, apiPerm := range apiPermissions {
			// 注意：API权限路径不需要包含clusterID和namespace，因为API权限是全局的
			// 但我们可以通过查询参数来限制，这里先创建全局API权限
			// 如果API权限已存在，忽略错误（casbin会自动去重）
			apiSuccess, err := casbin.AddPolicy(sub, apiPerm.Path, apiPerm.Method)
			if err != nil {
				logger.Warnf("添加API权限失败: %s %s %s, 错误: %v", sub, apiPerm.Path, apiPerm.Method, err)
			} else if apiSuccess {
				logger.Infof("自动创建API权限: %s %s %s", sub, apiPerm.Path, apiPerm.Method)
			} else {
				logger.Debugf("API权限已存在: %s %s %s", sub, apiPerm.Path, apiPerm.Method)
			}
		}
	}

	return success, nil
}

// RemovePermission 删除权限策略（包括自动删除API权限）
func (s *K8sPermissionService) RemovePermission(sub string, clusterID string, namespace string, resourceType ResourceType, resourceName string, action Action) (bool, error) {
	// 删除资源权限
	resourcePath := BuildResourcePath(clusterID, namespace, resourceType, resourceName)
	success, err := casbin.RemovePolicy(sub, resourcePath, string(action))
	if err != nil {
		return false, err
	}

	// 检查是否还有其他资源权限（可能影响API权限的保留）
	// 如果用户还有其他相同资源类型的权限，保留API权限
	// 删除资源权限时，检查是否还有其他相同资源类型的权限需要这个API权限
	if success {
		apiPermissions := GetAPIPermissionsForResource(resourceType, action)
		for _, apiPerm := range apiPermissions {
			// 检查是否还有其他相同资源类型的权限需要这个API权限
			shouldRemove := true

			// 获取该用户/角色的所有权限
			allPolicies, err := casbin.GetFilteredPolicy(0, sub)
			if err == nil {
				// 检查是否还有其他相同资源类型的权限
				for _, policy := range allPolicies {
					if len(policy) >= 3 {
						policyPath := policy[1]
						policyAction := policy[2]

						// 解析路径，检查是否是相同资源类型
						_, _, policyResourceType, _, parseErr := ParseResourcePath(policyPath)
						if parseErr == nil && policyResourceType == resourceType && policyAction == string(action) {
							// 如果还有其他相同资源类型的权限，保留API权限
							shouldRemove = false
							break
						}
					}
				}
			}

			if shouldRemove {
				apiSuccess, err := casbin.RemovePolicy(sub, apiPerm.Path, apiPerm.Method)
				if err != nil {
					logger.Warnf("删除API权限失败: %s %s %s, 错误: %v", sub, apiPerm.Path, apiPerm.Method, err)
				} else if apiSuccess {
					logger.Infof("自动删除API权限: %s %s %s", sub, apiPerm.Path, apiPerm.Method)
				} else {
					logger.Debugf("API权限不存在或已被删除: %s %s %s", sub, apiPerm.Path, apiPerm.Method)
				}
			} else {
				logger.Debugf("保留API权限（还有其他资源权限需要）: %s %s %s", sub, apiPerm.Path, apiPerm.Method)
			}
		}
	}

	return success, nil
}

// UpdatePermission 更新权限策略（原子操作：使用 Casbin UpdatePolicy）
// 同时同步关联的 API 权限（如日志、终端等）
func (s *K8sPermissionService) UpdatePermission(
	oldSub, oldClusterID, oldNamespace string, oldResourceType ResourceType, oldResourceName string, oldAction Action,
	newSub, newClusterID, newNamespace string, newResourceType ResourceType, newResourceName string, newAction Action,
) (bool, error) {
	oldPath := BuildResourcePath(oldClusterID, oldNamespace, oldResourceType, oldResourceName)
	newPath := BuildResourcePath(newClusterID, newNamespace, newResourceType, newResourceName)
	oldRule := []string{oldSub, oldPath, string(oldAction)}
	newRule := []string{newSub, newPath, string(newAction)}

	success, err := casbin.UpdatePolicy(oldRule, newRule)
	if err != nil {
		return false, err
	}
	if !success {
		logger.Warnf("更新权限失败或规则不存在: old=%v", oldRule)
		return false, nil
	}

	// 同步 API 权限：移除旧 sub 的 API 权限（若无其他策略需要则删除）
	if oldSub != "" {
		oldAPIPerms := GetAPIPermissionsForResource(oldResourceType, oldAction)
		for _, apiPerm := range oldAPIPerms {
			shouldRemove := true
			allPolicies, err := casbin.GetFilteredPolicy(0, oldSub)
			if err == nil {
				for _, policy := range allPolicies {
					if len(policy) >= 3 {
						_, _, rt, _, parseErr := ParseResourcePath(policy[1])
						if parseErr == nil && rt == oldResourceType && policy[2] == string(oldAction) {
							shouldRemove = false
							break
						}
					}
				}
			}
			if shouldRemove {
				casbin.RemovePolicy(oldSub, apiPerm.Path, apiPerm.Method)
			}
		}
	}

	// 同步 API 权限：为新 sub 添加 API 权限
	if newSub != "" {
		newAPIPerms := GetAPIPermissionsForResource(newResourceType, newAction)
		for _, apiPerm := range newAPIPerms {
			casbin.AddPolicy(newSub, apiPerm.Path, apiPerm.Method)
		}
	}

	return true, nil
}

// GetPermissions 获取用户/角色的所有K8s权限
func (s *K8sPermissionService) GetPermissions(sub string) ([][]string, error) {
	// 获取所有以 /k8s/cluster/ 开头的策略
	return casbin.GetFilteredPolicy(0, sub)
}

// ParseResourcePath 解析资源路径
// 返回: clusterID, namespace, resourceType, resourceName
func ParseResourcePath(path string) (string, string, ResourceType, string, error) {
	// 格式: /k8s/cluster/{clusterId}/namespace/{namespace}/deployment/{deploymentName}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	if len(parts) < 4 || parts[0] != "k8s" || parts[1] != "cluster" {
		return "", "", "", "", fmt.Errorf("无效的资源路径格式: %s", path)
	}

	clusterID := parts[2]

	if len(parts) < 5 || parts[3] != "namespace" {
		return clusterID, "", ResourceTypeNamespace, "", nil
	}

	namespace := parts[4]

	if len(parts) < 6 {
		return clusterID, namespace, ResourceTypeNamespace, "", nil
	}

	resourceType := ResourceType(parts[5])
	resourceName := ""
	if len(parts) > 6 {
		resourceName = parts[6]
	}

	return clusterID, namespace, resourceType, resourceName, nil
}

// GetPermittedNamespacesForSubs 获取指定主体（用户+角色）在某个集群下的可访问命名空间列表
// subs: 用户ID + 角色ID 列表，如 [userId, roleId1, roleId2]
// 返回 nil 表示无限制（集群级权限），返回 []string 表示仅能访问列出的命名空间（管理员也按权限管理控制）
func (s *K8sPermissionService) GetPermittedNamespacesForSubs(clusterID string, subs []string) ([]string, error) {
	nsSet := make(map[string]bool)
	hasClusterLevel := false

	for _, sub := range subs {
		if sub == "" {
			continue
		}
		policies, err := casbin.GetFilteredPolicy(0, sub)
		if err != nil {
			logger.Warnf("GetFilteredPolicy for sub=%s failed: %v", sub, err)
			continue
		}
		for _, policy := range policies {
			if len(policy) < 2 {
				continue
			}
			path := policy[1]
			parsedClusterID, namespace, _, _, err := ParseResourcePath(path)
			if err != nil {
				continue
			}
			if parsedClusterID != clusterID {
				continue
			}
			// 集群级权限：path 形如 /k8s/cluster/{id}，namespace 解析为空
			if namespace == "" {
				hasClusterLevel = true
				break
			}
			nsSet[namespace] = true
		}
		if hasClusterLevel {
			break
		}
	}

	if hasClusterLevel {
		return nil, nil // 无限制，可访问所有命名空间
	}
	if len(nsSet) == 0 {
		return []string{}, nil // 无任何命名空间权限
	}
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)
	return namespaces, nil
}

// actionLevel 用于比较权限等级
var actionLevels = map[Action]int{
	ActionRead:   1,
	ActionWrite:  2,
	ActionDelete: 3,
	ActionAdmin:  4,
}

// GetEffectiveActionForSubs 获取指定主体在 cluster/namespace/resourceType 下的最高权限
// 用于前端按权限控制显示扩缩容、回滚、删除等按钮
func (s *K8sPermissionService) GetEffectiveActionForSubs(clusterID string, namespace string, resourceType ResourceType, subs []string) (Action, error) {
	maxLevel := 0
	var maxAction Action

	for _, sub := range subs {
		if sub == "" {
			continue
		}
		policies, err := casbin.GetFilteredPolicy(0, sub)
		if err != nil {
			continue
		}
		for _, policy := range policies {
			if len(policy) < 3 {
				continue
			}
			path := policy[1]
			act := Action(policy[2])
			level, ok := actionLevels[act]
			if !ok {
				continue
			}
			parsedClusterID, parsedNs, parsedRT, _, err := ParseResourcePath(path)
			if err != nil || parsedClusterID != clusterID || parsedNs != namespace {
				continue
			}
			// 命名空间级权限适用于所有资源类型
			if parsedRT == ResourceTypeNamespace {
				if level > maxLevel {
					maxLevel = level
					maxAction = act
				}
				continue
			}
			// 资源类型需匹配（或为空表示该类型下所有资源）
			if parsedRT != resourceType {
				continue
			}
			if level > maxLevel {
				maxLevel = level
				maxAction = act
			}
		}
	}

	if maxLevel == 0 {
		return "", nil // 无权限
	}
	return maxAction, nil
}

// ReloadPolicy 重新加载策略
func ReloadPolicy() error {
	return casbin.ReloadPolicy()
}

// GetAllPolicies 获取所有策略
func GetAllPolicies() ([][]string, error) {
	return casbin.GetFilteredPolicy(-1)
}
