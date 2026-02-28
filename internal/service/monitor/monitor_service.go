package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"gorm.io/gorm"
)

type MonitorService struct {
	repo *repository.MonitorRepository
}

func NewMonitorService(repo *repository.MonitorRepository) *MonitorService {
	return &MonitorService{repo: repo}
}

// CreateMonitor 创建监控查询语句
func (s *MonitorService) CreateMonitor(name, expr, userID string) (*model.Monitor, error) {
	// 检查名称是否已存在
	_, err := s.repo.FindByName(name)
	if err != nil {
		// 如果是记录不存在错误，说明名称可用，继续
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("检查监控名称失败: %w", err)
		}
	} else {
		// 记录存在，名称已使用
		return nil, fmt.Errorf("监控名称已存在: %s", name)
	}

	monitor := &model.Monitor{
		Name:      name,
		Expr:      expr,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	monitor.CreatedAt = time.Now()
	monitor.UpdatedAt = time.Now()

	if err := s.repo.Create(monitor); err != nil {
		return nil, err
	}

	return monitor, nil
}

// UpdateMonitor 更新监控查询语句
func (s *MonitorService) UpdateMonitor(id uint, expr, userID string) (*model.Monitor, error) {
	monitor, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("监控不存在: %v", err)
	}

	// 更新表达式和更新人
	monitor.Expr = expr
	monitor.UpdatedBy = userID
	monitor.UpdatedAt = time.Now()

	if err := s.repo.Update(monitor); err != nil {
		return nil, err
	}

	return monitor, nil
}

// DeleteMonitor 删除监控查询语句
func (s *MonitorService) DeleteMonitor(id uint) error {
	// 检查是否存在
	_, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("监控不存在: %v", err)
	}

	return s.repo.Delete(id)
}

// GetMonitor 获取单个监控详情
func (s *MonitorService) GetMonitor(id uint) (*model.Monitor, error) {
	return s.repo.FindByID(id)
}

// ListMonitors 获取监控列表
func (s *MonitorService) ListMonitors(name string, page, pageSize int) (total int64, monitors []model.Monitor, err error) {
	return s.repo.List(name, page, pageSize)
}

// CountMonitors 统计监控数量
func (s *MonitorService) CountMonitors(name string) (int64, error) {
	return s.repo.Count(name)
}

// QueryProbe 查询 Probe 监控数据
// TODO: 需要集成 Prometheus/VictoriaMetrics 等监控系统
func (s *MonitorService) QueryProbe(group, project, env, module, address string) (interface{}, error) {
	// TODO: 实现 Probe 查询逻辑
	// 目前返回模拟数据
	return map[string]interface{}{
		"group":     group,
		"project":   project,
		"env":       env,
		"module":    module,
		"address":   address,
		"probes":    []interface{}{},
	}, nil
}

