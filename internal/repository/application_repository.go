package repository

import (
	"fmt"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApplicationRepository interface {
	Create(app *model.Application) error
	Update(app *model.Application) error
	Delete(id string) error
	FindByID(id string) (*model.Application, error)
	FindAll() ([]model.Application, error)
	FindByOrg(org string) ([]model.Application, error)
	FindByDepartment(department string) ([]model.Application, error)
	FindByStatus(status string) ([]model.Application, error)
	FindBySrvType(srvType string) ([]model.Application, error)
	Search(params map[string]interface{}) ([]model.Application, error)
	SearchWithUserFilter(params map[string]interface{}, userID, username string, isAdmin bool) ([]model.Application, error)
	SearchWithUserFilterPaginated(params map[string]interface{}, userID, username string, isAdmin bool, page, pageSize int) ([]model.Application, int64, error)
	CheckNameExists(name string, excludeID string) (bool, error)
	FindByGitURL(repoURL string) (*model.Application, error)
}

type applicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{db: db}
}

func (r *applicationRepository) Create(app *model.Application) error {
	return r.db.Create(app).Error
}

func (r *applicationRepository) Update(app *model.Application) error {
	return r.db.Model(&model.Application{}).
		Where("id = ?", app.ID).
		Omit("created_at").
		Updates(app).Error
}

func (r *applicationRepository) Delete(id string) error {
	return r.db.Delete(&model.Application{}, "id = ?", id).Error
}

func (r *applicationRepository) FindByID(id string) (*model.Application, error) {
	var app model.Application
	err := r.db.Where("id = ?", id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *applicationRepository) FindAll() ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) FindByOrg(org string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("org = ?", org).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) FindByDepartment(department string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("department = ?", department).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) FindByStatus(status string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("status = ?", status).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) FindBySrvType(srvType string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("srv_type = ?", srvType).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) Search(params map[string]interface{}) ([]model.Application, error) {
	var apps []model.Application
	query := r.db.Model(&model.Application{})

	if name, ok := params["name"].(string); ok && name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if org, ok := params["org"].(string); ok && org != "" {
		query = query.Where("org = ?", org)
	}
	if department, ok := params["department"].(string); ok && department != "" {
		query = query.Where("department = ?", department)
	}
	if status, ok := params["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if srvType, ok := params["srvType"].(string); ok && srvType != "" {
		query = query.Where("srv_type = ?", srvType)
	}
	if virtualTech, ok := params["virtualTech"].(string); ok && virtualTech != "" {
		query = query.Where("virtual_tech = ?", virtualTech)
	}
	if site, ok := params["site"].(string); ok && site != "" {
		query = query.Where("site = ?", site)
	}
	if isCritical, ok := params["isCritical"].(bool); ok {
		query = query.Where("is_critical = ?", isCritical)
	}

	err := query.Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) SearchWithUserFilter(params map[string]interface{}, userID, username string, isAdmin bool) ([]model.Application, error) {
	var apps []model.Application
	query := r.db.Model(&model.Application{})

	if name, ok := params["name"].(string); ok && name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if org, ok := params["org"].(string); ok && org != "" {
		query = query.Where("org = ?", org)
	}
	if department, ok := params["department"].(string); ok && department != "" {
		query = query.Where("department = ?", department)
	}
	if status, ok := params["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if srvType, ok := params["srvType"].(string); ok && srvType != "" {
		query = query.Where("srv_type = ?", srvType)
	}
	if virtualTech, ok := params["virtualTech"].(string); ok && virtualTech != "" {
		query = query.Where("virtual_tech = ?", virtualTech)
	}
	if site, ok := params["site"].(string); ok && site != "" {
		query = query.Where("site = ?", site)
	}
	if isCritical, ok := params["isCritical"].(bool); ok {
		query = query.Where("is_critical = ?", isCritical)
	}

	if !isAdmin && userID != "" {
		query = r.addUserFilter(query, userID, username)
	}

	err := query.Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *applicationRepository) SearchWithUserFilterPaginated(params map[string]interface{}, userID, username string, isAdmin bool, page, pageSize int) ([]model.Application, int64, error) {
	query := r.db.Model(&model.Application{})

	if name, ok := params["name"].(string); ok && name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if org, ok := params["org"].(string); ok && org != "" {
		query = query.Where("org = ?", org)
	}
	if department, ok := params["department"].(string); ok && department != "" {
		query = query.Where("department = ?", department)
	}
	if status, ok := params["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if srvType, ok := params["srvType"].(string); ok && srvType != "" {
		query = query.Where("srv_type = ?", srvType)
	}
	if virtualTech, ok := params["virtualTech"].(string); ok && virtualTech != "" {
		query = query.Where("virtual_tech = ?", virtualTech)
	}
	if site, ok := params["site"].(string); ok && site != "" {
		query = query.Where("site = ?", site)
	}
	if isCritical, ok := params["isCritical"].(bool); ok {
		query = query.Where("is_critical = ?", isCritical)
	}

	if !isAdmin && userID != "" {
		query = r.addUserFilter(query, userID, username)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var apps []model.Application
	err := query.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}

func (r *applicationRepository) addUserFilter(query *gorm.DB, userID string, username string) *gorm.DB {
	useUsername := username != "" && username != userID
	fields := []string{"ops_owners", "test_owners", "dev_owners"}

	if r.db.Dialector.Name() == "postgres" {
		userIDArray := fmt.Sprintf(`["%s"]`, userID)
		usernameArray := fmt.Sprintf(`["%s"]`, username)
		var orParts []string
		var args []interface{}
		for _, f := range fields {
			if useUsername {
				orParts = append(orParts, fmt.Sprintf("(%s::jsonb @> ? OR %s::jsonb @> ?)", f, f))
				args = append(args, userIDArray, usernameArray)
			} else {
				orParts = append(orParts, fmt.Sprintf("%s::jsonb @> ?", f))
				args = append(args, userIDArray)
			}
		}
		return query.Where(strings.Join(orParts, " OR "), args...)
	}
	// MySQL
	userIDJSON := fmt.Sprintf(`"%s"`, userID)
	usernameJSON := fmt.Sprintf(`"%s"`, username)
	var orParts []string
	var args []interface{}
	for _, f := range fields {
		if useUsername {
			orParts = append(orParts, fmt.Sprintf("(JSON_CONTAINS(%s, ?) OR JSON_CONTAINS(%s, ?))", f, f))
			args = append(args, userIDJSON, usernameJSON)
		} else {
			orParts = append(orParts, fmt.Sprintf("JSON_CONTAINS(%s, ?)", f))
			args = append(args, userIDJSON)
		}
	}
	return query.Where(strings.Join(orParts, " OR "), args...)
}

func (r *applicationRepository) CheckNameExists(name string, excludeID string) (bool, error) {
	var count int64
	query := r.db.Model(&model.Application{}).Where("name = ?", name)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func normalizeGitURL(u string) string {
	s := strings.TrimSpace(u)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	return s
}

func (r *applicationRepository) FindByGitURL(repoURL string) (*model.Application, error) {
	norm := normalizeGitURL(repoURL)
	if norm == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var apps []model.Application
	err := r.db.Where("git_url != ''").Find(&apps).Error
	if err != nil {
		return nil, err
	}
	for i := range apps {
		if normalizeGitURL(apps[i].GitURL) == norm {
			return &apps[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
