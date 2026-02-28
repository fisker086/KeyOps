package types

import (
	"github.com/fisker086/keyops/internal/model"
)

// TimeSlot 时间段策略
type TimeSlot struct {
	Enable   bool     `json:"enable"`
	Type     string   `json:"type,omitempty"`
	Weeks    []int    `json:"weeks,omitempty"`
	Times    []string `json:"times,omitempty"`
	Calendar *int     `json:"calendar,omitempty"`
}

// StrategyFilter 策略过滤器
type StrategyFilter struct {
	Tag    string   `json:"tag"`
	Op     string   `json:"op"` // eq, ne, in, not_in, regex, not_regex
	Values []string `json:"values"`
}

// StrategySetItem 策略集项
type StrategySetItem struct {
	TimeSlot TimeSlot         `json:"time_slot"`
	Filters  []StrategyFilter `json:"filters"`
}

// SilenceConfig 静默配置
type SilenceConfig struct {
	Type      string   `json:"type"`                 // single, cycle
	RangeTime []string `json:"range_time,omitempty"` // 单次静默: ["2024-01-01 00:00:00", "2024-01-02 00:00:00"]
	Weeks     []int    `json:"weeks,omitempty"`      // 周期静默: [0,1,2,3,4,5,6]
	Times     []string `json:"times,omitempty"`      // 周期静默: ["09:00:00", "18:00:00"]
}

// AlertProcessor 告警处理器接口
type AlertProcessor interface {
	Process(event *model.AlertEvent) error
}

// StrategyMatcher 策略匹配器接口
type StrategyMatcher interface {
	Match(event *model.AlertEvent, departmentID string) ([]model.AlertStrategy, error)
}

// Aggregator 聚合器接口
type Aggregator interface {
	Aggregate(events []*model.AlertEvent) ([]*model.AlertEvent, error)
}
