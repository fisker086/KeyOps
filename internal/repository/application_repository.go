package repository

import (
	"fmt"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type ApplicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) *ApplicationRepository {
	return &ApplicationRepository{db: db}
}

// Create 创建应用
func (r *ApplicationRepository) Create(app *model.Application) error {
	return r.db.Create(app).Error
}

// Update 更新应用
func (r *ApplicationRepository) Update(app *model.Application) error {
	return r.db.Model(&model.Application{}).
		Where("id = ?", app.ID).
		Omit("created_at").
		Updates(app).Error
}

// Delete 删除应用
func (r *ApplicationRepository) Delete(id string) error {
	return r.db.Delete(&model.Application{}, "id = ?", id).Error
}

// FindByID 根据ID查找应用
func (r *ApplicationRepository) FindByID(id string) (*model.Application, error) {
	var app model.Application
	err := r.db.Where("id = ?", id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// FindAll 查找所有应用
func (r *ApplicationRepository) FindAll() ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

// FindByOrg 根据事业部查找应用
func (r *ApplicationRepository) FindByOrg(org string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("org = ?", org).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

// FindByDepartment 根据部门查找应用
func (r *ApplicationRepository) FindByDepartment(department string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("department = ?", department).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

// FindByStatus 根据状态查找应用
func (r *ApplicationRepository) FindByStatus(status string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("status = ?", status).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

// FindBySrvType 根据应用类型查找应用
func (r *ApplicationRepository) FindBySrvType(srvType string) ([]model.Application, error) {
	var apps []model.Application
	err := r.db.Where("srv_type = ?", srvType).Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

// Search 搜索应用（支持多条件）
func (r *ApplicationRepository) Search(params map[string]interface{}) ([]model.Application, error) {
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

// SearchWithUserFilter 搜索应用（支持多条件和用户权限过滤）
// userID: 当前用户ID；username: 当前用户名（前端可能存的是用户名，两者任一匹配即可）
// isAdmin: 是否为管理员，管理员可以看到所有应用
func (r *ApplicationRepository) SearchWithUserFilter(params map[string]interface{}, userID, username string, isAdmin bool) ([]model.Application, error) {
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

// SearchWithUserFilterPaginated 分页搜索应用（支持多条件、用户权限过滤）
// 返回 list 和 total；userID/username 用于负责人匹配（前端可能存的是用户名）
func (r *ApplicationRepository) SearchWithUserFilterPaginated(params map[string]interface{}, userID, username string, isAdmin bool, page, pageSize int) ([]model.Application, int64, error) {
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

// addUserFilter 添加用户过滤条件（检查用户是否在运维/测试/研发负责人中）
// 同时支持 userID（JWT 中的用户 ID）和 username（前端可能存的是用户名），任一匹配即可
func (r *ApplicationRepository) addUserFilter(query *gorm.DB, userID string, username string) *gorm.DB {
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

// CheckNameExists 检查应用名称是否存在
func (r *ApplicationRepository) CheckNameExists(name string, excludeID string) (bool, error) {
	var count int64
	query := r.db.Model(&model.Application{}).Where("name = ?", name)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// normalizeGitURL 规范化 Git URL 便于匹配（去空格、去 .git 后缀、去末尾斜杠）
func normalizeGitURL(u string) string {
	s := strings.TrimSpace(u)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	return s
}

// FindByGitURL 根据仓库 URL 查找应用（匹配 git_url 规范化后的值）
func (r *ApplicationRepository) FindByGitURL(repoURL string) (*model.Application, error) {
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

