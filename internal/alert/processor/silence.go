package processor

import (
	"encoding/json"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// SilenceProcessor 静默处理器
type SilenceProcessor struct {
	silenceRepo *repository.AlertSilenceRepository
}

// NewSilenceProcessor 创建静默处理器
func NewSilenceProcessor(silenceRepo *repository.AlertSilenceRepository) *SilenceProcessor {
	return &SilenceProcessor{silenceRepo: silenceRepo}
}

// CheckSilence 检查告警是否在静默期内
func (p *SilenceProcessor) CheckSilence(event *model.AlertEvent, departmentID string) (bool, error) {
	if departmentID == "" {
		return false, nil
	}

	silences, err := p.silenceRepo.ListByDepartment(departmentID)
	if err != nil {
		return false, err
	}

	var tags map[string]string
	if err := json.Unmarshal(event.Tags, &tags); err != nil {
		return false, err
	}

	now := time.Now()
	for _, silence := range silences {
		// 检查标签匹配
		if !p.matchSilenceFilters(tags, event.AlertTitle, silence.Filters) {
			continue
		}

		// 检查时间匹配
		var silenceTime struct {
			RangeTime []string `json:"range_time,omitempty"`
			Weeks     []int    `json:"weeks,omitempty"`
			Times     []string `json:"times,omitempty"`
		}
		if err := json.Unmarshal(silence.SilenceTime, &silenceTime); err != nil {
			continue
		}

		if silence.SilenceType == "once" {
			// 单次静默
			if len(silenceTime.RangeTime) >= 2 {
				if p.isInRangeTime(silenceTime.RangeTime, now) {
					return true, nil
				}
			}
		} else if silence.SilenceType == "period" {
			// 周期静默
			if p.isInWeekTime(silenceTime.Weeks, silenceTime.Times, now) {
				return true, nil
			}
		}
	}

	return false, nil
}

// matchSilenceFilters 匹配静默过滤器
func (p *SilenceProcessor) matchSilenceFilters(tags map[string]string, alertTitle string, filtersJSON []byte) bool {
	if len(filtersJSON) == 0 {
		return true // 无过滤器则匹配所有
	}

	var filters []struct {
		Tag    string   `json:"tag"`
		Values []string `json:"values"`
	}
	if err := json.Unmarshal(filtersJSON, &filters); err != nil {
		return false
	}

	for _, filter := range filters {
		if filter.Tag == "alertname" || filter.Tag == "__alertname__" {
			// 匹配告警名称
			if !contains(filter.Values, alertTitle) {
				return false
			}
		} else {
			// 匹配标签
			tagValue, exists := tags[filter.Tag]
			if !exists || !contains(filter.Values, tagValue) {
				return false
			}
		}
	}

	return true
}

// isInRangeTime 检查时间是否在范围内
func (p *SilenceProcessor) isInRangeTime(rangeTime []string, now time.Time) bool {
	if len(rangeTime) < 2 {
		return false
	}

	loc := now.Location()
	start, err := time.ParseInLocation("2006-01-02 15:04:05", rangeTime[0], loc)
	if err != nil {
		return false
	}

	end, err := time.ParseInLocation("2006-01-02 15:04:05", rangeTime[1], loc)
	if err != nil {
		return false
	}

	return now.After(start) && now.Before(end)
}

// isInWeekTime 检查时间是否在周期时间范围内
func (p *SilenceProcessor) isInWeekTime(weeks []int, times []string, now time.Time) bool {
	if len(weeks) == 0 || len(times) < 2 {
		return false
	}

	// 检查星期
	weekDay := int(now.Weekday())
	if !containsInt(weeks, weekDay) {
		return false
	}

	// 检查时间范围
	return p.isInTimeRange(times, now)
}

// isInTimeRange 检查时间是否在当天的范围内
func (p *SilenceProcessor) isInTimeRange(times []string, now time.Time) bool {
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

