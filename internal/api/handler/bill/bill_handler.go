package bill

import (
	"net/http"
	"strconv"
	"time"

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



// GetResource 获取我的资源列表
// @Summary 获取我的资源列表
// @Description 获取我的资源列表
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/resource [get]
func (h *BillHandler) GetResource(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page <= 0 {
		page = 1
	}
	if pageSize < 0 {
		pageSize = 10
	}

	result, err := h.service.GetResource(vendor, page, pageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

func stripCloudAccountSecret(a *model.CloudAccount) {
	if a == nil {
		return
	}
	a.SecretAccessKey = ""
}

// SyncBilling 同步账单数据
func (h *BillHandler) SyncBilling(c *gin.Context) {
	cloudAccountID, err := strconv.ParseUint(c.Param("cloud_account_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid cloud_account_id"))
		return
	}
	billingDate := time.Now()
	if dateStr := c.Query("date"); dateStr != "" {
		billingDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid date format"))
			return
		}
	}
	h.service.SyncCloudBillingAsync(uint(cloudAccountID), billingDate)
	c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "async": true}))
}

// GetSummaryByCloud 按云厂商汇总（含图表用 series）
func (h *BillHandler) GetSummaryByCloud(c *gin.Context) {
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
	summary, err := h.service.GetBillingSummaryByCloud(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	type seriesRow struct {
		Vendor string  `json:"vendor"`
		Amount float64 `json:"amount"`
	}
	series := make([]seriesRow, 0, len(summary))
	for v, d := range summary {
		f, _ := d.Float64()
		series = append(series, seriesRow{Vendor: v, Amount: f})
	}
	c.JSON(http.StatusOK, model.Success(gin.H{
		"by_vendor": summary,
		"series":    series,
	}))
}

// GetPricing 获取定价信息（可选 cloud_account_id 使用已存凭证）
func (h *BillHandler) GetPricing(c *gin.Context) {
	cloudType := c.DefaultQuery("cloud_type", "aws")
	filters := make(map[string]string)
	if serviceCode := c.Query("service_code"); serviceCode != "" {
		filters["servicecode"] = serviceCode
	}
	if instanceType := c.Query("instance_type"); instanceType != "" {
		filters["instanceType"] = instanceType
	}
	if idStr := c.Query("cloud_account_id"); idStr != "" {
		filters["cloud_account_id"] = idStr
	}
	pricing, err := h.service.GetCloudPricing(cloudType, filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}
	out := make([]gin.H, 0, len(pricing))
	for _, p := range pricing {
		out = append(out, gin.H{
			"cloud_type":     p.CloudType,
			"service_code":   p.ServiceCode,
			"region":         p.Region,
			"instance_type":  p.InstanceType,
			"price_per_unit": p.PricePerUnit.String(),
			"currency":       p.Currency,
			"unit":           p.Unit,
			"sku":            p.SKU,
		})
	}
	c.JSON(http.StatusOK, model.Success(out))
}

// GetBreakdownByTags 按标签分解费用
func (h *BillHandler) GetBreakdownByTags(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")

	result, err := h.service.GetBreakdownByTags(vendor, month)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetBreakdownByAccounts 按云账户分解费用
func (h *BillHandler) GetBreakdownByAccounts(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")

	result, err := h.service.GetBreakdownByAccounts(vendor, month)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetBreakdownByRegion 按区域分解费用
func (h *BillHandler) GetBreakdownByRegion(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")

	result, err := h.service.GetBreakdownByRegion(vendor, month)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// GetBreakdownByService 按服务分解费用
func (h *BillHandler) GetBreakdownByService(c *gin.Context) {
	vendor := c.DefaultQuery("vendor", "")
	month := c.DefaultQuery("month", "")

	result, err := h.service.GetBreakdownByService(vendor, month)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// TriggerSync 手动触发账单同步
// @Summary 手动触发账单同步
// @Description 手动触发指定云账户的账单同步
// @Tags bill
// @Accept json
// @Produce json
// @Param cloud_account_id query string true "云账户ID"
// @Param billing_date query string false "账单日期 (格式: 2024-01-01)"
// @Success 200 {object} model.Response
// @Router /api/bill/trigger-sync [post]
func (h *BillHandler) TriggerSync(c *gin.Context) {
	cloudAccountIDStr := c.Query("cloud_account_id")
	if cloudAccountIDStr == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "cloud_account_id is required"))
		return
	}

	cloudAccountID, err := strconv.ParseUint(cloudAccountIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid cloud_account_id"))
		return
	}

	billingDateStr := c.DefaultQuery("billing_date", time.Now().Format("2006-01-02"))
	billingDate, err := time.Parse("2006-01-02", billingDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid billing_date format"))
		return
	}

	h.service.SyncCloudBillingAsync(uint(cloudAccountID), billingDate)
	c.JSON(http.StatusOK, model.Success(gin.H{"async": true, "message": "sync triggered successfully"}))
}


