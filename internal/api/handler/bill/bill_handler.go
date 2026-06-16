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

// RebuildAggregates 从 bill_records 重建日费用与 Dashboard 预聚合（无需重新下载 CUR）
func (h *BillHandler) RebuildAggregates(c *gin.Context) {
	cycle := c.Query("cycle")
	if cycle == "" {
		c.JSON(http.StatusBadRequest, model.Error(400, "cycle is required (format: 2026-05)"))
		return
	}
	if _, err := time.Parse("2006-01", cycle); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid cycle format, use YYYY-MM"))
		return
	}
	if err := h.service.RebuildBillAggregates(cycle); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"cycle": cycle, "message": "aggregates rebuilt"}))
}
