package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/redis"
)

// AggregationProcessor 聚合处理器
type AggregationProcessor struct {
	aggregationRepo *repository.AlertAggregationRepository
	eventRepo       *repository.AlertEventRepository
}

// AggregationResult 聚合结果
type AggregationResult struct {
	ShouldNotify bool   // 是否应该发送通知
	InWindow     bool   // 是否在聚合窗口内
	Message      string // 结果说明
}

// NewAggregationProcessor 创建聚合处理器
func NewAggregationProcessor(
	aggregationRepo *repository.AlertAggregationRepository,
	eventRepo *repository.AlertEventRepository,
) *AggregationProcessor {
	return &AggregationProcessor{
		aggregationRepo: aggregationRepo,
		eventRepo:       eventRepo,
	}
}

// CheckAggregation 检查告警是否需要聚合
// 返回值：shouldNotify（是否通知）, inWindow（是否在窗口内）, error
func (p *AggregationProcessor) CheckAggregation(event *model.AlertEvent) (*AggregationResult, error) {
	// 检查Redis是否可用
	if !redis.IsEnabled() {
		return &AggregationResult{
			ShouldNotify: true, // 没有Redis时，默认通知（不进行聚合）
			InWindow:     false,
			Message:      "Redis未启用，告警聚合功能不可用，将直接发送通知",
		}, fmt.Errorf("告警聚合功能需要Redis支持，当前Redis未启用。请在配置文件中启用Redis（config.yaml -> redis.enabled: true）")
	}

	// 获取所有启用的聚合规则
	aggregations, err := p.aggregationRepo.ListEnabled()
	if err != nil {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     false,
			Message:      fmt.Sprintf("获取聚合规则失败: %v", err),
		}, err
	}

	// 如果没有聚合规则，直接通知
	if len(aggregations) == 0 {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     false,
			Message:      "无匹配的聚合规则，直接发送通知",
		}, nil
	}

	// 获取所有未关闭的告警事件
	unclosedEvents, err := p.eventRepo.ListUnclosed()
	if err != nil {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     false,
			Message:      fmt.Sprintf("获取未关闭告警失败: %v", err),
		}, err
	}

	// 遍历聚合规则，检查是否有匹配的
	for _, agg := range aggregations {
		if agg.Status != "enabled" {
			continue
		}

		// 检查是否匹配聚合规则
		matchedEvent := p.findMatchingEvent(event, unclosedEvents, &agg)
		if matchedEvent == nil {
			continue
		}

		// 检查聚合窗口
		return p.checkAggregationWindow(matchedEvent.ID, &agg)
	}

	// 没有匹配的聚合规则，直接通知
	return &AggregationResult{
		ShouldNotify: true,
		InWindow:     false,
		Message:      "无匹配的聚合规则，直接发送通知",
	}, nil
}

// findMatchingEvent 查找匹配的未关闭告警事件
func (p *AggregationProcessor) findMatchingEvent(
	newEvent *model.AlertEvent,
	unclosedEvents []model.AlertEvent,
	agg *model.AlertAggregation,
) *model.AlertEvent {
	var tags map[string]string
	if err := json.Unmarshal(newEvent.Tags, &tags); err != nil {
		return nil
	}

	for _, event := range unclosedEvents {
		// 检查标题维度
		if agg.TitleDimension {
			if newEvent.AlertTitle != event.AlertTitle {
				continue
			}
		}

		// 检查等级维度
		if agg.LevelDimension {
			if newEvent.Level != event.Level {
				continue
			}
		}

		// 检查标签维度
		if len(agg.TagsDimension) > 0 {
			var eventTags map[string]string
			if err := json.Unmarshal(event.Tags, &eventTags); err != nil {
				continue
			}

			var tagsDimension []map[string]interface{}
			if err := json.Unmarshal(agg.TagsDimension, &tagsDimension); err != nil {
				continue
			}

			matched := false
			for _, tagRule := range tagsDimension {
				if p.matchTagRule(tags, eventTags, tagRule) {
					matched = true
					break
				}
			}

			if !matched {
				continue
			}
		}

		// 所有维度都匹配
		return &event
	}

	return nil
}

// matchTagRule 匹配标签规则
func (p *AggregationProcessor) matchTagRule(
	newTags map[string]string,
	eventTags map[string]string,
	tagRule map[string]interface{},
) bool {
	// tagRule格式: {"tag": "environment", "values": ["production"]}
	tagName, ok := tagRule["tag"].(string)
	if !ok {
		return false
	}

	values, ok := tagRule["values"].([]interface{})
	if !ok {
		return false
	}

	newValue, newExists := newTags[tagName]
	eventValue, eventExists := eventTags[tagName]

	// 两个事件的标签值必须都存在且相同
	if !newExists || !eventExists {
		return false
	}

	if newValue != eventValue {
		return false
	}

	// 检查值是否在规则中
	for _, v := range values {
		if vStr, ok := v.(string); ok && vStr == newValue {
			return true
		}
	}

	return false
}

// checkAggregationWindow 检查聚合窗口
func (p *AggregationProcessor) checkAggregationWindow(eventID uint64, agg *model.AlertAggregation) (*AggregationResult, error) {
	if agg.Windows <= 0 {
		// 没有设置窗口，直接通知
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     false,
			Message:      "聚合规则未设置窗口时间，直接发送通知",
		}, nil
	}

	ctx := context.Background()
	windowKey := fmt.Sprintf("alert:aggregation:window:%d", eventID)
	redisClient := redis.GetClient()

	// 检查窗口是否存在
	exists, err := redisClient.Exists(ctx, windowKey).Result()
	if err != nil {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     false,
			Message:      fmt.Sprintf("检查聚合窗口失败: %v", err),
		}, err
	}

	if exists == 0 {
		// 窗口不存在，创建新窗口
		windowDuration := time.Duration(agg.Windows) * time.Second
		err := redisClient.Set(ctx, windowKey, 1, windowDuration).Err()
		if err != nil {
			return &AggregationResult{
				ShouldNotify: true,
				InWindow:     false,
				Message:      fmt.Sprintf("创建聚合窗口失败: %v", err),
			}, err
		}

		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     true,
			Message:      fmt.Sprintf("创建新的聚合窗口（%d秒），发送通知", agg.Windows),
		}, nil
	}

	// 窗口存在，检查风暴预警
	if agg.Storm <= 0 {
		// 没有设置风暴预警，不通知
		return &AggregationResult{
			ShouldNotify: false,
			InWindow:     true,
			Message:      "聚合窗口内，未设置风暴预警阈值，不发送通知",
		}, nil
	}

	// 累加计数
	count, err := redisClient.Incr(ctx, windowKey).Result()
	if err != nil {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     true,
			Message:      fmt.Sprintf("累加聚合计数失败: %v", err),
		}, err
	}

	// 检查是否达到风暴预警阈值
	if count%int64(agg.Storm) == 0 {
		return &AggregationResult{
			ShouldNotify: true,
			InWindow:     true,
			Message:      fmt.Sprintf("聚合窗口内达到风暴预警阈值（当前计数: %d，阈值: %d），发送通知", count, agg.Storm),
		}, nil
	}

	return &AggregationResult{
		ShouldNotify: false,
		InWindow:     true,
		Message:      fmt.Sprintf("聚合窗口内，当前计数: %d，阈值: %d，不发送通知", count, agg.Storm),
	}, nil
}
