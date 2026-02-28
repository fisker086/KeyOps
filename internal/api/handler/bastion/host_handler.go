package bastion

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	bastionService "github.com/fisker086/keyops/internal/service/bastion"
	"github.com/gin-gonic/gin"
)

type HostHandler struct {
	service        *bastionService.HostService
	monitorService *bastionService.HostMonitorService
}

func NewHostHandler(service *bastionService.HostService) *HostHandler {
	return &HostHandler{service: service}
}

// SetMonitorService 设置监控服务（用于创建主机后立即检查状态）
func (h *HostHandler) SetMonitorService(monitorService *bastionService.HostMonitorService) {
	h.monitorService = monitorService
}

func (h *HostHandler) CreateHost(c *gin.Context) {
	var host model.Host
	if err := c.ShouldBindJSON(&host); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 检查IP+端口组合是否已存在
	if host.Port == 0 {
		host.Port = 22 // 默认SSH端口
	}
	if err := h.service.CheckIPAndPortDuplicate(host.IP, host.Port, ""); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 验证设备类型
	if err := host.ValidateDeviceType(); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 如果未设置状态，默认为 unknown（表示还未检查）
	if host.Status == "" || host.Status == "offline" {
		host.Status = "unknown"
	}

	if err := h.service.CreateHost(&host); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// 创建成功后，异步触发一次状态检查
	if h.monitorService != nil {
		go func(hostID string) {
			_, _ = h.monitorService.CheckHostStatusNow(hostID)
		}(host.ID)
	}

	c.JSON(http.StatusOK, model.Success(host))
}

func (h *HostHandler) UpdateHost(c *gin.Context) {
	id := c.Param("id")
	var host model.Host
	if err := c.ShouldBindJSON(&host); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	// 如果提供了IP或端口，检查IP+端口组合是否与其他主机重复（排除自己）
	if host.IP != "" {
		// 如果端口未提供，使用现有主机的端口
		if host.Port == 0 {
			existing, err := h.service.GetHost(id)
			if err == nil && existing != nil {
				host.Port = existing.Port
			} else {
				host.Port = 22 // 默认SSH端口
			}
		}
		if err := h.service.CheckIPAndPortDuplicate(host.IP, host.Port, id); err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
			return
		}
	}

	// 如果提供了设备类型，验证它
	if host.DeviceType != "" {
		if err := host.ValidateDeviceType(); err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
			return
		}
	}

	if err := h.service.UpdateHost(id, &host); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

func (h *HostHandler) DeleteHost(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteHost(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

func (h *HostHandler) GetHost(c *gin.Context) {
	id := c.Param("id")
	host, err := h.service.GetHost(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "主机不存在"))
		return
	}

	// Host 模型已不再包含认证字段，无需移除敏感信息
	c.JSON(http.StatusOK, model.Success(host))
}

func (h *HostHandler) ListHosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	search := c.Query("search")
	tagsStr := c.Query("tags")

	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	// 获取当前用户信息
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	var hosts []model.Host
	var total int64
	var err error

	// 管理员查看所有主机
	// 普通用户只能查看有权限访问的主机（通过user_host_permissions或user_group_permissions授权）
	if role == "admin" {
		hosts, total, err = h.service.ListHosts(page, pageSize, search, tags)
	} else {
		// 普通用户：只显示有权限的主机
		hosts, total, err = h.service.ListHostsByPermissions(page, pageSize, search, tags, userID.(string))
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	// Host 模型已不再包含认证字段，无需移除敏感信息

	c.JSON(http.StatusOK, model.Success(model.HostsResponse{
		Hosts: hosts,
		Total: total,
	}))
}

// TestConnection 测试主机连接
// 需要提供 systemUserId 查询参数来指定使用哪个系统用户进行测试
func (h *HostHandler) TestConnection(c *gin.Context) {
	id := c.Param("id")
	systemUserID := c.Query("systemUserId")

	if systemUserID == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "需要提供系统用户ID (systemUserId) 进行连接测试"))
		return
	}

	// 这里应该实现完整的测试逻辑
	// 暂时返回提示信息
	c.JSON(http.StatusOK, model.Success(model.TestConnectionResponse{
		Success: false,
		Message: "测试连接功能需要配合系统用户实现。Host ID: " + id + ", System User ID: " + systemUserID,
	}))
}
