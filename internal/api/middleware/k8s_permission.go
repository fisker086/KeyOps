package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	k8sService "github.com/fisker086/keyops/internal/service/k8s"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/gin-gonic/gin"
)

// K8sPermissionMiddleware K8s资源权限中间件
// 检查用户是否有权限访问指定的K8s资源
// 使用统一的 Casbin 权限系统，权限存储在 casbin_rule 表中
func K8sPermissionMiddleware(permissionService *k8sService.K8sPermissionService, roleRepo *repository.RoleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket 请求跳过权限中间件，由 handler 自己处理权限验证
		if strings.Contains(c.Request.URL.Path, "/ws/") {
			c.Next()
			return
		}

		// 获取用户ID（AuthMiddleware 设置的是 userID）
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, model.Error(401, "未找到用户信息"))
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, model.Error(401, "用户ID格式错误"))
			c.Abort()
			return
		}

		// 获取用户的所有角色
		roles, err := roleRepo.GetRolesByUserID(userIDStr)
		if err != nil {
			logger.Warnf("获取用户角色失败: %v", err)
		}

		// 从请求参数中获取资源信息
		clusterID := c.Query("cluster_id")
		if clusterID == "" {
			clusterID = c.Param("cluster_id")
		}
		if clusterID == "" {
			// 尝试从body中获取（需要先读取并恢复body，避免消耗请求体）
			if c.Request.Body != nil && c.Request.ContentLength > 0 {
				// 读取原始body
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err == nil && len(bodyBytes) > 0 {
					// 恢复body供后续handler使用
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// 解析JSON获取cluster_id
					var body map[string]interface{}
					if err := json.Unmarshal(bodyBytes, &body); err == nil {
						if id, ok := body["cluster_id"].(string); ok {
							clusterID = id
						}
					}
				}
			}
		}

		namespace := c.Query("namespace")
		if namespace == "" {
			namespace = c.Param("namespace")
		}

		// 确定资源类型和名称
		resourceType := k8sService.ResourceTypeNamespace
		resourceName := ""

		// 根据路径判断资源类型
		path := c.Request.URL.Path
		if strings.Contains(path, "/deployment") {
			resourceType = k8sService.ResourceTypeDeployment
			resourceName = c.Query("deployment_name")
			if resourceName == "" {
				resourceName = c.Param("deployment_name")
			}
		} else if strings.Contains(path, "/statefulset") {
			resourceType = k8sService.ResourceTypeStatefulSet
			resourceName = c.Query("statefulset_name")
			if resourceName == "" {
				resourceName = c.Param("statefulset_name")
			}
		} else if strings.Contains(path, "/service") {
			resourceType = k8sService.ResourceTypeService
			resourceName = c.Query("service_name")
			if resourceName == "" {
				resourceName = c.Param("service_name")
			}
		} else if strings.Contains(path, "/pod") {
			resourceType = k8sService.ResourceTypePod
			resourceName = c.Query("pod_name")
			if resourceName == "" {
				resourceName = c.Param("pod_name")
			}
		} else if strings.Contains(path, "/ingress") {
			resourceType = k8sService.ResourceTypeIngress
			resourceName = c.Query("ingress_name")
			if resourceName == "" {
				resourceName = c.Param("ingress_name")
			}
		} else if strings.Contains(path, "/yaml") {
			// YAML 接口从 query 获取 resource_type 和 resource_name
			rt := strings.ToLower(c.Query("resource_type"))
			resourceName = c.Query("resource_name")
			switch rt {
			case "deployment":
				resourceType = k8sService.ResourceTypeDeployment
			case "statefulset":
				resourceType = k8sService.ResourceTypeStatefulSet
			case "daemonset":
				resourceType = k8sService.ResourceTypeDaemonSet
			case "service":
				resourceType = k8sService.ResourceTypeService
			case "pod":
				resourceType = k8sService.ResourceTypePod
			case "ingress":
				resourceType = k8sService.ResourceTypeIngress
			case "pvc":
				resourceType = k8sService.ResourceTypePVC
			case "configmap", "secret", "job", "cronjob":
				resourceType = k8sService.ResourceTypeNamespace
			default:
				resourceType = k8sService.ResourceTypeNamespace
			}
		}

		// 确定操作类型
		action := k8sService.ActionRead
		method := c.Request.Method
		switch method {
		case "GET":
			action = k8sService.ActionRead
		case "POST", "PUT", "PATCH":
			action = k8sService.ActionWrite
		case "DELETE":
			action = k8sService.ActionDelete
		}

		// 如果没有集群ID，跳过权限检查（可能是其他API）
		if clusterID == "" {
			c.Next()
			return
		}

		// 优先级1: 检查用户直接权限
		hasPermission, err := permissionService.CheckPermission(userIDStr, clusterID, namespace, resourceType, resourceName, action)
		if err != nil {
			logger.Errorf("用户权限检查失败: %v", err)
			c.JSON(http.StatusInternalServerError, model.Error(500, "权限检查失败"))
			c.Abort()
			return
		}

		// 优先级2: 如果用户没有权限，检查角色权限
		if !hasPermission && len(roles) > 0 {
			for _, role := range roles {
				hasPermission, err = permissionService.CheckPermission(role.ID, clusterID, namespace, resourceType, resourceName, action)
				if err != nil {
					logger.Warnf("角色权限检查失败: %v", err)
					continue
				}
				if hasPermission {
					break
				}
			}
		}

		// 管理员也按权限管理控制，不特殊对待
		if !hasPermission {
			c.JSON(http.StatusForbidden, model.Error(403, "没有访问该资源的权限"))
			c.Abort()
			return
		}

		c.Next()
	}
}
