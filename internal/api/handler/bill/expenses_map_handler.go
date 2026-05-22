package bill

import (
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
		billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type ExpensesMapHandler struct {
	service *billService.BillService
}

func NewExpensesMapHandler(service *billService.BillService) *ExpensesMapHandler {
	return &ExpensesMapHandler{service: service}
}

// GetRegionExpenses 获取按区域的费用
// @Summary 获取按区域的费用
// @Description 按区域聚合费用数据，用于费用地图展示
// @Tags bill
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2024-01-01)"
// @Param end_date query string true "结束日期 (格式: 2024-01-31)"
// @Success 200 {object} model.Response
// @Router /api/bill/region-expenses [get]
func (h *ExpensesMapHandler) GetRegionExpenses(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "start_date and end_date are required"))
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid start_date format"))
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid end_date format"))
		return
	}

	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, model.Error(400, "end_date must be >= start_date"))
		return
	}

	if endDate.Sub(startDate).Hours()/24 > 1095 {
		c.JSON(http.StatusBadRequest, model.Error(400, "date range cannot exceed 3 years"))
		return
	}

	result, err := h.service.GetRegionExpenses(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetTrafficExpenses 获取按流量/跨区的费用
// @Summary 获取按流量/跨区的费用
// @Description 按流量聚合费用数据，用于费用地图展示
// @Tags bill
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2024-01-01)"
// @Param end_date query string true "结束日期 (格式: 2024-01-31)"
// @Param resource_id query string false "资源ID筛选"
// @Success 200 {object} model.Response
// @Router /api/bill/traffic-expenses [get]
func (h *ExpensesMapHandler) GetTrafficExpenses(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	resourceID := c.Query("resource_id")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "start_date and end_date are required"))
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid start_date format"))
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid end_date format"))
		return
	}

	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, model.Error(400, "end_date must be >= start_date"))
		return
	}

	if endDate.Sub(startDate).Hours()/24 > 1095 {
		c.JSON(http.StatusBadRequest, model.Error(400, "date range cannot exceed 3 years"))
		return
	}

	result, err := h.service.GetTrafficExpenses(startDate, endDate, resourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}