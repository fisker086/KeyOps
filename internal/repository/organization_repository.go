package repository

import (
	"errors"
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type OrganizationRepository struct {
	db *gorm.DB
}

func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// Create 创建组织
func (r *OrganizationRepository) Create(org *model.Organization) error {
	return r.db.Create(org).Error
}

// Update 更新组织
func (r *OrganizationRepository) Update(org *model.Organization) error {
	return r.db.Model(&model.Organization{}).
		Where("id = ?", org.ID).
		Omit("created_at", "unit_code").
		Updates(org).Error
}

// Delete 删除组织
func (r *OrganizationRepository) Delete(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 检查是否有子组织
		var count int64
		if err := tx.Model(&model.Organization{}).
			Where("parent_id = ?", id).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("cannot delete organization with children")
		}
		// 删除组织
		return tx.Delete(&model.Organization{}, "id = ?", id).Error
	})
}

// FindByID 根据ID查找组织
func (r *OrganizationRepository) FindByID(id string) (*model.Organization, error) {
	var org model.Organization
	err := r.db.Where("id = ?", id).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// FindByUnitCode 根据组织标识符查找组织
func (r *OrganizationRepository) FindByUnitCode(unitCode string) (*model.Organization, error) {
	var org model.Organization
	err := r.db.Where("unit_code = ?", unitCode).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// FindAll 查找所有组织
func (r *OrganizationRepository) FindAll() ([]model.Organization, error) {
	var orgs []model.Organization
	err := r.db.Order("sort_order ASC, created_at ASC").Find(&orgs).Error
	return orgs, err
}

// FindByParentID 根据父组织ID查找子组织
func (r *OrganizationRepository) FindByParentID(parentID *string) ([]model.Organization, error) {
	var orgs []model.Organization
	query := r.db
	if parentID == nil || *parentID == "" {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	err := query.Order("sort_order ASC").Find(&orgs).Error
	return orgs, err
}

// FindByUnitType 根据组织类型查找组织
func (r *OrganizationRepository) FindByUnitType(unitType string) ([]model.Organization, error) {
	var orgs []model.Organization
	err := r.db.Where("unit_type = ?", unitType).
		Order("sort_order ASC").
		Find(&orgs).Error
	return orgs, err
}

// BuildOrganizationTree 构建组织树
func (r *OrganizationRepository) BuildOrganizationTree(orgs []model.Organization) []model.Organization {
	if len(orgs) == 0 {
		return []model.Organization{}
	}

	orgMap := make(map[string]*model.Organization)
	var rootOrgs []model.Organization

	// 第一遍遍历，创建组织映射并初始化Children
	for i := range orgs {
		orgs[i].Children = []model.Organization{}
		orgMap[orgs[i].ID] = &orgs[i]
	}

	// 第二遍遍历，构建树结构
	for i := range orgs {
		if orgs[i].ParentID == nil || *orgs[i].ParentID == "" {
			// 顶级组织
			rootOrgs = append(rootOrgs, orgs[i])
		} else {
			// 子组织，添加到父组织的Children中
			if parent, ok := orgMap[*orgs[i].ParentID]; ok {
				parent.Children = append(parent.Children, orgs[i])
			}
		}
	}

	return rootOrgs
}

// CheckUnitCodeExists 检查组织标识符是否存在
func (r *OrganizationRepository) CheckUnitCodeExists(unitCode string, excludeID string) (bool, error) {
	var count int64
	query := r.db.Model(&model.Organization{}).Where("unit_code = ?", unitCode)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// IsDescendant 检查 targetID 是否是 ancestorID 的后代（用于防止循环引用）
func (r *OrganizationRepository) IsDescendant(ancestorID, targetID string) (bool, error) {
	if ancestorID == targetID {
		return true, nil
	}

	// 获取目标组织的所有祖先
	currentID := targetID
	visited := make(map[string]bool)
	maxDepth := 100 // 防止无限循环

	for depth := 0; depth < maxDepth; depth++ {
		if visited[currentID] {
			// 检测到循环引用
			return false, errors.New("circular reference detected")
		}
		visited[currentID] = true

		org, err := r.FindByID(currentID)
		if err != nil {
			return false, err
		}

		if org.ParentID == nil || *org.ParentID == "" {
			// 到达根节点
			return false, nil
		}

		if *org.ParentID == ancestorID {
			// 找到祖先
			return true, nil
		}

		currentID = *org.ParentID
	}

	return false, errors.New("max depth exceeded")
}

