package system

import (
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrganizationHandler struct {
	repo *repository.OrganizationRepository
}

func NewOrganizationHandler(repo *repository.OrganizationRepository) *OrganizationHandler {
	return &OrganizationHandler{repo: repo}
}

// ListOrganizations 获取组织列表
// @Summary 获取组织列表
// @Tags organizations
// @Produce json
// @Param tree query boolean false "返回树形结构"
// @Param unitType query string false "组织类型过滤"
// @Success 200 {object} model.Response
// @Router /api/organizations [get]
func (h *OrganizationHandler) ListOrganizations(c *gin.Context) {
	tree := c.Query("tree") == "true"
	unitType := c.Query("unitType")

	var orgs []model.Organization
	var err error

	if unitType != "" {
		orgs, err = h.repo.FindByUnitType(unitType)
	} else {
		orgs, err = h.repo.FindAll()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch organizations",
			Data:    nil,
		})
		return
	}

	if tree {
		orgs = h.repo.BuildOrganizationTree(orgs)
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    orgs,
	})
}

// GetOrganization 获取单个组织
// @Summary 获取单个组织
// @Tags organizations
// @Produce json
// @Param id path string true "Organization ID"
// @Success 200 {object} model.Response
// @Router /api/organizations/{id} [get]
func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	id := c.Param("id")

	org, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "Organization not found",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    org,
	})
}

// CreateOrganization 创建组织
// @Summary 创建组织
// @Tags organizations
// @Accept json
// @Produce json
// @Param organization body model.CreateOrganizationRequest true "Organization"
// @Success 200 {object} model.Response
// @Router /api/organizations [post]
func (h *OrganizationHandler) CreateOrganization(c *gin.Context) {
	var req model.CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 检查组织标识符是否已存在
	exists, err := h.repo.CheckUnitCodeExists(req.UnitCode, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to check unit code",
			Data:    nil,
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Unit code already exists",
			Data:    nil,
		})
		return
	}

	// 如果指定了父组织，验证父组织是否存在
	if req.ParentID != nil && *req.ParentID != "" {
		_, err := h.repo.FindByID(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "Parent organization not found",
				Data:    nil,
			})
			return
		}
	}

	org := &model.Organization{
		ID:          uuid.New().String(),
		UnitCode:    req.UnitCode,
		UnitName:    req.UnitName,
		UnitType:    req.UnitType,
		UnitOwner:   req.UnitOwner,
		IsActive:    req.IsActive,
		ParentID:    req.ParentID,
		SortOrder:   req.SortOrder,
		Description: req.Description,
	}

	if err := h.repo.Create(org); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create organization: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Organization created successfully",
		Data:    org,
	})
}

// UpdateOrganization 更新组织
// @Summary 更新组织
// @Tags organizations
// @Accept json
// @Produce json
// @Param id path string true "Organization ID"
// @Param organization body model.UpdateOrganizationRequest true "Organization"
// @Success 200 {object} model.Response
// @Router /api/organizations/{id} [put]
func (h *OrganizationHandler) UpdateOrganization(c *gin.Context) {
	id := c.Param("id")

	var req model.UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body: " + err.Error(),
			Data:    nil,
		})
		return
	}

	// 检查组织是否存在
	org, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "Organization not found",
			Data:    nil,
		})
		return
	}

	// 如果指定了父组织，验证父组织是否存在且不是自己，且不会产生循环引用
	if req.ParentID != nil && *req.ParentID != "" {
		if *req.ParentID == id {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "Cannot set parent to itself",
				Data:    nil,
			})
			return
		}
		_, err := h.repo.FindByID(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "Parent organization not found",
				Data:    nil,
			})
			return
		}
		// 检查是否会产生循环引用（不能将子组织设置为父组织）
		isDescendant, err := h.repo.IsDescendant(id, *req.ParentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{
				Code:    http.StatusInternalServerError,
				Message: "Failed to check circular reference: " + err.Error(),
				Data:    nil,
			})
			return
		}
		if isDescendant {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "Cannot set parent to a descendant organization (would create circular reference)",
				Data:    nil,
			})
			return
		}
	}

	// 更新组织信息
	org.UnitName = req.UnitName
	org.UnitType = req.UnitType
	org.UnitOwner = req.UnitOwner
	org.IsActive = req.IsActive
	org.ParentID = req.ParentID
	org.SortOrder = req.SortOrder
	org.Description = req.Description

	if err := h.repo.Update(org); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update organization: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Organization updated successfully",
		Data:    org,
	})
}

// DeleteOrganization 删除组织
// @Summary 删除组织
// @Tags organizations
// @Produce json
// @Param id path string true "Organization ID"
// @Success 200 {object} model.Response
// @Router /api/organizations/{id} [delete]
func (h *OrganizationHandler) DeleteOrganization(c *gin.Context) {
	id := c.Param("id")

	// 检查组织是否存在
	_, err := h.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Code:    http.StatusNotFound,
			Message: "Organization not found",
			Data:    nil,
		})
		return
	}

	if err := h.repo.Delete(id); err != nil {
		if err.Error() == "cannot delete organization with children" {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    http.StatusBadRequest,
				Message: "Cannot delete organization with children",
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete organization: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Organization deleted successfully",
		Data:    nil,
	})
}

// GetOrganizationTree 获取组织树
// @Summary 获取组织树
// @Tags organizations
// @Produce json
// @Success 200 {object} model.Response
// @Router /api/organizations/tree [get]
func (h *OrganizationHandler) GetOrganizationTree(c *gin.Context) {
	orgs, err := h.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    http.StatusInternalServerError,
			Message: "Failed to fetch organizations",
			Data:    nil,
		})
		return
	}

	tree := h.repo.BuildOrganizationTree(orgs)

	c.JSON(http.StatusOK, model.Response{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    tree,
	})
}

