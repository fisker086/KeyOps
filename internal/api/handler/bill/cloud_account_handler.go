package bill

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/model"
	billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type CloudAccountHandler struct {
	service *billService.BillService
}

func NewCloudAccountHandler(service *billService.BillService) *CloudAccountHandler {
	return &CloudAccountHandler{service: service}
}

// ListCloudAccounts 列出云账户（含费用详情）
// @Summary 列出云账户
// @Description 获取云账户列表，含资源数、最近导入与状态等详情
// @Tags bill
// @Accept json
// @Produce json
// @Param type query string false "云类型筛选"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts [get]
func (h *CloudAccountHandler) ListCloudAccounts(c *gin.Context) {
	cloudType := c.Query("type")

	accounts, err := h.service.ListCloudAccountsWithDetails(cloudType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(accounts))
}

// GetCloudAccount 获取云账户详情
// @Summary 获取云账户详情
// @Description 获取单个云账户的详细信息
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "云账户ID"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts/:id [get]
func (h *CloudAccountHandler) GetCloudAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	account, err := h.service.GetCloudAccountWithDetails(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(account))
}

// AddCloudAccount 添加云账户
// @Summary 添加云账户
// @Description 创建新的云账户
// @Tags bill
// @Accept json
// @Produce json
// @Param account body model.CloudAccount true "云账户信息"
// @Success 201 {object} model.Response
// @Router /api/bill/cloud-accounts [post]
func (h *CloudAccountHandler) AddCloudAccount(c *gin.Context) {
	var account model.CloudAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.AddCloudAccount(&account); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	stripCloudAccountSecret(&account)
	c.JSON(http.StatusCreated, model.Success(account))
}

// UpdateCloudAccount 更新云账户
// @Summary 更新云账户
// @Description 更新云账户信息（密钥为空则保留原值）
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "云账户ID"
// @Param account body model.CloudAccount true "云账户信息"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts/:id [put]
func (h *CloudAccountHandler) UpdateCloudAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	var patch model.CloudAccount
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	if err := h.service.UpdateCloudAccount(uint(id), &patch); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	account, err := h.service.GetCloudAccount(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, model.Success(gin.H{"updated": true}))
		return
	}
	stripCloudAccountSecret(account)
	c.JSON(http.StatusOK, model.Success(account))
}

// DeleteCloudAccount 删除云账户
// @Summary 删除云账户
// @Description 删除指定的云账户
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "云账户ID"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts/:id [delete]
func (h *CloudAccountHandler) DeleteCloudAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	if err := h.service.DeleteCloudAccount(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"deleted": true}))
}

type ValidateCloudAccountReq struct {
	CloudType       string `json:"cloud_type" binding:"required"`
	AccessKeyID     string `json:"access_key_id" binding:"required"`
	SecretAccessKey string `json:"secret_access_key" binding:"required"`
	Region          string `json:"region"`
	BucketName      string `json:"bucket_name"`
	BucketPrefix    string `json:"bucket_prefix"`
	ReportName      string `json:"report_name"`
}

// ValidateCloudAccount 验证云账户凭证
// @Summary 验证云账户凭证
// @Description 验证云账户凭证是否有效（不落库）
// @Tags bill
// @Accept json
// @Produce json
// @Param body body ValidateCloudAccountReq true "凭证信息"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts/validate [post]
func (h *CloudAccountHandler) ValidateCloudAccount(c *gin.Context) {
	var req ValidateCloudAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	config := map[string]interface{}{
		"access_key_id":     req.AccessKeyID,
		"secret_access_key": req.SecretAccessKey,
		"region":            req.Region,
		"bucket_name":       req.BucketName,
		"bucket_prefix":     req.BucketPrefix,
		"report_name":       req.ReportName,
	}

	result, err := h.service.ValidateCloudAccount(req.CloudType, config)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "validation failed: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// SyncCloudAccount 同步云账户账单
// @Summary 同步云账户账单
// @Description 从云厂商同步账单数据
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "云账户ID"
// @Success 200 {object} model.Response
// @Router /api/bill/cloud-accounts/:id/sync [post]
func (h *CloudAccountHandler) SyncCloudAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid cloud_account_id"))
		return
	}

	startMonthStr := c.Query("start_month")
	endMonthStr := c.Query("end_month")

	if startMonthStr != "" && endMonthStr != "" {
		startDate, err := time.Parse("2006-01", startMonthStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid start_month format (use YYYY-MM)"))
			return
		}
		endDate, err := time.Parse("2006-01", endMonthStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid end_month format (use YYYY-MM)"))
			return
		}

		if startDate.After(endDate) {
			c.JSON(http.StatusBadRequest, model.Error(400, "start_month must be before end_month"))
			return
		}

		h.service.SyncCloudBillingRange(uint(id), startDate, endDate)
		c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "async": true, "synced_months": fmt.Sprintf("%s~%s", startMonthStr, endMonthStr)}))
		return
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid start_date format (use YYYY-MM-DD)"))
			return
		}
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid end_date format (use YYYY-MM-DD)"))
			return
		}

		if startDate.After(endDate) {
			c.JSON(http.StatusBadRequest, model.Error(400, "start_date must be before end_date"))
			return
		}

		h.service.SyncCloudBillingRange(uint(id), startDate, endDate)
		c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "async": true, "synced_months": fmt.Sprintf("%s~%s", startDateStr, endDateStr)}))
		return
	}

	billingDate := time.Now()
	if monthStr := c.Query("month"); monthStr != "" {
		billingDate, err = time.Parse("2006-01", monthStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid month format (use YYYY-MM)"))
			return
		}
	} else if dateStr := c.Query("date"); dateStr != "" {
		billingDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid date format"))
			return
		}
	} else {
		billingDate = time.Now().AddDate(0, -1, 0)
	}

	h.service.SyncCloudBillingAsync(uint(id), billingDate)
	c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "async": true, "synced_at": billingDate}))
}

// CancelSync 取消正在同步中的账单
// @Summary 取消同步
// @Tags bill
// @Success 200
// @Router /api/bill/cloud-accounts/:id/sync/cancel [post]
func (h *CloudAccountHandler) CancelSync(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid cloud_account_id"))
		return
	}

	billingDate := time.Now()
	if monthStr := c.Query("month"); monthStr != "" {
		billingDate, err = time.Parse("2006-01", monthStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid month format"))
			return
		}
	}

	if err := h.service.CancelSync(uint(id), billingDate); err != nil {
		c.JSON(http.StatusNotFound, model.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(gin.H{"ok": true, "cancelled": true}))
}
