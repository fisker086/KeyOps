package repository

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

// AlertRuleGroupRepository 告警规则组仓库
type AlertRuleGroupRepository struct {
	db *gorm.DB
}

func NewAlertRuleGroupRepository(db *gorm.DB) *AlertRuleGroupRepository {
	return &AlertRuleGroupRepository{db: db}
}

func (r *AlertRuleGroupRepository) Create(group *model.AlertRuleGroup) error {
	return r.db.Create(group).Error
}

func (r *AlertRuleGroupRepository) Update(group *model.AlertRuleGroup) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(group).
		Select("group_name", "description", "department_id", "source_id", "file", "enabled").
		Updates(map[string]interface{}{
			"group_name":    group.GroupName,
			"description":   group.Description,
			"department_id": group.DepartmentID,
			"source_id":     group.SourceID,
			"file":          group.File,
			"enabled":       group.Enabled,
		}).Error
}

func (r *AlertRuleGroupRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertRuleGroup{}, "id = ?", id).Error
}

func (r *AlertRuleGroupRepository) FindByID(id uint) (*model.AlertRuleGroup, error) {
	var group model.AlertRuleGroup
	err := r.db.Where("id = ?", id).First(&group).Error
	return &group, err
}

func (r *AlertRuleGroupRepository) List(departmentID *string) ([]model.AlertRuleGroup, error) {
	var groups []model.AlertRuleGroup
	query := r.db.Model(&model.AlertRuleGroup{})
	if departmentID != nil {
		// 返回该部门的规则组和全局规则组
		query = query.Where("department_id = ? OR department_id IS NULL", *departmentID)
	} else {
		// 只返回全局规则组
		query = query.Where("department_id IS NULL")
	}
	err := query.Where("enabled = ?", true).Order("group_name ASC").Find(&groups).Error
	return groups, err
}

func (r *AlertRuleGroupRepository) ListAll(page, pageSize int) (total int64, groups []model.AlertRuleGroup, err error) {
	query := r.db.Model(&model.AlertRuleGroup{})

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertRuleGroup{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("created_at DESC").Find(&groups).Error
	return
}

// ListBySourceID 根据数据源ID获取规则组列表（不分页）
func (r *AlertRuleGroupRepository) ListBySourceID(sourceID uint) ([]model.AlertRuleGroup, error) {
	var groups []model.AlertRuleGroup
	err := r.db.Where("source_id = ?", sourceID).Order("group_name ASC").Find(&groups).Error
	return groups, err
}

// FindBySourceIDAndGroupName 根据数据源ID和规则组名称查询
func (r *AlertRuleGroupRepository) FindBySourceIDAndGroupName(sourceID uint, groupName string) (*model.AlertRuleGroup, error) {
	var group model.AlertRuleGroup
	err := r.db.Where("source_id = ? AND group_name = ?", sourceID, groupName).First(&group).Error
	return &group, err
}

// ListBySourceIDWithPagination 根据数据源ID分页获取规则组列表
func (r *AlertRuleGroupRepository) ListBySourceIDWithPagination(sourceID uint, page, pageSize int) (total int64, groups []model.AlertRuleGroup, err error) {
	query := r.db.Model(&model.AlertRuleGroup{}).Where("source_id = ?", sourceID)
	if err = query.Count(&total).Error; err != nil {
		return
	}
	if total == 0 {
		return 0, []model.AlertRuleGroup{}, nil
	}
	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}
	err = query.Order("group_name ASC").Find(&groups).Error
	return
}

// AlertRuleSourceRepository 告警规则数据源仓库
type AlertRuleSourceRepository struct {
	db *gorm.DB
}

func NewAlertRuleSourceRepository(db *gorm.DB) *AlertRuleSourceRepository {
	return &AlertRuleSourceRepository{db: db}
}

func (r *AlertRuleSourceRepository) Create(source *model.AlertRuleSource) error {
	return r.db.Create(source).Error
}

func (r *AlertRuleSourceRepository) Update(source *model.AlertRuleSource) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(source).
		Select("source_name", "source_type", "address", "api_key", "auto_sync", "sync_interval").
		Updates(map[string]interface{}{
			"source_name":   source.SourceName,
			"source_type":   source.SourceType,
			"address":       source.Address,
			"api_key":       source.APIKey,
			"auto_sync":     source.AutoSync,
			"sync_interval": source.SyncInterval,
		}).Error
}

func (r *AlertRuleSourceRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertRuleSource{}, "id = ?", id).Error
}

func (r *AlertRuleSourceRepository) FindByID(id uint) (*model.AlertRuleSource, error) {
	var source model.AlertRuleSource
	err := r.db.Where("id = ?", id).First(&source).Error
	return &source, err
}

// FindByAPIKey 根据API密钥查找数据源
func (r *AlertRuleSourceRepository) FindByAPIKey(apiKey string) (*model.AlertRuleSource, error) {
	var source model.AlertRuleSource
	err := r.db.Where("api_key = ?", apiKey).First(&source).Error
	return &source, err
}

func (r *AlertRuleSourceRepository) List(page, pageSize int) (total int64, sources []model.AlertRuleSource, err error) {
	query := r.db.Model(&model.AlertRuleSource{})

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertRuleSource{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("created_at DESC").Find(&sources).Error
	return
}

// ListAll 获取所有数据源（不分页，用于调度器启动时加载）
func (r *AlertRuleSourceRepository) ListAll(page, pageSize int) (total int64, sources []model.AlertRuleSource, err error) {
	query := r.db.Model(&model.AlertRuleSource{})

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertRuleSource{}, nil
	}

	// 如果指定了分页参数，则分页；否则返回所有
	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("created_at DESC").Find(&sources).Error
	return
}

// ListByDepartment 根据部门获取数据源列表（不分页）
// 简化后：不再区分部门，返回所有数据源
func (r *AlertRuleSourceRepository) ListByDepartment(departmentID *string) ([]model.AlertRuleSource, error) {
	var sources []model.AlertRuleSource
	// 不再区分部门，返回所有数据源
	err := r.db.Model(&model.AlertRuleSource{}).Order("source_name ASC").Find(&sources).Error
	return sources, err
}

// ListByGroup 根据规则组获取数据源列表（已废弃：规则组属于数据源，应该通过规则组的source_id查询数据源）
// 保留此方法以保持向后兼容，实际应该使用 GetRuleGroup 然后通过 source_id 查询数据源
func (r *AlertRuleSourceRepository) ListByGroup(groupID *uint) ([]model.AlertRuleSource, error) {
	// 如果提供了规则组ID，先查询规则组获取数据源ID，然后查询数据源
	if groupID != nil {
		var group model.AlertRuleGroup
		if err := r.db.First(&group, *groupID).Error; err != nil {
			return nil, err
		}
		var source model.AlertRuleSource
		if err := r.db.First(&source, group.SourceID).Error; err != nil {
			return nil, err
		}
		return []model.AlertRuleSource{source}, nil
	}
	// 如果没有提供规则组ID，返回所有数据源
	var sources []model.AlertRuleSource
	err := r.db.Model(&model.AlertRuleSource{}).Order("source_name ASC").Find(&sources).Error
	return sources, err
}

// AlertRuleRepository 告警规则仓库
type AlertRuleRepository struct {
	db *gorm.DB
}

func NewAlertRuleRepository(db *gorm.DB) *AlertRuleRepository {
	return &AlertRuleRepository{db: db}
}

func (r *AlertRuleRepository) Create(rule *model.AlertRule) error {
	return r.db.Create(rule).Error
}

func (r *AlertRuleRepository) Update(rule *model.AlertRule) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(rule).
		Select("name", "group_id", "group", "expr", "duration", "labels", "annotations", "health", "source_id", "enabled").
		Updates(map[string]interface{}{
			"name":        rule.Name,
			"group_id":    rule.GroupID,
			"group":       rule.Group,
			"expr":        rule.Expr,
			"duration":    rule.Duration,
			"labels":      rule.Labels,
			"annotations": rule.Annotations,
			"health":      rule.Health,
			"source_id":   rule.SourceID,
			"enabled":     rule.Enabled,
		}).Error
}

func (r *AlertRuleRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertRule{}, "id = ?", id).Error
}

func (r *AlertRuleRepository) FindByID(id uint) (*model.AlertRule, error) {
	var rule model.AlertRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	return &rule, err
}

func (r *AlertRuleRepository) List(sourceID uint, groupID *uint, excludeGroupID *uint, group, name string, page, pageSize int) (total int64, rules []model.AlertRule, err error) {
	query := r.db.Model(&model.AlertRule{})

	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	}
	if excludeGroupID != nil {
		// 排除指定规则组（用于「将现有规则加入本组」：只查不在本组的规则）
		query = query.Where("(group_id IS NULL OR group_id != ?)", *excludeGroupID)
	} else if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if group != "" {
		// 兼容旧的 group 字段
		// 根据数据库类型使用正确的引号
		groupColumn := "`group`"
		if r.db.Dialector.Name() == "postgres" {
			groupColumn = "\"group\""
		}
		query = query.Where(groupColumn+" = ?", group)
	}
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertRule{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("created_at DESC").Find(&rules).Error
	return
}

func (r *AlertRuleRepository) Toggle(id uint, enabled bool) error {
	return r.db.Model(&model.AlertRule{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// ListByGroupID 根据规则组ID获取规则列表
func (r *AlertRuleRepository) ListByGroupID(groupID uint) ([]model.AlertRule, error) {
	var rules []model.AlertRule
	err := r.db.Where("group_id = ?", groupID).Find(&rules).Error
	return rules, err
}

// AlertEventRepository 告警事件仓库
type AlertEventRepository struct {
	db *gorm.DB
}

func NewAlertEventRepository(db *gorm.DB) *AlertEventRepository {
	return &AlertEventRepository{db: db}
}

func (r *AlertEventRepository) Create(event *model.AlertEvent) error {
	return r.db.Create(event).Error
}

func (r *AlertEventRepository) Update(event *model.AlertEvent) error {
	// 必须指定 ID，否则 GORM 会报错 "WHERE conditions required"
	if event.ID == 0 {
		return fmt.Errorf("更新告警事件失败：ID 不能为空")
	}
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、finger_print（指纹不应该被更新）
	// AlertEvent 的更新主要用于更新告警状态，不更新 uid（处理人通过 Claim 等方法单独更新）
	return r.db.Model(event).
		Where("id = ?", event.ID).
		Select("alert_title", "source_id", "description", "level", "first_trigger_time", "first_ack_time", "trigger_time", "recover_time", "annotations", "is_recovered", "progress", "tags", "source_ip", "department_id", "integration_id").
		Updates(map[string]interface{}{
			"alert_title":        event.AlertTitle,
			"source_id":          event.SourceID,
			"description":        event.Description,
			"level":              event.Level,
			"first_trigger_time": event.FirstTriggerTime,
			"first_ack_time":     event.FirstAckTime,
			"trigger_time":       event.TriggerTime,
			"recover_time":       event.RecoverTime,
			"annotations":        event.Annotations,
			"is_recovered":       event.IsRecovered,
			"progress":           event.Progress,
			"tags":               event.Tags,
			"source_ip":          event.SourceIP,
			"department_id":      event.DepartmentID,
			"integration_id":     event.IntegrationID,
		}).Error
}

func (r *AlertEventRepository) FindByID(id uint64) (*model.AlertEvent, error) {
	var event model.AlertEvent
	err := r.db.Where("id = ?", id).First(&event).Error
	return &event, err
}

func (r *AlertEventRepository) FindByFingerprint(fingerprint string) (*model.AlertEvent, error) {
	if fingerprint == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var event model.AlertEvent
	err := r.db.Where("finger_print = ? AND is_recovered = ?", fingerprint, false).First(&event).Error
	return &event, err
}

func (r *AlertEventRepository) FindSimilarEvents(fingerprint string, since time.Time) ([]model.AlertEvent, error) {
	var events []model.AlertEvent
	err := r.db.Where("finger_print = ? AND trigger_time >= ?", fingerprint, since).Find(&events).Error
	return events, err
}

func (r *AlertEventRepository) List(departmentID string, level, progress int, title, timeRange string, page, pageSize int) (total int64, events []model.AlertEvent, err error) {
	query := r.db.Model(&model.AlertEvent{})

	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}
	if level > 0 {
		query = query.Where("level = ?", level)
	}
	if progress > 0 {
		if progress == 4 {
			// 待处理：未认领或已认领
			query = query.Where("progress IN ?", []int{1, 2})
		} else {
			query = query.Where("progress = ?", progress)
		}
	}
	if title != "" {
		query = query.Where("alert_title LIKE ?", "%"+title+"%")
	}
	if timeRange != "" {
		// TODO: 实现时间范围过滤
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertEvent{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("trigger_time DESC").Find(&events).Error
	return
}

func (r *AlertEventRepository) Claim(id uint64, uid string) error {
	return r.db.Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"progress": 2,
		"uid":      uid,
	}).Error
}

func (r *AlertEventRepository) CancelClaim(id uint64) error {
	return r.db.Model(&model.AlertEvent{}).Where("id = ?", id).Update("progress", 1).Error
}

func (r *AlertEventRepository) Close(id uint64, uid string) error {
	return r.db.Model(&model.AlertEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"progress": 3,
		"uid":      uid,
	}).Error
}

func (r *AlertEventRepository) Open(id uint64) error {
	return r.db.Model(&model.AlertEvent{}).Where("id = ?", id).Update("progress", 1).Error
}

// ListUnclosed 获取所有未关闭的告警事件
func (r *AlertEventRepository) ListUnclosed() ([]model.AlertEvent, error) {
	var events []model.AlertEvent
	err := r.db.Where("is_recovered = ?", false).Find(&events).Error
	return events, err
}

// GetStatistics 获取告警统计信息
func (r *AlertEventRepository) GetStatistics(timeRange string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// 计算时间范围
	var startTime time.Time
	if timeRange != "" {
		now := time.Now()
		switch timeRange {
		case "24h":
			startTime = now.Add(-24 * time.Hour)
		case "7d":
			startTime = now.Add(-7 * 24 * time.Hour)
		case "30d":
			startTime = now.Add(-30 * 24 * time.Hour)
		default:
			startTime = now.Add(-24 * time.Hour)
		}
	}
	
	query := r.db.Model(&model.AlertEvent{})
	if timeRange != "" {
		query = query.Where("first_trigger_time >= ?", startTime)
	}
	
	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total
	
	// 按状态统计
	var statusStats []struct {
		Progress int
		Count    int64
	}
	if err := query.Select("progress, COUNT(*) as count").Group("progress").Find(&statusStats).Error; err != nil {
		return nil, err
	}
	statusMap := make(map[int]int64)
	for _, s := range statusStats {
		statusMap[s.Progress] = s.Count
	}
	stats["by_status"] = map[string]int64{
		"unclaimed": statusMap[1], // 未认领
		"claimed":   statusMap[2], // 已认领
		"closed":    statusMap[3], // 已关闭
	}
	
	// 按等级统计
	var levelStats []struct {
		Level int
		Count int64
	}
	if err := query.Select("level, COUNT(*) as count").Group("level").Find(&levelStats).Error; err != nil {
		return nil, err
	}
	levelMap := make(map[int]int64)
	for _, l := range levelStats {
		levelMap[l.Level] = l.Count
	}
	stats["by_level"] = levelMap
	
	// 已恢复/未恢复统计
	var recoveredCount, unrecoveredCount int64
	if err := query.Where("is_recovered = ?", true).Count(&recoveredCount).Error; err != nil {
		return nil, err
	}
	if err := query.Where("is_recovered = ?", false).Count(&unrecoveredCount).Error; err != nil {
		return nil, err
	}
	stats["by_recovery"] = map[string]int64{
		"recovered":   recoveredCount,
		"unrecovered": unrecoveredCount,
	}
	
	// 今日告警数
	todayStart := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := r.db.Model(&model.AlertEvent{}).Where("first_trigger_time >= ?", todayStart).Count(&todayCount).Error; err != nil {
		return nil, err
	}
	stats["today_count"] = todayCount
	
	// 昨日告警数
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	var yesterdayCount int64
	if err := r.db.Model(&model.AlertEvent{}).Where("first_trigger_time >= ? AND first_trigger_time < ?", yesterdayStart, todayStart).Count(&yesterdayCount).Error; err != nil {
		return nil, err
	}
	stats["yesterday_count"] = yesterdayCount
	
	return stats, nil
}

// GetTrendStatistics 获取告警趋势统计（按日期）
func (r *AlertEventRepository) GetTrendStatistics(timeRange string) ([]map[string]interface{}, error) {
	var startTime time.Time
	now := time.Now()
	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-7 * 24 * time.Hour)
	}
	
	var results []struct {
		Date  string
		Count int64
	}
	
	// 根据时间范围选择分组方式
	var groupBy string
	if r.db.Dialector.Name() == "postgres" {
		// PostgreSQL: 使用 to_char 函数
		if timeRange == "24h" {
			groupBy = "to_char(first_trigger_time, 'YYYY-MM-DD HH24:00:00')"
		} else {
			groupBy = "DATE(first_trigger_time)"
		}
	} else {
		// MySQL: 使用 DATE_FORMAT 函数
		if timeRange == "24h" {
			groupBy = "DATE_FORMAT(first_trigger_time, '%Y-%m-%d %H:00:00')"
		} else {
			groupBy = "DATE(first_trigger_time)"
		}
	}
	
	if err := r.db.Model(&model.AlertEvent{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as count", groupBy)).
		Where("first_trigger_time >= ?", startTime).
		Group("date").
		Order("date ASC").
		Find(&results).Error; err != nil {
		return nil, err
	}
	
	trends := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		trends = append(trends, map[string]interface{}{
			"date":  r.Date,
			"count": r.Count,
		})
	}
	
	return trends, nil
}

// GetTopAlerts 获取Top N告警（按触发次数）
func (r *AlertEventRepository) GetTopAlerts(timeRange string, limit int) ([]map[string]interface{}, error) {
	var startTime time.Time
	now := time.Now()
	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-7 * 24 * time.Hour)
	}
	
	if limit <= 0 {
		limit = 10
	}
	
	var results []struct {
		AlertTitle string
		Count      int64
	}
	
	if err := r.db.Model(&model.AlertEvent{}).
		Select("alert_title, COUNT(*) as count").
		Where("first_trigger_time >= ?", startTime).
		Group("alert_title, finger_print").
		Order("count DESC").
		Limit(limit).
		Find(&results).Error; err != nil {
		return nil, err
	}
	
	topAlerts := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		topAlerts = append(topAlerts, map[string]interface{}{
			"alert_title": r.AlertTitle,
			"count":       r.Count,
		})
	}
	
	return topAlerts, nil
}

// AlertLogRepository 告警日志仓库
type AlertLogRepository struct {
	db *gorm.DB
}

func NewAlertLogRepository(db *gorm.DB) *AlertLogRepository {
	return &AlertLogRepository{db: db}
}

func (r *AlertLogRepository) Create(log *model.AlertLog) error {
	return r.db.Create(log).Error
}

// AlertStrategyRepository 告警策略仓库
type AlertStrategyRepository struct {
	db *gorm.DB
}

func NewAlertStrategyRepository(db *gorm.DB) *AlertStrategyRepository {
	return &AlertStrategyRepository{db: db}
}

func (r *AlertStrategyRepository) Create(strategy *model.AlertStrategy) error {
	// 调试日志
	logger.Infof("[AlertStrategyRepository] Create: StrategyName=%s, Filters=%s, StrategySet=%s",
		strategy.StrategyName, string(strategy.Filters), string(strategy.StrategySet))
	err := r.db.Create(strategy).Error
	if err != nil {
		logger.Infof("[AlertStrategyRepository] Create error: %v", err)
	} else {
		logger.Infof("[AlertStrategyRepository] Create success: ID=%d", strategy.ID)
	}
	return err
}

func (r *AlertStrategyRepository) Update(strategy *model.AlertStrategy) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、uid
	return r.db.Model(strategy).
		Select("strategy_name", "department_id", "template_id", "status", "weight", "continuous", "delay", "time_slot", "filters", "strategy_set").
		Updates(map[string]interface{}{
			"strategy_name": strategy.StrategyName,
			"department_id": strategy.DepartmentID,
			"template_id":   strategy.TemplateID,
			"status":        strategy.Status,
			"weight":        strategy.Weight,
			"continuous":    strategy.Continuous,
			"delay":         strategy.Delay,
			"time_slot":     strategy.TimeSlot,
			"filters":       strategy.Filters,
			"strategy_set":  strategy.StrategySet,
		}).Error
}

func (r *AlertStrategyRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertStrategy{}, "id = ?", id).Error
}

func (r *AlertStrategyRepository) FindByID(id uint) (*model.AlertStrategy, error) {
	var strategy model.AlertStrategy
	err := r.db.Where("id = ?", id).First(&strategy).Error
	return &strategy, err
}

func (r *AlertStrategyRepository) List(departmentID string, status string, page, pageSize int) (total int64, strategies []model.AlertStrategy, err error) {
	query := r.db.Model(&model.AlertStrategy{})

	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertStrategy{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("weight DESC, created_at DESC").Find(&strategies).Error
	return
}

func (r *AlertStrategyRepository) Toggle(id uint, status string) error {
	return r.db.Model(&model.AlertStrategy{}).Where("id = ?", id).Update("status", status).Error
}

// AlertLevelRepository 告警等级仓库
type AlertLevelRepository struct {
	db *gorm.DB
}

func NewAlertLevelRepository(db *gorm.DB) *AlertLevelRepository {
	return &AlertLevelRepository{db: db}
}

func (r *AlertLevelRepository) List() ([]model.AlertLevel, error) {
	var levels []model.AlertLevel
	err := r.db.Order("id ASC").Find(&levels).Error
	return levels, err
}

// AlertAggregationRepository 告警聚合仓库
type AlertAggregationRepository struct {
	db *gorm.DB
}

func NewAlertAggregationRepository(db *gorm.DB) *AlertAggregationRepository {
	return &AlertAggregationRepository{db: db}
}

func (r *AlertAggregationRepository) Create(agg *model.AlertAggregation) error {
	return r.db.Create(agg).Error
}

func (r *AlertAggregationRepository) Update(agg *model.AlertAggregation) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、uid
	return r.db.Model(agg).
		Select("aggregation_name", "aggregation_desc", "level_dimension", "tags_dimension", "title_dimension", "windows", "storm", "status").
		Updates(map[string]interface{}{
			"aggregation_name": agg.AggregationName,
			"aggregation_desc": agg.AggregationDesc,
			"level_dimension":  agg.LevelDimension,
			"tags_dimension":   agg.TagsDimension,
			"title_dimension":  agg.TitleDimension,
			"windows":          agg.Windows,
			"storm":            agg.Storm,
			"status":           agg.Status,
		}).Error
}

func (r *AlertAggregationRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertAggregation{}, "id = ?", id).Error
}

func (r *AlertAggregationRepository) FindByID(id uint) (*model.AlertAggregation, error) {
	var agg model.AlertAggregation
	err := r.db.Where("id = ?", id).First(&agg).Error
	return &agg, err
}

func (r *AlertAggregationRepository) ListEnabled() ([]model.AlertAggregation, error) {
	var aggs []model.AlertAggregation
	err := r.db.Where("status = ?", "enabled").Find(&aggs).Error
	return aggs, err
}

func (r *AlertAggregationRepository) List(page, pageSize int) (int64, []model.AlertAggregation, error) {
	var aggs []model.AlertAggregation
	var total int64
	query := r.db.Model(&model.AlertAggregation{})
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&aggs).Error
	return total, aggs, err
}

// AlertSilenceRepository 告警静默仓库
type AlertSilenceRepository struct {
	db *gorm.DB
}

func NewAlertSilenceRepository(db *gorm.DB) *AlertSilenceRepository {
	return &AlertSilenceRepository{db: db}
}

func (r *AlertSilenceRepository) Create(silence *model.AlertSilence) error {
	return r.db.Create(silence).Error
}

func (r *AlertSilenceRepository) Update(silence *model.AlertSilence) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、uid
	return r.db.Model(silence).
		Select("department_id", "silence_name", "silence_desc", "silence_type", "silence_time", "filters").
		Updates(map[string]interface{}{
			"department_id": silence.DepartmentID,
			"silence_name":  silence.SilenceName,
			"silence_desc":  silence.SilenceDesc,
			"silence_type":  silence.SilenceType,
			"silence_time":  silence.SilenceTime,
			"filters":       silence.Filters,
		}).Error
}

func (r *AlertSilenceRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertSilence{}, "id = ?", id).Error
}

func (r *AlertSilenceRepository) FindByID(id uint) (*model.AlertSilence, error) {
	var silence model.AlertSilence
	err := r.db.Where("id = ?", id).First(&silence).Error
	return &silence, err
}

func (r *AlertSilenceRepository) ListByDepartment(departmentID string) ([]model.AlertSilence, error) {
	var silences []model.AlertSilence
	query := r.db.Model(&model.AlertSilence{})
	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}
	err := query.Find(&silences).Error
	return silences, err
}

func (r *AlertSilenceRepository) List(page, pageSize int) (int64, []model.AlertSilence, error) {
	var silences []model.AlertSilence
	var total int64
	query := r.db.Model(&model.AlertSilence{})
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Find(&silences).Error
	return total, silences, err
}

// AlertRestrainRepository 告警抑制仓库
type AlertRestrainRepository struct {
	db *gorm.DB
}

func NewAlertRestrainRepository(db *gorm.DB) *AlertRestrainRepository {
	return &AlertRestrainRepository{db: db}
}

func (r *AlertRestrainRepository) Create(restrain *model.AlertRestrain) error {
	return r.db.Create(restrain).Error
}

func (r *AlertRestrainRepository) Update(restrain *model.AlertRestrain) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、uid
	return r.db.Model(restrain).
		Select("restrain_type", "fields", "cumulative_time").
		Updates(map[string]interface{}{
			"restrain_type":   restrain.RestrainType,
			"fields":          restrain.Fields,
			"cumulative_time": restrain.CumulativeTime,
		}).Error
}

func (r *AlertRestrainRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertRestrain{}, "id = ?", id).Error
}

func (r *AlertRestrainRepository) FindByID(id uint) (*model.AlertRestrain, error) {
	var restrain model.AlertRestrain
	err := r.db.Where("id = ?", id).First(&restrain).Error
	return &restrain, err
}

func (r *AlertRestrainRepository) ListAll() ([]model.AlertRestrain, error) {
	var restrains []model.AlertRestrain
	err := r.db.Find(&restrains).Error
	return restrains, err
}

func (r *AlertRestrainRepository) List(page, pageSize int) (int64, []model.AlertRestrain, error) {
	var restrains []model.AlertRestrain
	var total int64
	query := r.db.Model(&model.AlertRestrain{})
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&restrains).Error
	return total, restrains, err
}

// AlertTemplateRepository 告警模板仓库
type AlertTemplateRepository struct {
	db *gorm.DB
}

func NewAlertTemplateRepository(db *gorm.DB) *AlertTemplateRepository {
	return &AlertTemplateRepository{db: db}
}

func (r *AlertTemplateRepository) Create(template *model.AlertTemplate) error {
	return r.db.Create(template).Error
}

func (r *AlertTemplateRepository) Update(template *model.AlertTemplate) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(template).
		Select("template_name", "template_desc", "channels", "members", "alert_groups", "enable").
		Updates(map[string]interface{}{
			"template_name": template.TemplateName,
			"template_desc": template.TemplateDesc,
			"channels":      template.Channels,
			"members":       template.Members,
			"alert_groups":  template.AlertGroups,
			"enable":        template.Enable,
		}).Error
}

func (r *AlertTemplateRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertTemplate{}, "id = ?", id).Error
}

func (r *AlertTemplateRepository) FindByID(id uint) (*model.AlertTemplate, error) {
	var template model.AlertTemplate
	err := r.db.Where("id = ?", id).First(&template).Error
	return &template, err
}

func (r *AlertTemplateRepository) List() ([]model.AlertTemplate, error) {
	var templates []model.AlertTemplate
	err := r.db.Order("created_at DESC").Find(&templates).Error
	return templates, err
}

func (r *AlertTemplateRepository) ListWithPagination(page, pageSize int) (int64, []model.AlertTemplate, error) {
	var templates []model.AlertTemplate
	var total int64
	query := r.db.Model(&model.AlertTemplate{})
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&templates).Error
	return total, templates, err
}

// AlertChannelRepository 告警渠道仓库
type AlertChannelRepository struct {
	db *gorm.DB
}

func NewAlertChannelRepository(db *gorm.DB) *AlertChannelRepository {
	return &AlertChannelRepository{db: db}
}

func (r *AlertChannelRepository) Create(channel *model.AlertChannel) error {
	return r.db.Create(channel).Error
}

func (r *AlertChannelRepository) Update(channel *model.AlertChannel) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(channel).
		Select("channel_name", "channel_type", "channel_sign", "channel_group").
		Updates(map[string]interface{}{
			"channel_name":  channel.ChannelName,
			"channel_type":  channel.ChannelType,
			"channel_sign":  channel.ChannelSign,
			"channel_group": channel.ChannelGroup,
		}).Error
}

func (r *AlertChannelRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertChannel{}, "id = ?", id).Error
}

func (r *AlertChannelRepository) FindByID(id uint) (*model.AlertChannel, error) {
	var channel model.AlertChannel
	err := r.db.Where("id = ?", id).First(&channel).Error
	return &channel, err
}

func (r *AlertChannelRepository) List() ([]model.AlertChannel, error) {
	var channels []model.AlertChannel
	err := r.db.Order("created_at DESC").Find(&channels).Error
	return channels, err
}

func (r *AlertChannelRepository) ListWithPagination(page, pageSize int) (int64, []model.AlertChannel, error) {
	var channels []model.AlertChannel
	var total int64
	query := r.db.Model(&model.AlertChannel{})
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&channels).Error
	return total, channels, err
}

// StrategyLogRepository 策略日志仓库
type StrategyLogRepository struct {
	db *gorm.DB
}

func NewStrategyLogRepository(db *gorm.DB) *StrategyLogRepository {
	return &StrategyLogRepository{db: db}
}

func (r *StrategyLogRepository) Create(logEntry *model.StrategyLog) error {
	logger.Infof("[StrategyLogRepository] Creating strategy log: alertID=%d, strategyID=%d, isNotify=%v, notifyType=%d", logEntry.AlertID, logEntry.StrategyID, logEntry.IsNotify, logEntry.NotifyType)
	err := r.db.Create(logEntry).Error
	if err != nil {
		logger.Infof("[StrategyLogRepository] Failed to create strategy log: alertID=%d, strategyID=%d, error=%v", logEntry.AlertID, logEntry.StrategyID, err)
	} else {
		logger.Infof("[StrategyLogRepository] Successfully created strategy log: alertID=%d, strategyID=%d, logID=%d", logEntry.AlertID, logEntry.StrategyID, logEntry.ID)
	}
	return err
}

func (r *StrategyLogRepository) ListByAlertID(alertID uint64) ([]model.StrategyLog, error) {
	var logs []model.StrategyLog
	err := r.db.Where("alert_id = ?", alertID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *StrategyLogRepository) ListByAlertIDWithPagination(alertID uint64, page, pageSize int) (int64, []model.StrategyLog, error) {
	var logs []model.StrategyLog
	var total int64
	query := r.db.Model(&model.StrategyLog{})
	if alertID > 0 {
		query = query.Where("alert_id = ?", alertID)
		logger.Infof("[StrategyLogRepository] Querying strategy logs with filter: alertID=%d, page=%d, pageSize=%d", alertID, page, pageSize)
	} else {
		logger.Infof("[StrategyLogRepository] Querying all strategy logs: page=%d, pageSize=%d", page, pageSize)
	}
	err := query.Count(&total).Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&logs).Error
	if err != nil {
		logger.Infof("[StrategyLogRepository] Query error: alertID=%d, error=%v", alertID, err)
	} else {
		logger.Infof("[StrategyLogRepository] Query result: alertID=%d, total=%d, found=%d logs", alertID, total, len(logs))
	}
	return total, logs, err
}

func (r *StrategyLogRepository) FindByID(id uint) (*model.StrategyLog, error) {
	var log model.StrategyLog
	err := r.db.Where("id = ?", id).First(&log).Error
	return &log, err
}

// AlertGroupRepository 告警组仓库
type AlertGroupRepository struct {
	db *gorm.DB
}

func NewAlertGroupRepository(db *gorm.DB) *AlertGroupRepository {
	return &AlertGroupRepository{db: db}
}

func (r *AlertGroupRepository) Create(group *model.AlertGroup) error {
	return r.db.Create(group).Error
}

func (r *AlertGroupRepository) Update(group *model.AlertGroup) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at、uid
	return r.db.Model(group).
		Select("group_name", "description", "department_id", "members").
		Updates(map[string]interface{}{
			"group_name":    group.GroupName,
			"description":   group.Description,
			"department_id": group.DepartmentID,
			"members":       group.Members,
		}).Error
}

func (r *AlertGroupRepository) Delete(id uint) error {
	return r.db.Delete(&model.AlertGroup{}, "id = ?", id).Error
}

func (r *AlertGroupRepository) FindByID(id uint) (*model.AlertGroup, error) {
	var group model.AlertGroup
	err := r.db.Where("id = ?", id).First(&group).Error
	return &group, err
}

func (r *AlertGroupRepository) List(departmentID *string, page, pageSize int) (total int64, groups []model.AlertGroup, err error) {
	query := r.db.Model(&model.AlertGroup{})

	// 如果指定了部门ID，只返回该部门的告警组和全局告警组
	if departmentID != nil && *departmentID != "" {
		query = query.Where("department_id = ? OR department_id IS NULL", *departmentID)
	} else {
		// 如果没有指定部门ID，只返回全局告警组
		query = query.Where("department_id IS NULL")
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.AlertGroup{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("created_at DESC").Find(&groups).Error
	return
}

// ListAll 获取所有告警组（不分页，用于下拉选择）
func (r *AlertGroupRepository) ListAll(departmentID *string) ([]model.AlertGroup, error) {
	var groups []model.AlertGroup
	query := r.db.Model(&model.AlertGroup{})

	// 如果指定了部门ID，只返回该部门的告警组和全局告警组
	if departmentID != nil && *departmentID != "" {
		query = query.Where("department_id = ? OR department_id IS NULL", *departmentID)
	} else {
		// 如果没有指定部门ID，只返回全局告警组
		query = query.Where("department_id IS NULL")
	}

	err := query.Order("group_name ASC").Find(&groups).Error
	return groups, err
}

// ChannelTemplateRepository 渠道模板仓库
type ChannelTemplateRepository struct {
	db *gorm.DB
}

func NewChannelTemplateRepository(db *gorm.DB) *ChannelTemplateRepository {
	return &ChannelTemplateRepository{db: db}
}

func (r *ChannelTemplateRepository) Create(channelTemplate *model.ChannelTemplate) error {
	return r.db.Create(channelTemplate).Error
}

func (r *ChannelTemplateRepository) Update(channelTemplate *model.ChannelTemplate) error {
	// 使用 Select 明确指定要更新的字段，排除 created_at、updated_at
	return r.db.Model(channelTemplate).
		Select("channel_id", "template_id", "content", "finished").
		Updates(map[string]interface{}{
			"channel_id":  channelTemplate.ChannelID,
			"template_id": channelTemplate.TemplateID,
			"content":     channelTemplate.Content,
			"finished":    channelTemplate.Finished,
		}).Error
}

func (r *ChannelTemplateRepository) Delete(id uint) error {
	return r.db.Delete(&model.ChannelTemplate{}, "id = ?", id).Error
}

func (r *ChannelTemplateRepository) FindByID(id uint) (*model.ChannelTemplate, error) {
	var channelTemplate model.ChannelTemplate
	err := r.db.Where("id = ?", id).First(&channelTemplate).Error
	return &channelTemplate, err
}

func (r *ChannelTemplateRepository) FindByTemplateIDAndChannelID(templateID, channelID uint) (*model.ChannelTemplate, error) {
	var channelTemplate model.ChannelTemplate
	err := r.db.Where("template_id = ? AND channel_id = ?", templateID, channelID).First(&channelTemplate).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &channelTemplate, err
}

func (r *ChannelTemplateRepository) ListByTemplateID(templateID uint) ([]model.ChannelTemplate, error) {
	var channelTemplates []model.ChannelTemplate
	err := r.db.Where("template_id = ?", templateID).Find(&channelTemplates).Error
	return channelTemplates, err
}

func (r *ChannelTemplateRepository) DeleteByTemplateID(templateID uint) error {
	return r.db.Where("template_id = ?", templateID).Delete(&model.ChannelTemplate{}).Error
}

func (r *ChannelTemplateRepository) DeleteByTemplateIDAndChannelID(templateID, channelID uint) error {
	return r.db.Where("template_id = ? AND channel_id = ?", templateID, channelID).Delete(&model.ChannelTemplate{}).Error
}
