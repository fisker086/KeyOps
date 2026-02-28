package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	alertpkg "github.com/fisker086/keyops/internal/alert"
	alertmatcher "github.com/fisker086/keyops/internal/alert/matcher"
	alertnotification "github.com/fisker086/keyops/internal/alert/notification"
	alertprocessor "github.com/fisker086/keyops/internal/alert/processor"
	"github.com/fisker086/keyops/internal/alert/rulefile"
	"github.com/fisker086/keyops/internal/model"
	notifpkg "github.com/fisker086/keyops/internal/notification"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/internal/scheduler"
	"gorm.io/gorm"
)

// AlertService 告警服务
type AlertService struct {
	ruleGroupRepo       *repository.AlertRuleGroupRepository
	ruleSourceRepo      *repository.AlertRuleSourceRepository
	ruleRepo            *repository.AlertRuleRepository
	eventRepo           *repository.AlertEventRepository
	logRepo             *repository.AlertLogRepository
	strategyRepo        *repository.AlertStrategyRepository
	levelRepo           *repository.AlertLevelRepository
	aggregationRepo     *repository.AlertAggregationRepository
	silenceRepo         *repository.AlertSilenceRepository
	restrainRepo        *repository.AlertRestrainRepository
	templateRepo        *repository.AlertTemplateRepository
	channelRepo         *repository.AlertChannelRepository
	channelTemplateRepo *repository.ChannelTemplateRepository
	alertGroupRepo      *repository.AlertGroupRepository
	strategyLogRepo     *repository.StrategyLogRepository

	// 业务逻辑处理器
	silenceProcessor     *alertprocessor.SilenceProcessor
	restrainProcessor    *alertprocessor.RestrainProcessor
	aggregationProcessor *alertprocessor.AggregationProcessor
	strategyMatcher      *alertmatcher.StrategyMatcher
	alertNotifier        *alertnotification.AlertNotifier

	// 规则文件管理器
	ruleFileManager *rulefile.RuleFileManager

	// 数据源同步调度器
	syncScheduler *scheduler.DatasourceSyncScheduler
}

func NewAlertService(
	ruleGroupRepo *repository.AlertRuleGroupRepository,
	ruleSourceRepo *repository.AlertRuleSourceRepository,
	ruleRepo *repository.AlertRuleRepository,
	eventRepo *repository.AlertEventRepository,
	logRepo *repository.AlertLogRepository,
	strategyRepo *repository.AlertStrategyRepository,
	levelRepo *repository.AlertLevelRepository,
	aggregationRepo *repository.AlertAggregationRepository,
	silenceRepo *repository.AlertSilenceRepository,
	restrainRepo *repository.AlertRestrainRepository,
	templateRepo *repository.AlertTemplateRepository,
	channelRepo *repository.AlertChannelRepository,
	channelTemplateRepo *repository.ChannelTemplateRepository,
	alertGroupRepo *repository.AlertGroupRepository,
	strategyLogRepo *repository.StrategyLogRepository,
	ruleDir string, // 规则文件目录，如 "/etc/prometheus/rules" 或从配置读取
) *AlertService {
	service := &AlertService{
		ruleGroupRepo:       ruleGroupRepo,
		ruleSourceRepo:      ruleSourceRepo,
		ruleRepo:            ruleRepo,
		eventRepo:           eventRepo,
		logRepo:             logRepo,
		strategyRepo:        strategyRepo,
		levelRepo:           levelRepo,
		aggregationRepo:     aggregationRepo,
		silenceRepo:         silenceRepo,
		restrainRepo:        restrainRepo,
		templateRepo:        templateRepo,
		channelRepo:         channelRepo,
		channelTemplateRepo: channelTemplateRepo,
		alertGroupRepo:      alertGroupRepo,
		strategyLogRepo:     strategyLogRepo,

		// 初始化业务逻辑处理器
		silenceProcessor:     alertprocessor.NewSilenceProcessor(silenceRepo),
		restrainProcessor:    alertprocessor.NewRestrainProcessor(restrainRepo, eventRepo),
		aggregationProcessor: alertprocessor.NewAggregationProcessor(aggregationRepo, eventRepo),
		strategyMatcher:      alertmatcher.NewStrategyMatcher(strategyRepo),
		alertNotifier:        alertnotification.NewAlertNotifier(strategyLogRepo, templateRepo, channelRepo, channelTemplateRepo, alertGroupRepo, ruleSourceRepo, getFrontendURL()),

		// 初始化规则文件管理器
		ruleFileManager: rulefile.NewRuleFileManager(ruleDir),
	}

	// 设置调度器的同步服务（避免循环依赖，先创建 service 再设置）
	service.syncScheduler = scheduler.NewDatasourceSyncScheduler(ruleSourceRepo, service)

	return service
}

// getFrontendURL 获取前端URL（从环境变量读取）
func getFrontendURL() string {
	// 优先从环境变量读取
	if url := os.Getenv("FRONTEND_URL"); url != "" {
		return url
	}
	// 如果没有配置，返回空字符串，使用相对路径
	return ""
}

// GenerateFingerprint 生成告警指纹（委托给 alert 包）
func (s *AlertService) GenerateFingerprint(labels map[string]string) string {
	return alertpkg.GenerateFingerprint(labels)
}

// ProcessPrometheusAlert 处理Prometheus告警（增强版，包含静默、抑制、策略匹配、通知）
func (s *AlertService) ProcessPrometheusAlert(alert *model.PrometheusAlert, sourceID uint, sourceIP string, notificationManager *notifpkg.NotificationManager) error {
	for _, alertItem := range alert.Alerts {
		// 验证 labels 不为 nil
		if alertItem.Labels == nil {
			alertItem.Labels = make(map[string]string)
		}

		// 生成指纹
		fingerprint := s.GenerateFingerprint(alertItem.Labels)
		if fingerprint == "" {
			return fmt.Errorf("无法生成告警指纹：labels 为空")
		}

		// 检查是否已存在
		existing, err := s.eventRepo.FindByFingerprint(fingerprint)
		if err != nil && err != gorm.ErrRecordNotFound {
			log.Printf("[AlertService] Error finding existing alert by fingerprint %s: %v", fingerprint, err)
			return err
		}

		now := time.Now()
		// 只有当 existing 不为 nil、ID 大于 0 且未恢复时，才更新现有告警
		if existing != nil && existing.ID > 0 && !existing.IsRecovered {
			log.Printf("[AlertService] Alert with fingerprint %s already exists (ID=%d), updating existing alert", fingerprint, existing.ID)
			// 更新现有告警
			existing.TriggerTime = &now
			if existing.FirstTriggerTime == nil {
				existing.FirstTriggerTime = &now
			}
			if alertItem.Status == "resolved" {
				existing.IsRecovered = true
				existing.RecoverTime = &now
			}
			// 更新注解和标签
			if annotationsJSON, err := json.Marshal(alertItem.Annotations); err == nil {
				existing.Annotations = annotationsJSON
			}
			if tagsJSON, err := json.Marshal(alertItem.Labels); err == nil {
				existing.Tags = tagsJSON
			}
			if err := s.eventRepo.Update(existing); err != nil {
				log.Printf("[AlertService] Failed to update existing alert ID=%d: %v", existing.ID, err)
				return err
			}

			// 如果是恢复告警，发送恢复通知
			if existing.IsRecovered {
				log.Printf("[AlertService] Existing alert ID=%d is recovered, sending recovery notification", existing.ID)
				strategies, _ := s.strategyMatcher.Match(existing, existing.DepartmentID)
				if len(strategies) > 0 {
					s.alertNotifier.SendRecoveryNotification(existing, strategies, notificationManager)
				} else {
					log.Printf("[AlertService] No strategies matched for recovery notification of alert ID=%d", existing.ID)
				}
			} else {
				log.Printf("[AlertService] Existing alert ID=%d is not recovered, skipping notification (already notified before)", existing.ID)
			}
			continue
		}

		log.Printf("[AlertService] Creating new alert with fingerprint %s", fingerprint)

		// 如果 existing 为 nil 或 ID 为 0，说明是新告警，继续创建新事件

		// 创建新事件
		alertTitle := alertItem.Labels["alertname"]
		if alertTitle == "" {
			alertTitle = "Unknown Alert"
		}

		description := alertItem.Annotations["description"]
		if description == "" {
			description = alertItem.Annotations["message"]
		}

		annotationsJSON, _ := json.Marshal(alertItem.Annotations)
		tagsJSON, _ := json.Marshal(alertItem.Labels)

		// 从标签中提取部门ID（如果存在）
		departmentID := ""
		if deptIDStr, ok := alertItem.Labels["department_id"]; ok {
			departmentID = deptIDStr
		}

		event := &model.AlertEvent{
			AlertTitle:       alertTitle,
			SourceID:         sourceID,
			Description:      description,
			Level:            0, // 默认等级，需要从标签中提取
			FirstTriggerTime: &now,
			TriggerTime:      &now,
			Annotations:      annotationsJSON,
			Tags:             tagsJSON,
			FingerPrint:      fingerprint,
			SourceIP:         sourceIP,
			Progress:         1, // 未认领
			IsRecovered:      alertItem.Status == "resolved",
			DepartmentID:     departmentID,
		}

		if event.IsRecovered {
			event.RecoverTime = &now
		}

		// 检查静默
		if silenced, _ := s.silenceProcessor.CheckSilence(event, departmentID); silenced {
			log.Printf("[AlertService] Event %s (title=%s) is silenced, skipping notification", fingerprint, event.AlertTitle)
			// 静默的告警仍然保存，但不发送通知
			return s.eventRepo.Create(event)
		}

		// 检查抑制
		if restrained, _ := s.restrainProcessor.CheckRestrain(event); restrained {
			log.Printf("[AlertService] Event %s (title=%s) is restrained, skipping notification", fingerprint, event.AlertTitle)
			// 被抑制的告警仍然保存，但不发送通知
			return s.eventRepo.Create(event)
		}

		// 检查聚合
		aggResult, err := s.aggregationProcessor.CheckAggregation(event)
		if err != nil {
			// Redis未启用或其他错误，记录日志但继续处理
			log.Printf("[Alert] Event %s aggregation check failed: %v", fingerprint, err)
			if aggResult != nil && !aggResult.ShouldNotify {
				log.Printf("[Alert] Event %s is aggregated, skipping notification: %s", fingerprint, aggResult.Message)
				// 被聚合的告警仍然保存，但不发送通知
				return s.eventRepo.Create(event)
			}
		} else if aggResult != nil && !aggResult.ShouldNotify {
			log.Printf("[Alert] Event %s is aggregated, skipping notification: %s", fingerprint, aggResult.Message)
			// 被聚合的告警仍然保存，但不发送通知
			return s.eventRepo.Create(event)
		}

		// 保存事件
		if err := s.eventRepo.Create(event); err != nil {
			return err
		}

		// 自动分配告警给值班人员（如果启用了值班排班）
		// 注意：这里需要传入 onCallService，但为了避免循环依赖，我们暂时注释掉
		// 可以在 handler 层调用自动分配功能
		// if onCallService != nil && departmentID != "" {
		// 	go func() {
		// 		if err := onCallService.AutoAssignAlert(event.ID, departmentID); err != nil {
		// 			log.Printf("[Alert] Failed to auto-assign alert: %v", err)
		// 		}
		// 	}()
		// }

		// 匹配策略并发送通知
		if !event.IsRecovered {
			log.Printf("[AlertService] Processing alert event ID=%d, title=%s, departmentID=%s", event.ID, event.AlertTitle, departmentID)
			log.Printf("[AlertService] NotificationManager enabled: %v", notificationManager != nil && notificationManager.IsEnabled())

			strategies, err := s.strategyMatcher.Match(event, departmentID)
			if err != nil {
				log.Printf("[AlertService] Strategy matching error: %v", err)
			} else {
				log.Printf("[AlertService] Matched %d strategies for alert event ID=%d", len(strategies), event.ID)
				for i, strategy := range strategies {
					log.Printf("[AlertService] Strategy[%d]: ID=%d, Name=%s, TemplateID=%d, Delay=%d",
						i, strategy.ID, strategy.StrategyName, strategy.TemplateID, strategy.Delay)
				}
			}

			if err == nil && len(strategies) > 0 {
				// 延迟通知处理
				go func() {
					for _, strategy := range strategies {
						if strategy.Delay > 0 {
							log.Printf("[AlertService] Delaying notification for strategy %d by %d seconds", strategy.ID, strategy.Delay)
							time.Sleep(time.Duration(strategy.Delay) * time.Second)
						}
						log.Printf("[AlertService] Sending notification for strategy %d", strategy.ID)
						s.alertNotifier.SendNotification(event, []model.AlertStrategy{strategy}, notificationManager)
					}
				}()
			} else {
				log.Printf("[AlertService] No strategies matched or error occurred, skipping notification")
			}
		} else {
			log.Printf("[AlertService] Alert event ID=%d is already recovered, skipping notification", event.ID)
		}
	}

	return nil
}

// ClaimEvent 认领告警
func (s *AlertService) ClaimEvent(id uint64, uid string) error {
	if err := s.eventRepo.Claim(id, uid); err != nil {
		return err
	}

	// 记录日志
	log := &model.AlertLog{
		AlertID: id,
		Action:  "claim",
		UID:     uid,
	}
	return s.logRepo.Create(log)
}

// CancelClaimEvent 取消认领
func (s *AlertService) CancelClaimEvent(id uint64, uid string) error {
	if err := s.eventRepo.CancelClaim(id); err != nil {
		return err
	}

	log := &model.AlertLog{
		AlertID: id,
		Action:  "cancel_claim",
		UID:     uid,
	}
	return s.logRepo.Create(log)
}

// CloseEvent 关闭告警
func (s *AlertService) CloseEvent(id uint64, uid string) error {
	if err := s.eventRepo.Close(id, uid); err != nil {
		return err
	}

	log := &model.AlertLog{
		AlertID: id,
		Action:  "closed",
		UID:     uid,
	}
	return s.logRepo.Create(log)
}

// OpenEvent 打开告警
func (s *AlertService) OpenEvent(id uint64, uid string) error {
	if err := s.eventRepo.Open(id); err != nil {
		return err
	}

	log := &model.AlertLog{
		AlertID: id,
		Action:  "opened",
		UID:     uid,
	}
	return s.logRepo.Create(log)
}

// ==================== 告警规则数据源服务方法 ====================

func (s *AlertService) GetRuleSources(page, pageSize int) (int64, []model.AlertRuleSource, error) {
	return s.ruleSourceRepo.List(page, pageSize)
}

func (s *AlertService) GetRuleSource(id uint) (*model.AlertRuleSource, error) {
	return s.ruleSourceRepo.FindByID(id)
}

// GetRuleSourceByAPIKey 根据API密钥获取数据源
func (s *AlertService) GetRuleSourceByAPIKey(apiKey string) (*model.AlertRuleSource, error) {
	return s.ruleSourceRepo.FindByAPIKey(apiKey)
}

func (s *AlertService) CreateRuleSource(source *model.AlertRuleSource) (*model.AlertRuleSource, error) {
	if err := s.ruleSourceRepo.Create(source); err != nil {
		return nil, err
	}

	// 为数据源创建默认规则组，便于同步来的规则或未指定规则组的规则归属
	if _, err := s.GetOrCreateDefaultRuleGroup(source.ID); err != nil {
		log.Printf("[AlertService] Create default rule group for source %d failed: %v", source.ID, err)
	}

	// 如果启用了自动同步，启动定时任务
	if source.AutoSync && s.syncScheduler != nil {
		if err := s.syncScheduler.StartTask(source.ID, source.SourceName, source.SyncInterval); err != nil {
			log.Printf("[AlertService] Failed to start sync task for source %d: %v", source.ID, err)
		}
	}

	return source, nil
}

func (s *AlertService) UpdateRuleSource(id uint, source *model.AlertRuleSource) (*model.AlertRuleSource, error) {
	source.ID = id
	if err := s.ruleSourceRepo.Update(source); err != nil {
		return nil, err
	}

	// 更新定时任务（如果启用了自动同步则启动/更新，否则停止）
	if s.syncScheduler != nil {
		if err := s.syncScheduler.UpdateTask(source); err != nil {
			log.Printf("[AlertService] Failed to update sync task for source %d: %v", id, err)
		}
	}

	return source, nil
}

func (s *AlertService) DeleteRuleSource(id uint) error {
	// 先停止定时任务
	if s.syncScheduler != nil {
		if err := s.syncScheduler.StopTask(id); err != nil {
			log.Printf("[AlertService] Failed to stop sync task for source %d: %v", id, err)
		}
	}

	return s.ruleSourceRepo.Delete(id)
}

func (s *AlertService) GetRuleSourcesByDepartment(departmentID *string) ([]model.AlertRuleSource, error) {
	return s.ruleSourceRepo.ListByDepartment(departmentID)
}

func (s *AlertService) GetRuleSourcesByGroup(groupID *uint) ([]model.AlertRuleSource, error) {
	return s.ruleSourceRepo.ListByGroup(groupID)
}

// SyncRulesFromDatasource 从数据源同步规则到数据库。只使用该数据源的一个默认规则组，不按 Prometheus 的 group 创建多个规则组。
func (s *AlertService) SyncRulesFromDatasource(sourceID uint) error {
	// 获取数据源信息
	source, err := s.ruleSourceRepo.FindByID(sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return scheduler.ErrDatasourceNotFound
		}
		return fmt.Errorf("获取数据源失败: %w", err)
	}

	// 支持 Prometheus、Thanos、VictoriaMetrics（都使用 Prometheus 兼容的 API）
	sourceTypeLower := strings.ToLower(source.SourceType)
	if sourceTypeLower != "prometheus" && sourceTypeLower != "thanos" && sourceTypeLower != "victoriametrics" {
		return fmt.Errorf("不支持的数据源类型: %s，目前支持: prometheus, thanos, victoriametrics", source.SourceType)
	}

	// 从 Prometheus 兼容 API 获取规则
	groups, err := rulefile.GetPrometheusRules(source.Address)
	if err != nil {
		return fmt.Errorf("从 Prometheus 获取规则失败: %w", err)
	}

	log.Printf("从 Prometheus 获取到 %d 个规则组", len(groups))

	if len(groups) == 0 {
		return fmt.Errorf("未从 Prometheus 获取到任何规则组，请检查数据源地址是否正确")
	}

	// 只使用一个默认规则组：已存在则直接复用并追加规则，不存在则创建一条，所有同步规则都归入该组
	defaultGroup, err := s.GetOrCreateDefaultRuleGroup(sourceID)
	if err != nil {
		return fmt.Errorf("获取或创建默认规则组失败: %w", err)
	}

	// 获取该数据源下已有规则（按名称去重，避免跨组重复）
	_, existingRules, err := s.ruleRepo.List(sourceID, nil, nil, "", "", 1, 10000)
	if err != nil {
		return fmt.Errorf("获取现有规则失败: %w", err)
	}
	ruleMap := make(map[string]*model.AlertRule)
	for i := range existingRules {
		ruleMap[existingRules[i].Name] = &existingRules[i]
	}

	var syncedRules int

	// 遍历所有 Prometheus 规则组下的规则，统一归入默认规则组
	for _, promGroup := range groups {
		for _, promRule := range promGroup.Rules {
			// 只处理告警规则（type == "alerting"）
			if promRule.Type != "" && promRule.Type != "alerting" {
				continue
			}

			labelsJSON, _ := json.Marshal(promRule.Labels)
			annotationsJSON, _ := json.Marshal(promRule.Annotations)
			duration := int(promRule.Duration)

			if existingRule, ok := ruleMap[promRule.Name]; ok {
				// 更新现有规则，并归入默认规则组
				existingRule.GroupID = &defaultGroup.ID
				existingRule.Group = defaultGroup.GroupName
				existingRule.Expr = promRule.Query
				existingRule.Duration = duration
				existingRule.Labels = labelsJSON
				existingRule.Annotations = annotationsJSON
				existingRule.Health = promRule.Health
				if err := s.ruleRepo.Update(existingRule); err != nil {
					log.Printf("更新规则失败: %v", err)
				} else {
					syncedRules++
				}
			} else {
				// 创建新规则，归属默认规则组
				rule := &model.AlertRule{
					Name:        promRule.Name,
					GroupID:     &defaultGroup.ID,
					Group:       defaultGroup.GroupName,
					Expr:        promRule.Query,
					Duration:    duration,
					Labels:      labelsJSON,
					Annotations: annotationsJSON,
					Health:      promRule.Health,
					SourceID:    sourceID,
					Enabled:     true,
				}
				if err := s.ruleRepo.Create(rule); err != nil {
					log.Printf("创建规则失败: %v", err)
				} else {
					syncedRules++
					ruleMap[promRule.Name] = rule
				}
			}
		}
	}

	log.Printf("规则同步完成: 使用默认规则组 %s，同步 %d 条规则", defaultGroup.GroupName, syncedRules)
	return nil
}

// ==================== 规则组管理 ====================

func (s *AlertService) GetRuleGroups(departmentID *string) ([]model.AlertRuleGroup, error) {
	return s.ruleGroupRepo.List(departmentID)
}

// DefaultRuleGroupNamePrefix 默认规则组名称前缀，完整名称为 default_<source_id>，保证全局唯一
const DefaultRuleGroupNamePrefix = "default_"

// GetOrCreateDefaultRuleGroup 获取或创建数据源的默认规则组（用于无 group_id 的规则写入、数据源同步等）
func (s *AlertService) GetOrCreateDefaultRuleGroup(sourceID uint) (*model.AlertRuleGroup, error) {
	groupName := fmt.Sprintf("%s%d", DefaultRuleGroupNamePrefix, sourceID)
	group, err := s.ruleGroupRepo.FindBySourceIDAndGroupName(sourceID, groupName)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	source, err := s.ruleSourceRepo.FindByID(sourceID)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}
	group = &model.AlertRuleGroup{
		GroupName:   groupName,
		Description: "该数据源的默认规则组。从 Prometheus 同步的规则会自动归入此组；新建告警规则时若未选其他规则组，也会归属到此组。",
		SourceID:    sourceID,
		File:        "",
		Enabled:     true,
	}
	if err := s.ruleGroupRepo.Create(group); err != nil {
		return nil, err
	}
	group.File = fmt.Sprintf("%s_%d.rules", source.SourceName, group.ID)
	_ = s.ruleGroupRepo.Update(group)
	log.Printf("创建默认规则组: source_id=%d, group_name=%s, file=%s", sourceID, group.GroupName, group.File)
	return group, nil
}

// GetRuleGroupsBySourceID 根据数据源ID获取规则组列表（不分页，用于下拉等）
func (s *AlertService) GetRuleGroupsBySourceID(sourceID uint) ([]model.AlertRuleGroup, error) {
	return s.ruleGroupRepo.ListBySourceID(sourceID)
}

// GetRuleGroupsBySourceIDWithPagination 根据数据源ID分页获取规则组列表
func (s *AlertService) GetRuleGroupsBySourceIDWithPagination(sourceID uint, page, pageSize int) (int64, []model.AlertRuleGroup, error) {
	return s.ruleGroupRepo.ListBySourceIDWithPagination(sourceID, page, pageSize)
}

func (s *AlertService) GetRuleGroup(id uint) (*model.AlertRuleGroup, error) {
	return s.ruleGroupRepo.FindByID(id)
}

func (s *AlertService) CreateRuleGroup(group *model.AlertRuleGroup) (*model.AlertRuleGroup, error) {
	// 验证 SourceID
	if group.SourceID == 0 {
		return nil, fmt.Errorf("数据源ID不能为空")
	}

	// 获取数据源信息
	source, err := s.ruleSourceRepo.FindByID(group.SourceID)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	// 先创建规则组，获取ID
	if err := s.ruleGroupRepo.Create(group); err != nil {
		return nil, err
	}

	// 如果没有指定文件路径，生成默认路径（使用创建后的ID）
	if group.File == "" {
		group.File = fmt.Sprintf("%s_%d.rules", source.SourceName, group.ID)
		// 更新规则组，保存文件路径
		if err := s.ruleGroupRepo.Update(group); err != nil {
			log.Printf("更新规则组文件路径失败: %v", err)
			// 不返回错误，继续执行
		}
	}

	// 同步到规则文件
	if err := s.ruleFileManager.SyncRuleGroupToFile(source, group, "create"); err != nil {
		log.Printf("同步规则组到文件失败: %v", err)
		// 不返回错误，允许规则组创建成功，但记录日志
	}

	return group, nil
}

func (s *AlertService) UpdateRuleGroup(id uint, group *model.AlertRuleGroup) (*model.AlertRuleGroup, error) {
	group.ID = id
	if err := s.ruleGroupRepo.Update(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *AlertService) DeleteRuleGroup(id uint) error {
	// 获取规则组信息
	group, err := s.ruleGroupRepo.FindByID(id)
	if err != nil {
		return err
	}

	// 获取数据源信息
	if group.SourceID > 0 {
		source, err := s.ruleSourceRepo.FindByID(group.SourceID)
		if err == nil {
			// 删除规则文件
			if err := s.ruleFileManager.SyncRuleGroupToFile(source, group, "delete"); err != nil {
				log.Printf("删除规则文件失败: %v", err)
				// 不返回错误，允许规则组删除成功
			}
		}
	}

	return s.ruleGroupRepo.Delete(id)
}

func (s *AlertService) GetRuleGroupsWithPagination(page, pageSize int) (int64, []model.AlertRuleGroup, error) {
	return s.ruleGroupRepo.ListAll(page, pageSize)
}

// ==================== 告警规则服务方法 ====================

func (s *AlertService) GetRules(sourceID uint, groupID *uint, excludeGroupID *uint, group, name string, page, pageSize int) (int64, []model.AlertRule, error) {
	return s.ruleRepo.List(sourceID, groupID, excludeGroupID, group, name, page, pageSize)
}

func (s *AlertService) GetRule(id uint) (*model.AlertRule, error) {
	return s.ruleRepo.FindByID(id)
}

func (s *AlertService) CreateRule(rule *model.AlertRule) (*model.AlertRule, error) {
	// 创建规则
	if err := s.ruleRepo.Create(rule); err != nil {
		return nil, err
	}

	// 同步到规则文件
	if err := s.syncRuleToFile(rule, "create"); err != nil {
		log.Printf("同步规则到文件失败: %v", err)
		// 不返回错误，允许规则创建成功
	}

	return rule, nil
}

func (s *AlertService) UpdateRule(id uint, rule *model.AlertRule) (*model.AlertRule, error) {
	// 获取旧规则信息（用于更新时的规则名称匹配）
	oldRule, err := s.ruleRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	rule.ID = id
	if err := s.ruleRepo.Update(rule); err != nil {
		return nil, err
	}

	// 同步到规则文件
	if err := s.syncRuleToFileWithOldName(rule, oldRule.Name, "update"); err != nil {
		log.Printf("同步规则到文件失败: %v", err)
		// 不返回错误，允许规则更新成功
	}

	return rule, nil
}

func (s *AlertService) DeleteRule(id uint) error {
	// 获取规则信息
	rule, err := s.ruleRepo.FindByID(id)
	if err != nil {
		return err
	}

	// 删除规则
	if err := s.ruleRepo.Delete(id); err != nil {
		return err
	}

	// 同步到规则文件
	if err := s.syncRuleToFile(rule, "delete"); err != nil {
		log.Printf("同步规则到文件失败: %v", err)
		// 不返回错误，允许规则删除成功
	}

	return nil
}

func (s *AlertService) ToggleRule(id uint, enabled bool) error {
	return s.ruleRepo.Toggle(id, enabled)
}

// MoveRuleToDefaultGroup 将规则移出当前规则组，归入该数据源的默认规则组（用于规则组详情页「移出规则组」）
func (s *AlertService) MoveRuleToDefaultGroup(ruleID uint) error {
	rule, err := s.ruleRepo.FindByID(ruleID)
	if err != nil {
		return err
	}
	defaultGroup, err := s.GetOrCreateDefaultRuleGroup(rule.SourceID)
	if err != nil {
		return err
	}
	rule.GroupID = &defaultGroup.ID
	if err := s.ruleRepo.Update(rule); err != nil {
		return err
	}
	if err := s.syncRuleToFile(rule, "update"); err != nil {
		log.Printf("同步规则到文件失败: %v", err)
	}
	return nil
}

// ReloadDatasource 重新加载数据源配置（触发 Prometheus reload）
func (s *AlertService) ReloadDatasource(sourceID uint) error {
	// 获取数据源信息
	source, err := s.ruleSourceRepo.FindByID(sourceID)
	if err != nil {
		return fmt.Errorf("获取数据源失败: %w", err)
	}

	// 调用规则文件管理器的 ReloadDatasource 方法
	if err := s.ruleFileManager.ReloadDatasource(source); err != nil {
		return err
	}

	// Reload 成功后，等待一小段时间让 Prometheus 重新加载配置
	// 然后同步规则以更新健康状态
	time.Sleep(2 * time.Second)

	// 同步规则以更新健康状态
	if err := s.SyncRulesFromDatasource(sourceID); err != nil {
		log.Printf("Reload 后同步规则失败（不影响 reload 结果）: %v", err)
		// 不返回错误，因为 reload 已经成功
	}

	return nil
}

// syncRuleToFile 同步规则到文件（辅助方法）；若规则无 group_id 则使用该数据源的默认规则组
func (s *AlertService) syncRuleToFile(rule *model.AlertRule, operation string) error {
	var group *model.AlertRuleGroup
	var err error
	if rule.GroupID != nil {
		group, err = s.ruleGroupRepo.FindByID(*rule.GroupID)
		if err != nil {
			return fmt.Errorf("获取规则组失败: %w", err)
		}
	} else {
		// 无规则组时使用数据源默认规则组（如从 Prometheus 同步来的规则尚未关联规则组）
		group, err = s.GetOrCreateDefaultRuleGroup(rule.SourceID)
		if err != nil {
			return fmt.Errorf("获取默认规则组失败: %w", err)
		}
		rule.GroupID = &group.ID
		rule.Group = group.GroupName
		if err := s.ruleRepo.Update(rule); err != nil {
			log.Printf("更新规则 group_id 失败: %v", err)
		}
	}

	source, err := s.ruleSourceRepo.FindByID(rule.SourceID)
	if err != nil {
		return fmt.Errorf("获取数据源失败: %w", err)
	}

	return s.ruleFileManager.SyncRuleToFile(source, group, rule, operation)
}

// syncRuleToFileWithOldName 同步规则到文件（带旧规则名称，用于更新）；若规则无 group_id 则使用默认规则组
func (s *AlertService) syncRuleToFileWithOldName(rule *model.AlertRule, oldRuleName string, operation string) error {
	var group *model.AlertRuleGroup
	var err error
	if rule.GroupID != nil {
		group, err = s.ruleGroupRepo.FindByID(*rule.GroupID)
		if err != nil {
			return fmt.Errorf("获取规则组失败: %w", err)
		}
	} else {
		group, err = s.GetOrCreateDefaultRuleGroup(rule.SourceID)
		if err != nil {
			return fmt.Errorf("获取默认规则组失败: %w", err)
		}
		rule.GroupID = &group.ID
		rule.Group = group.GroupName
		if err := s.ruleRepo.Update(rule); err != nil {
			log.Printf("更新规则 group_id 失败: %v", err)
		}
	}

	source, err := s.ruleSourceRepo.FindByID(rule.SourceID)
	if err != nil {
		return fmt.Errorf("获取数据源失败: %w", err)
	}

	// 对于更新操作，需要特殊处理
	if operation == "update" {
		// 如果规则名称没有改变，直接更新
		if rule.Name == oldRuleName {
			return s.ruleFileManager.SyncRuleToFileWithOldName(source, group, rule, oldRuleName, operation)
		}
		// 如果规则名称改变了，先删除旧规则，再创建新规则
		tempRule := *rule
		tempRule.Name = oldRuleName
		// 先删除旧规则
		if err := s.ruleFileManager.SyncRuleToFile(source, group, &tempRule, "delete"); err != nil {
			// 如果删除失败（可能规则不存在），记录日志但继续
			log.Printf("删除旧规则失败（可能规则不存在）: %v", err)
		}
		// 再创建新规则
		return s.ruleFileManager.SyncRuleToFile(source, group, rule, "create")
	}

	return s.ruleFileManager.SyncRuleToFile(source, group, rule, operation)
}

// ==================== 告警事件服务方法 ====================

func (s *AlertService) GetEvents(departmentID string, level, progress int, title, timeRange string, page, pageSize int) (int64, []model.AlertEvent, error) {
	return s.eventRepo.List(departmentID, level, progress, title, timeRange, page, pageSize)
}

func (s *AlertService) GetEvent(id uint64) (*model.AlertEvent, error) {
	return s.eventRepo.FindByID(id)
}

// ==================== 告警策略服务方法 ====================

func (s *AlertService) GetStrategies(departmentID string, status string, page, pageSize int) (int64, []model.AlertStrategy, error) {
	return s.strategyRepo.List(departmentID, status, page, pageSize)
}

func (s *AlertService) GetStrategy(id uint) (*model.AlertStrategy, error) {
	return s.strategyRepo.FindByID(id)
}

func (s *AlertService) CreateStrategy(strategy *model.AlertStrategy) (*model.AlertStrategy, error) {
	if err := s.strategyRepo.Create(strategy); err != nil {
		return nil, err
	}
	return strategy, nil
}

func (s *AlertService) UpdateStrategy(id uint, strategy *model.AlertStrategy) (*model.AlertStrategy, error) {
	strategy.ID = id
	if err := s.strategyRepo.Update(strategy); err != nil {
		return nil, err
	}
	return strategy, nil
}

func (s *AlertService) DeleteStrategy(id uint) error {
	return s.strategyRepo.Delete(id)
}

func (s *AlertService) ToggleStrategy(id uint, status string) error {
	return s.strategyRepo.Toggle(id, status)
}

// ==================== 告警等级服务方法 ====================

func (s *AlertService) GetLevels() ([]model.AlertLevel, error) {
	return s.levelRepo.List()
}

// ==================== 告警静默服务方法 ====================

func (s *AlertService) GetSilences(departmentID string, page, pageSize int) (int64, []model.AlertSilence, error) {
	if departmentID != "" {
		silences, err := s.silenceRepo.ListByDepartment(departmentID)
		if err != nil {
			return 0, nil, err
		}
		total := int64(len(silences))
		// 简单分页
		start := (page - 1) * pageSize
		end := start + pageSize
		if start >= len(silences) {
			return total, []model.AlertSilence{}, nil
		}
		if end > len(silences) {
			end = len(silences)
		}
		return total, silences[start:end], nil
	}
	// 如果没有 departmentID，使用通用 List 方法
	return s.silenceRepo.List(page, pageSize)
}

func (s *AlertService) GetSilence(id uint) (*model.AlertSilence, error) {
	return s.silenceRepo.FindByID(id)
}

func (s *AlertService) CreateSilence(silence *model.AlertSilence) (*model.AlertSilence, error) {
	if err := s.silenceRepo.Create(silence); err != nil {
		return nil, err
	}
	return silence, nil
}

func (s *AlertService) UpdateSilence(id uint, silence *model.AlertSilence) (*model.AlertSilence, error) {
	silence.ID = id
	if err := s.silenceRepo.Update(silence); err != nil {
		return nil, err
	}
	return silence, nil
}

func (s *AlertService) DeleteSilence(id uint) error {
	return s.silenceRepo.Delete(id)
}

// ==================== 告警聚合服务方法 ====================

func (s *AlertService) GetAggregations(page, pageSize int) (int64, []model.AlertAggregation, error) {
	return s.aggregationRepo.List(page, pageSize)
}

func (s *AlertService) GetAggregation(id uint) (*model.AlertAggregation, error) {
	return s.aggregationRepo.FindByID(id)
}

func (s *AlertService) CreateAggregation(agg *model.AlertAggregation) (*model.AlertAggregation, error) {
	if err := s.aggregationRepo.Create(agg); err != nil {
		return nil, err
	}
	return agg, nil
}

func (s *AlertService) UpdateAggregation(id uint, agg *model.AlertAggregation) (*model.AlertAggregation, error) {
	agg.ID = id
	if err := s.aggregationRepo.Update(agg); err != nil {
		return nil, err
	}
	return agg, nil
}

func (s *AlertService) DeleteAggregation(id uint) error {
	return s.aggregationRepo.Delete(id)
}

// ==================== 告警抑制服务方法 ====================

func (s *AlertService) GetRestrains(page, pageSize int) (int64, []model.AlertRestrain, error) {
	return s.restrainRepo.List(page, pageSize)
}

func (s *AlertService) GetRestrain(id uint) (*model.AlertRestrain, error) {
	return s.restrainRepo.FindByID(id)
}

func (s *AlertService) CreateRestrain(restrain *model.AlertRestrain) (*model.AlertRestrain, error) {
	if err := s.restrainRepo.Create(restrain); err != nil {
		return nil, err
	}
	return restrain, nil
}

func (s *AlertService) UpdateRestrain(id uint, restrain *model.AlertRestrain) (*model.AlertRestrain, error) {
	restrain.ID = id
	if err := s.restrainRepo.Update(restrain); err != nil {
		return nil, err
	}
	return restrain, nil
}

func (s *AlertService) DeleteRestrain(id uint) error {
	return s.restrainRepo.Delete(id)
}

// ==================== 告警模板服务方法 ====================

func (s *AlertService) GetTemplates(page, pageSize int) (int64, []model.AlertTemplate, error) {
	return s.templateRepo.ListWithPagination(page, pageSize)
}

func (s *AlertService) GetTemplate(id uint) (*model.AlertTemplate, error) {
	return s.templateRepo.FindByID(id)
}

func (s *AlertService) CreateTemplate(template *model.AlertTemplate) (*model.AlertTemplate, error) {
	if err := s.templateRepo.Create(template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *AlertService) UpdateTemplate(id uint, template *model.AlertTemplate) (*model.AlertTemplate, error) {
	template.ID = id
	if err := s.templateRepo.Update(template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *AlertService) DeleteTemplate(id uint) error {
	// 删除模板时，同时删除关联的渠道模板
	if err := s.channelTemplateRepo.DeleteByTemplateID(id); err != nil {
		return err
	}
	return s.templateRepo.Delete(id)
}

// ==================== 渠道模板内容服务方法 ====================

// GetChannelTemplates 获取模板的所有渠道模板内容
func (s *AlertService) GetChannelTemplates(templateID uint) ([]model.ChannelTemplate, error) {
	return s.channelTemplateRepo.ListByTemplateID(templateID)
}

// UpdateChannelTemplate 更新或创建渠道模板内容
func (s *AlertService) UpdateChannelTemplate(templateID, channelID uint, content string, finished bool) (*model.ChannelTemplate, error) {
	// 查找是否已存在
	existing, err := s.channelTemplateRepo.FindByTemplateIDAndChannelID(templateID, channelID)
	if err != nil && existing == nil {
		return nil, err
	}

	if existing != nil {
		// 更新
		existing.Content = content
		existing.Finished = finished
		if err := s.channelTemplateRepo.Update(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// 创建
	channelTemplate := &model.ChannelTemplate{
		TemplateID: &templateID,
		ChannelID:  &channelID,
		Content:    content,
		Finished:   finished,
	}
	if err := s.channelTemplateRepo.Create(channelTemplate); err != nil {
		return nil, err
	}
	return channelTemplate, nil
}

// DeleteChannelTemplate 删除渠道模板内容
func (s *AlertService) DeleteChannelTemplate(templateID, channelID uint) error {
	return s.channelTemplateRepo.DeleteByTemplateIDAndChannelID(templateID, channelID)
}

// ==================== 告警渠道服务方法 ====================

func (s *AlertService) GetChannels(page, pageSize int) (int64, []model.AlertChannel, error) {
	return s.channelRepo.ListWithPagination(page, pageSize)
}

func (s *AlertService) GetChannel(id uint) (*model.AlertChannel, error) {
	return s.channelRepo.FindByID(id)
}

func (s *AlertService) CreateChannel(channel *model.AlertChannel) (*model.AlertChannel, error) {
	if err := s.channelRepo.Create(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *AlertService) UpdateChannel(id uint, channel *model.AlertChannel) (*model.AlertChannel, error) {
	channel.ID = id
	if err := s.channelRepo.Update(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *AlertService) DeleteChannel(id uint) error {
	return s.channelRepo.Delete(id)
}

// ==================== 策略日志服务方法 ====================

func (s *AlertService) GetStrategyLogs(alertID uint64, page, pageSize int) (int64, []model.StrategyLog, error) {
	return s.strategyLogRepo.ListByAlertIDWithPagination(alertID, page, pageSize)
}

func (s *AlertService) GetStrategyLog(id uint) (*model.StrategyLog, error) {
	return s.strategyLogRepo.FindByID(id)
}

// StartSyncScheduler 启动数据源同步调度器
func (s *AlertService) StartSyncScheduler() error {
	if s.syncScheduler == nil {
		return fmt.Errorf("sync scheduler is not initialized")
	}
	return s.syncScheduler.Start()
}

// StopSyncScheduler 停止数据源同步调度器
func (s *AlertService) StopSyncScheduler() {
	if s.syncScheduler != nil {
		s.syncScheduler.Stop()
	}
}

// ==================== 告警组服务方法 ====================

func (s *AlertService) GetAlertGroups(departmentID *string, page, pageSize int) (int64, []model.AlertGroup, error) {
	return s.alertGroupRepo.List(departmentID, page, pageSize)
}

func (s *AlertService) GetAlertGroup(id uint) (*model.AlertGroup, error) {
	return s.alertGroupRepo.FindByID(id)
}

func (s *AlertService) CreateAlertGroup(group *model.AlertGroup) (*model.AlertGroup, error) {
	if err := s.alertGroupRepo.Create(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *AlertService) UpdateAlertGroup(id uint, group *model.AlertGroup) (*model.AlertGroup, error) {
	group.ID = id
	if err := s.alertGroupRepo.Update(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *AlertService) DeleteAlertGroup(id uint) error {
	return s.alertGroupRepo.Delete(id)
}

func (s *AlertService) GetAllAlertGroups(departmentID *string) ([]model.AlertGroup, error) {
	return s.alertGroupRepo.ListAll(departmentID)
}

// ==================== 告警统计服务方法 ====================

// GetStatistics 获取告警统计信息
func (s *AlertService) GetStatistics(timeRange string) (map[string]interface{}, error) {
	return s.eventRepo.GetStatistics(timeRange)
}

// GetTrendStatistics 获取告警趋势统计
func (s *AlertService) GetTrendStatistics(timeRange string) ([]map[string]interface{}, error) {
	return s.eventRepo.GetTrendStatistics(timeRange)
}

// GetTopAlerts 获取Top N告警
func (s *AlertService) GetTopAlerts(timeRange string, limit int) ([]map[string]interface{}, error) {
	return s.eventRepo.GetTopAlerts(timeRange, limit)
}
