package bill

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type BillHandler struct {
	service *billService.BillService
}

func NewBillHandler(service *billService.BillService) *BillHandler {
	return &BillHandler{service: service}
}

// GetRecords 获取账单明细列表
// @Summary 获取账单明细列表
// @Description 获取账单明细列表，支持分页和筛选
// @Tags bill
// @Accept json
// @Produce json
// @Param vendor query string true "云厂商 (tencent/huawei-langgemap/huawei-bjlg)"
// @Param month query string true "账单月份 (格式: 2024-01)"
// @Param resource_code query string false "资源类型代码"
// @Param service_code query string false "服务类型代码"
// @Param page query int false "页码，从1开始" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param remote query string false "是否从云厂商API查询 (0=本地, 1=远程)" default(0)
// @Param with_amount query string false "是否计算费用 (0=否, 1=是)" default(0)
// @Success 200 {object} model.Response
// @Router /api/bill/records [get]
func (h *BillHandler) GetRecords(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")
	resourceCode := c.DefaultQuery("resource_code", "")
	serviceCode := c.DefaultQuery("service_code", "")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")
	remoteStr := c.DefaultQuery("remote", "0")
	withAmountStr := c.DefaultQuery("with_amount", "0")

	// 参数验证
	if vendor == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "vendor参数不能为空"))
		return
	}
	if month == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "month参数不能为空"))
		return
	}

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	queryRemote := remoteStr != "0"
	withAmount := withAmountStr != "0"

	// 参数验证和默认值处理
	if page <= 0 {
		page = 1
	}
	if pageSize < 0 {
		pageSize = 10 // 默认每页10条
	}
	// 如果page和pageSize都为0（或pageSize为0），表示全量查询
	if pageSize == 0 {
		page = 1 // 设置为1，避免offset为负数
	}

	result, err := h.service.GetRecords(vendor, month, resourceCode, serviceCode, page, pageSize, queryRemote, withAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetSummary 获取月度账单汇总
// @Summary 获取月度账单汇总
// @Description 获取指定云厂商和月份的账单汇总信息
// @Tags bill
// @Accept json
// @Produce json
// @Param vendor query string true "云厂商"
// @Param month query string true "账单月份"
// @Param remote query string false "是否从云厂商API查询" default(0)
// @Success 200 {object} model.Response
// @Router /api/bill/summary [get]
func (h *BillHandler) GetSummary(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")
	remoteStr := c.DefaultQuery("remote", "0")

	if vendor == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "vendor参数不能为空"))
		return
	}
	if month == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "month参数不能为空"))
		return
	}

	queryRemote := remoteStr != "0"
	result, err := h.service.GetSummary(vendor, month, queryRemote)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetStatistics 获取费用统计
// @Summary 获取费用统计
// @Description 获取当月总费用，用于前端展示饼图
// @Tags bill
// @Accept json
// @Produce json
// @Param month query string true "账单月份"
// @Success 200 {object} model.Response
// @Router /api/bill/statistics [get]
func (h *BillHandler) GetStatistics(c *gin.Context) {
	month := c.DefaultQuery("month", "")

	if month == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "month参数不能为空"))
		return
	}

	result, err := h.service.GetStatistics(month)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetTrend 获取费用趋势
// @Summary 获取费用趋势
// @Description 获取月度费用列表，用于前端展示折线图
// @Tags bill
// @Accept json
// @Produce json
// @Param vendor query string false "云厂商"
// @Param year query string false "年份"
// @Success 200 {object} model.Response
// @Router /api/bill/trend [get]
func (h *BillHandler) GetTrend(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	year := c.DefaultQuery("year", "")

	result, err := h.service.GetTrend(vendor, year)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetTrendMonth 获取费用趋势月份列表
// @Summary 获取费用趋势月份列表
// @Description 获取折线图上x轴的月份列表，默认查询最近6个月
// @Tags bill
// @Accept json
// @Produce json
// @Param year query string false "年份"
// @Success 200 {object} model.Response
// @Router /api/bill/trend/month [get]
func (h *BillHandler) GetTrendMonth(c *gin.Context) {
	year := c.DefaultQuery("year", "")

	result, err := h.service.GetTrendMonth(year)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetVM 获取虚拟机分摊账单
// @Summary 获取虚拟机分摊账单
// @Description 获取虚拟机分摊账单（本地数据库，包含硬盘）
// @Tags bill
// @Accept json
// @Produce json
// @Param vendor query string true "云厂商"
// @Param month query string true "账单月份"
// @Param split_type query string false "分摊类型 (department/business)"
// @Param with_detail query string false "是否包含详情" default(0)
// @Success 200 {object} model.Response
// @Router /api/bill/vm [get]
func (h *BillHandler) GetVM(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")
	splitType := c.DefaultQuery("split_type", "")
	withDetailStr := c.DefaultQuery("with_detail", "0")

	if vendor == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "vendor参数不能为空"))
		return
	}
	if month == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "month参数不能为空"))
		return
	}

	withDetail := withDetailStr != "0"
	result, err := h.service.GetVM(vendor, month, splitType, withDetail)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetPriceList 获取单价列表
// @Summary 获取单价列表
// @Description 获取单价管理列表
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/price [get]
func (h *BillHandler) GetPriceList(c *gin.Context) {
	result, err := h.service.GetPriceList()
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// CreatePrice 创建单价
// @Summary 创建单价
// @Description 创建单价配置
// @Tags bill
// @Accept json
// @Produce json
// @Param price body model.BillPrice true "单价信息"
// @Success 200 {object} model.Response
// @Router /api/bill/price [post]
func (h *BillHandler) CreatePrice(c *gin.Context) {
	var price model.BillPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.CreatePrice(&price)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// UpdatePrice 更新单价
// @Summary 更新单价
// @Description 更新单价配置
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "单价ID"
// @Param price body model.BillPrice true "单价信息"
// @Success 200 {object} model.Response
// @Router /api/bill/price/:id [put]
func (h *BillHandler) UpdatePrice(c *gin.Context) {
	id := c.Param("id")
	var price model.BillPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.UpdatePrice(id, &price)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetResource 获取我的资源列表
// @Summary 获取我的资源列表
// @Description 获取我的资源列表
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/resource [get]
func (h *BillHandler) GetResource(c *gin.Context) {
	result, err := h.service.GetResource()
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

