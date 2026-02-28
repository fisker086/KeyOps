package dms

import (
	"context"
	"errors"
	"fmt"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/crypto"
)

// ErrInstanceNameExists 实例名称已存在（用于返回 400）
var ErrInstanceNameExists = errors.New("实例名称已存在")

type InstanceService struct {
	instanceRepo *repository.DBInstanceRepository
	crypto       *crypto.Crypto
}

func NewInstanceService(
	instanceRepo *repository.DBInstanceRepository,
	crypto *crypto.Crypto,
) *InstanceService {
	return &InstanceService{
		instanceRepo: instanceRepo,
		crypto:       crypto,
	}
}

// CreateInstance 创建数据库实例
func (s *InstanceService) CreateInstance(req *CreateInstanceRequest, createdBy string) (*model.DBInstance, error) {
	// 名称不能重复
	exists, err := s.instanceRepo.ExistsByName(req.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("检查实例名称失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: %s", ErrInstanceNameExists, req.Name)
	}

	// 加密密码
	encryptedPassword, err := s.crypto.Encrypt(req.Password)
	if err != nil {
		return nil, fmt.Errorf("加密密码失败: %w", err)
	}

	instance := &model.DBInstance{
		Name:            req.Name,
		DBType:          req.DBType,
		Host:            req.Host,
		Port:            req.Port,
		Username:        req.Username,
		Password:        encryptedPassword,
		DatabaseName:    req.DatabaseName,
		AuthDatabase:    req.AuthDatabase,
		Charset:         req.Charset,
		ConnectionString: req.ConnectionString,
		SSLEnabled:      req.SSLEnabled,
		SSLCert:         req.SSLCert,
		Description:     req.Description,
		IsEnabled:       true,
		CreatedBy:       createdBy,
	}

	if err := s.instanceRepo.Create(instance); err != nil {
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}

	// 不返回密码
	instance.Password = ""
	return instance, nil
}

// UpdateInstance 更新数据库实例
func (s *InstanceService) UpdateInstance(id uint, req *UpdateInstanceRequest, updatedBy string) (*model.DBInstance, error) {
	instance, err := s.instanceRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("实例不存在: %w", err)
	}

	// 若修改了名称，检查新名称是否与其他实例重复
	if req.Name != "" && req.Name != instance.Name {
		exists, err := s.instanceRepo.ExistsByName(req.Name, &id)
		if err != nil {
			return nil, fmt.Errorf("检查实例名称失败: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("%w: %s", ErrInstanceNameExists, req.Name)
		}
	}

	// 更新字段
	instance.Name = req.Name
	instance.Host = req.Host
	instance.Port = req.Port
	instance.Username = req.Username
	instance.DatabaseName = req.DatabaseName
	instance.AuthDatabase = req.AuthDatabase
	instance.Charset = req.Charset
	instance.ConnectionString = req.ConnectionString
	instance.SSLEnabled = req.SSLEnabled
	instance.SSLCert = req.SSLCert
	instance.Description = req.Description
	instance.IsEnabled = req.IsEnabled

	// 如果提供了新密码，加密并更新
	if req.Password != "" {
		encryptedPassword, err := s.crypto.Encrypt(req.Password)
		if err != nil {
			return nil, fmt.Errorf("加密密码失败: %w", err)
		}
		instance.Password = encryptedPassword
	}

	if err := s.instanceRepo.Update(instance); err != nil {
		return nil, fmt.Errorf("更新实例失败: %w", err)
	}

	// 不返回密码
	instance.Password = ""
	return instance, nil
}

// DeleteInstance 删除数据库实例
func (s *InstanceService) DeleteInstance(id uint) error {
	return s.instanceRepo.Delete(id)
}

// GetInstance 获取实例详情
func (s *InstanceService) GetInstance(id uint) (*model.DBInstance, error) {
	instance, err := s.instanceRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	// 不返回密码
	instance.Password = ""
	return instance, nil
}

// ListInstances 获取实例列表
func (s *InstanceService) ListInstances(offset, limit int, filters map[string]interface{}) ([]model.DBInstance, int64, error) {
	instances, total, err := s.instanceRepo.List(offset, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	// 清除密码
	for i := range instances {
		instances[i].Password = ""
	}

	return instances, total, nil
}

// TestConnection 测试连接（使用 Executor，要求实例已存在）
func (s *InstanceService) TestConnection(id uint) error {
	instance, err := s.instanceRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("实例不存在: %w", err)
	}

	executor, err := NewExecutor(instance, s.crypto)
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := context.Background()
	return executor.TestConnection(ctx)
}

// TestConnectionWithRequest 仅测试连接，不落库（用于新增前“测试连接”）
func (s *InstanceService) TestConnectionWithRequest(req *CreateInstanceRequest) error {
	encryptedPassword, err := s.crypto.Encrypt(req.Password)
	if err != nil {
		return fmt.Errorf("加密密码失败: %w", err)
	}
	instance := &model.DBInstance{
		Name:             req.Name,
		DBType:           req.DBType,
		Host:             req.Host,
		Port:             req.Port,
		Username:         req.Username,
		Password:         encryptedPassword,
		DatabaseName:     req.DatabaseName,
		AuthDatabase:     req.AuthDatabase,
		Charset:          req.Charset,
		ConnectionString: req.ConnectionString,
		SSLEnabled:       req.SSLEnabled,
		SSLCert:          req.SSLCert,
	}
	executor, err := NewExecutor(instance, s.crypto)
	if err != nil {
		return err
	}
	defer executor.Close()
	ctx := context.Background()
	return executor.TestConnection(ctx)
}

type CreateInstanceRequest struct {
	Name            string `json:"name" binding:"required"`
	DBType          string `json:"dbType" binding:"required,oneof=mysql postgresql mongodb redis"`
	Host            string `json:"host" binding:"required"`
	Port            int    `json:"port" binding:"required"`
	Username        string `json:"username"`
	Password        string `json:"password"` // Redis 类型密码可选，其他类型在 handler 中验证
	DatabaseName    string `json:"databaseName"`
	AuthDatabase    string `json:"authDatabase"`
	Charset         string `json:"charset"`
	ConnectionString string `json:"connectionString"`
	SSLEnabled      bool   `json:"sslEnabled"`
	SSLCert         string `json:"sslCert"`
	Description     string `json:"description"`
}

type UpdateInstanceRequest struct {
	Name            string `json:"name"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"` // 可选，如果提供则更新
	DatabaseName    string `json:"databaseName"`
	AuthDatabase    string `json:"authDatabase"`
	Charset         string `json:"charset"`
	ConnectionString string `json:"connectionString"`
	SSLEnabled      bool   `json:"sslEnabled"`
	SSLCert         string `json:"sslCert"`
	Description     string `json:"description"`
	IsEnabled       bool   `json:"isEnabled"`
}
