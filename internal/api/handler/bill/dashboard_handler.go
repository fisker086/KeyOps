package bill

import (
	"net/http"

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

// GetDashboardData 获取 Dashboard 数据
// @Summary 获取费用 Dashboard 数据
// @Description 获取本月费用、预测、环比、Top资源等信息
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/dashboard [get]
func (h *DashboardHandler) GetDashboardData(c *gin.Context) {
	currency := c.DefaultQuery("currency", "CNY")
	if currency != "USD" && currency != "CNY" {
		currency = "CNY"
	}
	result, err := h.service.GetDashboardData(currency)
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
