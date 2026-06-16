package bill

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	service *billService.BillService
}

func NewDashboardHandler(service *billService.BillService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

func parseBillCurrency(c *gin.Context) string {
	return billService.NormalizeDisplayCurrency(c.DefaultQuery("currency", "CNY"))
}

// GetDashboardData 获取 Dashboard 数据
// @Summary 获取费用 Dashboard 数据
// @Description 获取本月费用、预测、环比、Top资源等信息
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/dashboard [get]
func (h *DashboardHandler) GetDashboardData(c *gin.Context) {
	result, err := h.service.GetDashboardData(parseBillCurrency(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

func (h *DashboardHandler) GetDashboardSummary(c *gin.Context) {
	result, err := h.service.GetDashboardSummary(parseBillCurrency(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

func (h *DashboardHandler) GetDashboardCharts(c *gin.Context) {
	result, err := h.service.GetDashboardCharts(parseBillCurrency(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

func (h *DashboardHandler) GetDashboardTrend(c *gin.Context) {
	result, err := h.service.GetDashboardTrend(parseBillCurrency(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// GetDashboardTopResources 获取 Dashboard Top 资源（独立接口，降低主接口延迟）
func (h *DashboardHandler) GetDashboardTopResources(c *gin.Context) {
	currency := parseBillCurrency(c)
	limit := 10
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	result, err := h.service.GetDashboardTopResources(currency, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// GetDashboardFull 获取全量 Dashboard 数据（合并 4 个接口，单次 HTTP 请求）
func (h *DashboardHandler) GetDashboardFull(c *gin.Context) {
	result, err := h.service.GetDashboardFull(parseBillCurrency(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// GetInsights 获取成本优化洞察
func (h *DashboardHandler) GetInsights(c *gin.Context) {
	result, err := h.service.GetInsights()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// GetRecommendations 获取优化建议
// @Summary 获取优化建议
// @Description 获取资源优化建议（未使用、Rightsizing等）
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/recommendations [get]
func (h *DashboardHandler) GetRecommendations(c *gin.Context) {
	result, err := h.service.GetRecommendations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}
