package system

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	systemService "github.com/fisker086/keyops/internal/service/system"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type AssetSyncHandler struct {
	repo    *repository.AssetSyncRepository
	service *systemService.AssetSyncService
}

func NewAssetSyncHandler(repo *repository.AssetSyncRepository, svc *systemService.AssetSyncService) *AssetSyncHandler {
	return &AssetSyncHandler{
		repo:    repo,
		service: svc,
	}
}

// ListConfigs 获取所有同步配置
func (h *AssetSyncHandler) ListConfigs(c *gin.Context) {
	configs, err := h.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"configs": configs,
	}))
}

// CreateConfig 创建同步配置
func (h *AssetSyncHandler) CreateConfig(c *gin.Context) {
	var req model.AssetSyncConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	req.ID = uuid.New().String()

	if err := h.repo.Create(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(req))
}

// UpdateConfig 更新同步配置
func (h *AssetSyncHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")

	var req model.AssetSyncConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	req.ID = id

	if err := h.repo.Update(&req); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(req))
}

// DeleteConfig 删除同步配置
func (h *AssetSyncHandler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(nil))
}

// ToggleConfig 启用/禁用配置
func (h *AssetSyncHandler) ToggleConfig(c *gin.Context) {
	id := c.Param("id")

	config, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, "配置不存在"))
		return
	}

	config.Enabled = !config.Enabled

	if err := h.repo.Update(config); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(config))
}

// SyncNow 立即同步
func (h *AssetSyncHandler) SyncNow(c *gin.Context) {
	id := c.Param("id")

	// 异步执行同步
	go func() {
		if err := h.service.SyncNow(id); err != nil {
			// 错误已经记录在日志中
		}
	}()

	c.JSON(http.StatusOK, model.Success(gin.H{
		"message": "同步任务已启动",
	}))
}

// GetLogs 获取同步日志
func (h *AssetSyncHandler) GetLogs(c *gin.Context) {
	configID := c.Query("configId")

	logs, err := h.repo.GetLogs(configID, 50) // 最多返回50条
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"logs": logs,
	}))
}

// DownloadExcelTemplate 下载资产同步 Excel 模板（第一行为表头，第二行为示例数据）
func (h *AssetSyncHandler) DownloadExcelTemplate(c *gin.Context) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	// 第一行：表头（与格式要求一致）
	headers := []interface{}{"主机名", "IP地址", "端口", "设备类型", "标签", "描述"}
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "生成模板失败"))
		return
	}
	// 第二行：示例数据
	exampleRow := []interface{}{"示例服务器", "192.168.1.100", 22, "server", "生产环境,Web", "示例主机描述"}
	if err := f.SetSheetRow(sheetName, "A2", &exampleRow); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "生成模板失败"))
		return
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, "生成模板失败"))
		return
	}

	fileName := "资产同步模板.xlsx"
	c.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}
