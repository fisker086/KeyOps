package system

import (
	"net/http"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	registryService "github.com/fisker086/keyops/internal/service/registry"
	"github.com/gin-gonic/gin"
)

type RegistryHandler struct {
	appRepo *repository.ApplicationRepository
	svc     *registryService.Service
}

func NewRegistryHandler(appRepo *repository.ApplicationRepository, svc *registryService.Service) *RegistryHandler {
	return &RegistryHandler{appRepo: appRepo, svc: svc}
}

// GetApplicationVersions 根据应用 ID 获取该应用在已配置容器仓库中的制品版本号列表（Harbor/ECR/Nexus）
// GET /api/registry/applications/:appId/versions
func (h *RegistryHandler) GetApplicationVersions(c *gin.Context) {
	appId := c.Param("appId")
	if appId == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "appId required"))
		return
	}
	app, err := h.appRepo.FindByID(appId)
	if err != nil || app == nil {
		c.JSON(http.StatusNotFound, model.Error(404, "application not found"))
		return
	}
	appName := app.Name
	if appName == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "application name is empty"))
		return
	}
	tags, err := h.svc.ListTags(c.Request.Context(), appName)
	if err != nil {
		errMsg := err.Error()
		// 未配置或类型不支持：不报错，直接返回空列表（前端不弹服务器错误）
		if strings.Contains(errMsg, "registry not configured") || strings.Contains(errMsg, "unsupported registry_type") {
			c.JSON(http.StatusOK, model.Success(gin.H{"versions": []string{}}))
			return
		}
		c.JSON(http.StatusInternalServerError, model.Error(500, "list registry tags: "+errMsg))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"versions": tags}))
}

// TestConnection 测试制品仓库连接（使用当前 Settings 中的 registry 配置）
// GET /api/registry/test?app_name=test
func (h *RegistryHandler) TestConnection(c *gin.Context) {
	appName := c.Query("app_name")
	if appName == "" {
		appName = "test"
	}
	_, err := h.svc.ListTags(c.Request.Context(), appName)
	if err != nil {
		errMsg := err.Error()
		// 未配置或类型不支持：返回 200 + ok:false，前端用提示展示，不弹服务器错误
		if strings.Contains(errMsg, "registry not configured") || strings.Contains(errMsg, "unsupported registry_type") {
			c.JSON(http.StatusOK, model.Success(gin.H{"ok": false, "message": "未配置容器仓库，请先选择仓库类型并填写地址与认证信息"}))
			return
		}
		c.JSON(http.StatusOK, model.Success(gin.H{"ok": false, "message": "连接失败: " + errMsg}))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "message": "连接成功"}))
}
