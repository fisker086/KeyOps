package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type DBPermissionRepository struct {
	db *gorm.DB
}

func NewDBPermissionRepository(db *gorm.DB) *DBPermissionRepository {
	return &DBPermissionRepository{db: db}
}

// Create 创建权限元数据
func (r *DBPermissionRepository) Create(metadata *model.DBPermissionMetadata) error {
	return r.db.Create(metadata).Error
}

// Delete 删除权限元数据
func (r *DBPermissionRepository) Delete(userID string, instanceID uint, databaseName, tableName, permissionType string) error {
	query := r.db.Where("user_id = ? AND instance_id = ? AND permission_type = ?", userID, instanceID, permissionType)
	if databaseName != "" {
		query = query.Where("database_name = ?", databaseName)
	} else {
		query = query.Where("database_name IS NULL OR database_name = ''")
	}
	if tableName != "" {
		query = query.Where("table_name = ?", tableName)
	} else {
		query = query.Where("table_name IS NULL OR table_name = ''")
	}
	return query.Delete(&model.DBPermissionMetadata{}).Error
}

// Get 获取权限元数据
func (r *DBPermissionRepository) Get(userID string, instanceID uint, databaseName, tableName, permissionType string) (*model.DBPermissionMetadata, error) {
	var metadata model.DBPermissionMetadata
	query := r.db.Where("user_id = ? AND instance_id = ? AND permission_type = ?", userID, instanceID, permissionType)
	if databaseName != "" {
		query = query.Where("database_name = ?", databaseName)
	} else {
		query = query.Where("database_name IS NULL OR database_name = ''")
	}
	if tableName != "" {
		query = query.Where("table_name = ?", tableName)
	} else {
		query = query.Where("table_name IS NULL OR table_name = ''")
	}
	err := query.First(&metadata).Error
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

// List 获取权限列表
func (r *DBPermissionRepository) List(filters map[string]interface{}) ([]model.DBPermissionMetadata, error) {
	var metadatas []model.DBPermissionMetadata
	query := r.db.Model(&model.DBPermissionMetadata{})

	if userID, ok := filters["user_id"].(string); ok && userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if instanceID, ok := filters["instance_id"].(uint); ok && instanceID > 0 {
		query = query.Where("instance_id = ?", instanceID)
	}

	err := query.Order("created_at DESC").Find(&metadatas).Error
	return metadatas, err
}

// BatchCreate 批量创建
func (r *DBPermissionRepository) BatchCreate(metadatas []*model.DBPermissionMetadata) error {
	if len(metadatas) == 0 {
		return nil
	}
	return r.db.Create(metadatas).Error
}

// Update 更新权限元数据
func (r *DBPermissionRepository) Update(userID string, instanceID uint, databaseName, tableName, permissionType string, updates map[string]interface{}) error {
	query := r.db.Model(&model.DBPermissionMetadata{}).Where("user_id = ? AND instance_id = ? AND permission_type = ?", userID, instanceID, permissionType)
	if databaseName != "" {
		query = query.Where("database_name = ?", databaseName)
	} else {
		query = query.Where("database_name IS NULL OR database_name = ''")
	}
	if tableName != "" {
		query = query.Where("table_name = ?", tableName)
	} else {
		query = query.Where("table_name IS NULL OR table_name = ''")
	}
	return query.Updates(updates).Error
}
