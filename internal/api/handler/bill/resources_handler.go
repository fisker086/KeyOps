package bill

import (
	"net/http"
	"time"

	"github.com/fisker086/keyops/internal/model"
	billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type ResourcesHandler struct {
	service *billService.BillService
}

func NewResourcesHandler(service *billService.BillService) *ResourcesHandler {
	return &ResourcesHandler{service: service}
}

// ExpensesBreakdown 获取费用分解数据
// @Summary 获取资源费用分解
// @Description 按时间范围和分组维度获取费用分解数据
// @Tags bill
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2024-01-01)"
// @Param end_date query string true "结束日期 (格式: 2024-01-31)"
// @Param granularity query string false "粒度 (daily/monthly)" default(daily)
// @Param group_by query string false "分组维度 (service_name/service_code/cloud_type/region)" default(service_name)
// @Param vendor query string false "云厂商筛选"
// @Param service_code query string false "服务代码筛选"
// @Param keyword query string false "资源关键词筛选"
// @Success 200 {object} model.Response
// @Router /api/bill/resources/expenses-breakdown [get]
func (h *ResourcesHandler) ExpensesBreakdown(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	granularity := c.DefaultQuery("granularity", "daily")
	groupBy := c.DefaultQuery("group_by", "service_name")
	vendor := c.Query("vendor")
	serviceCode := c.Query("service_code")
	keyword := c.Query("keyword")

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

	result, err := h.service.GetExpensesBreakdown(startDate, endDate, granularity, groupBy, vendor, serviceCode, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// ResourceCountBreakdown 获取资源数量分解
// @Summary 获取资源数量分解
// @Description 按时间范围和分组维度获取资源数量分解数据
// @Tags bill
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期"
// @Param end_date query string true "结束日期"
// @Param group_by query string false "分组维度"
// @Success 200 {object} model.Response
// @Router /api/bill/resources/resource-count-breakdown [get]
func (h *ResourcesHandler) ResourceCountBreakdown(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	groupBy := c.DefaultQuery("group_by", "service_name")
	vendor := c.Query("vendor")

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

	result, err := h.service.GetResourceCountBreakdown(startDate, endDate, groupBy, vendor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}