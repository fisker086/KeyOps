package bastion

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	bastionService "github.com/fisker086/keyops/internal/service/bastion"
	"github.com/gin-gonic/gin"
)

type HostMonitorHandler struct {
	monitorService *bastionService.HostMonitorService
}

func NewHostMonitorHandler(monitorService *bastionService.HostMonitorService) *HostMonitorHandler {
	return &HostMonitorHandler{
		monitorService: monitorService,
	}
}

// CheckHostStatus 手动检查指定主机状态
// POST /api/hosts/:id/check-status
func (h *HostMonitorHandler) CheckHostStatus(c *gin.Context) {
	hostID := c.Param("id")

	online, err := h.monitorService.CheckHostStatusNow(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	status := "offline"
	if online {
		status = "online"
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"online":  online,
		"status":  status,
		"message": "状态检查完成",
	}))
}

// CheckAllHostsStatus 手动检查所有主机状态
// POST /api/hosts/check-all-status
func (h *HostMonitorHandler) CheckAllHostsStatus(c *gin.Context) {
	// 异步执行，立即返回
	go h.monitorService.CheckAllHosts()

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "状态检查任务已启动，将在后台执行",
	}))
}
