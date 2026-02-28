package matcher

import (
	"encoding/json"
	"log"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// StrategyMatcher 策略匹配器
type StrategyMatcher struct {
	strategyRepo *repository.AlertStrategyRepository
}

// NewStrategyMatcher 创建策略匹配器
func NewStrategyMatcher(strategyRepo *repository.AlertStrategyRepository) *StrategyMatcher {
	return &StrategyMatcher{strategyRepo: strategyRepo}
}

// Match 匹配告警策略
func (m *StrategyMatcher) Match(event *model.AlertEvent, departmentID string) ([]model.AlertStrategy, error) {
	_, strategies, err := m.strategyRepo.List(departmentID, "enabled", 1, 1000)
	if err != nil {
		return nil, err
	}

	var matchedStrategies []model.AlertStrategy
	var tags map[string]string
	if err := json.Unmarshal(event.Tags, &tags); err != nil {
		return nil, err
	}

	log.Printf("[StrategyMatcher] Matching strategies for alert: title=%s, tags=%v, departmentID=%s, total strategies=%d", event.AlertTitle, tags, departmentID, len(strategies))

	now := time.Now()
	for _, strategy := range strategies {
		if m.matchStrategyFilters(strategy, tags, event.AlertTitle, now) {
			log.Printf("[StrategyMatcher] Strategy %d (name=%s) matched!", strategy.ID, strategy.StrategyName)
			matchedStrategies = append(matchedStrategies, strategy)
			if !strategy.Continuous {
				break // 不接续匹配，找到第一个就停止
			}
		}
	}
	
	log.Printf("[StrategyMatcher] Total matched strategies: %d", len(matchedStrategies))
	return matchedStrategies, nil
}

// matchStrategyFilters 匹配策略过滤器
func (m *StrategyMatcher) matchStrategyFilters(strategy model.AlertStrategy, tags map[string]string, alertTitle string, now time.Time) bool {
	// 解析策略详情
	var strategySet []struct {
		StrategyName string `json:"strategy_name"`
		TimeSlot     struct {
			Enable bool     `json:"enable"`
			Weeks  []int    `json:"weeks,omitempty"`
			Times  []string `json:"times,omitempty"`
		} `json:"time_slot"`
		Filters []struct {
			Tag    string   `json:"tag"`
			Values []string `json:"values"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(strategy.StrategySet, &strategySet); err != nil {
		return false
	}

	if len(strategySet) == 0 {
		return false
	}

	// 检查每个策略项
	for _, item := range strategySet {
		// 检查时间段
		if item.TimeSlot.Enable {
			// 检查周期
			if len(item.TimeSlot.Weeks) > 0 {
				weekDay := int(now.Weekday())
				if !containsInt(item.TimeSlot.Weeks, weekDay) {
					continue
				}
			}

			// 检查时间范围
			if len(item.TimeSlot.Times) == 2 {
				if !m.isInTimeRange(item.TimeSlot.Times, now) {
					continue
				}
			}
		}

		// 检查标签匹配
		if len(item.Filters) > 0 {
			pass := false
			for _, filter := range item.Filters {
				matched := m.matchFilter(filter.Tag, filter.Values, tags, alertTitle)
				if matched {
					pass = true
					break
				}
			}
			if !pass {
				continue
			}
		}

		return true
	}

	return false
}

// matchFilter 匹配单个过滤器
func (m *StrategyMatcher) matchFilter(tag string, values []string, tags map[string]string, alertTitle string) bool {
	if tag == "alertname" || tag == "__alertname__" {
		return contains(values, alertTitle)
	}

	tagValue, exists := tags[tag]
	if !exists {
		return false
	}

	return contains(values, tagValue)
}

// isInTimeRange 检查时间是否在当天的范围内
func (m *StrategyMatcher) isInTimeRange(times []string, now time.Time) bool {
	if len(times) < 2 {
		return false
	}

	loc := now.Location()
	startTime, err := time.ParseInLocation("15:04:05", times[0], loc)
	if err != nil {
		return false
	}

	endTime, err := time.ParseInLocation("15:04:05", times[1], loc)
	if err != nil {
		return false
	}

	finalStartTime := time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), startTime.Second(), 0, loc)
	finalEndTime := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), endTime.Second(), 0, loc)

	return !now.Before(finalStartTime) && !now.After(finalEndTime)
}

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsInt 检查整数是否在切片中
func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

