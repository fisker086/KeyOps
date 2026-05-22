package bill

import (
	"net/http"
	"strconv"

	"github.com/fisker086/keyops/internal/model"
	billService "github.com/fisker086/keyops/internal/service/bill"
	"github.com/gin-gonic/gin"
)

type FinOpsHandler struct {
	service *billService.BillService
}

func NewFinOpsHandler(service *billService.BillService) *FinOpsHandler {
	return &FinOpsHandler{service: service}
}

// ============ Budgets ============

// ListBudgets 获取预算列表
// @Summary 获取预算列表
// @Description 获取所有预算及当前状态
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/budgets [get]
func (h *FinOpsHandler) ListBudgets(c *gin.Context) {
	result, err := h.service.ListBudgets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// CreateBudget 创建预算
// @Summary 创建预算
// @Description 创建新的预算
// @Tags bill
// @Accept json
// @Produce json
// @Param budget body model.Budget true "预算信息"
// @Success 201 {object} model.Response
// @Router /api/bill/budgets [post]
func (h *FinOpsHandler) CreateBudget(c *gin.Context) {
	var budget model.Budget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.CreateBudget(&budget)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, model.Success(result))
}

// UpdateBudget 更新预算
// @Summary 更新预算
// @Description 更新预算信息
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "预算ID"
// @Param budget body model.Budget true "预算信息"
// @Success 200 {object} model.Response
// @Router /api/bill/budgets/:id [put]
func (h *FinOpsHandler) UpdateBudget(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	var budget model.Budget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.UpdateBudget(uint(id), &budget)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// DeleteBudget 删除预算
// @Summary 删除预算
// @Description 删除指定的预算
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "预算ID"
// @Success 200 {object} model.Response
// @Router /api/bill/budgets/:id [delete]
func (h *FinOpsHandler) DeleteBudget(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	if err := h.service.DeleteBudget(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"deleted": true}))
}

// ============ Pools ============

// ListPools 获取资源池列表
// @Summary 获取资源池列表
// @Description 获取所有资源池及费用信息
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/pools [get]
func (h *FinOpsHandler) ListPools(c *gin.Context) {
	result, err := h.service.ListPools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// CreatePool 创建资源池
// @Summary 创建资源池
// @Description 创建新的资源池
// @Tags bill
// @Accept json
// @Produce json
// @Param pool body model.Pool true "资源池信息"
// @Success 201 {object} model.Response
// @Router /api/bill/pools [post]
func (h *FinOpsHandler) CreatePool(c *gin.Context) {
	var pool model.Pool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.CreatePool(&pool)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, model.Success(result))
}

// UpdatePool 更新资源池
// @Summary 更新资源池
// @Description 更新资源池信息
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "资源池ID"
// @Param pool body model.Pool true "资源池信息"
// @Success 200 {object} model.Response
// @Router /api/bill/pools/:id [put]
func (h *FinOpsHandler) UpdatePool(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	var pool model.Pool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.UpdatePool(uint(id), &pool)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// DeletePool 删除资源池
// @Summary 删除资源池
// @Description 删除指定的资源池
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "资源池ID"
// @Success 200 {object} model.Response
// @Router /api/bill/pools/:id [delete]
func (h *FinOpsHandler) DeletePool(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	if err := h.service.DeletePool(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"deleted": true}))
}

// ============ Policies ============

// ListPolicies 获取策略列表
// @Summary 获取策略列表
// @Description 获取所有策略（TTL、电源管理等）
// @Tags bill
// @Accept json
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/bill/policies [get]
func (h *FinOpsHandler) ListPolicies(c *gin.Context) {
	result, err := h.service.ListPolicies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.Success(result))
}

// CreatePolicy 创建策略
// @Summary 创建策略
// @Description 创建新的策略
// @Tags bill
// @Accept json
// @Produce json
// @Param policy body model.Policy true "策略信息"
// @Success 201 {object} model.Response
// @Router /api/bill/policies [post]
func (h *FinOpsHandler) CreatePolicy(c *gin.Context) {
	var policy model.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.CreatePolicy(&policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, model.Success(result))
}

// UpdatePolicy 更新策略
// @Summary 更新策略
// @Description 更新策略信息
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Param policy body model.Policy true "策略信息"
// @Success 200 {object} model.Response
// @Router /api/bill/policies/:id [put]
func (h *FinOpsHandler) UpdatePolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	var policy model.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	result, err := h.service.UpdatePolicy(uint(id), &policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// DeletePolicy 删除策略
// @Summary 删除策略
// @Description 删除指定的策略
// @Tags bill
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Success 200 {object} model.Response
// @Router /api/bill/policies/:id [delete]
func (h *FinOpsHandler) DeletePolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "invalid id"))
		return
	}

	if err := h.service.DeletePolicy(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, model.Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.Success(gin.H{"deleted": true}))
}
