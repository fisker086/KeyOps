package processor

import (
	"encoding/json"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

// RestrainProcessor 抑制处理器
type RestrainProcessor struct {
	restrainRepo *repository.AlertRestrainRepository
	eventRepo    *repository.AlertEventRepository
}

// NewRestrainProcessor 创建抑制处理器
func NewRestrainProcessor(
	restrainRepo *repository.AlertRestrainRepository,
	eventRepo *repository.AlertEventRepository,
) *RestrainProcessor {
	return &RestrainProcessor{
		restrainRepo: restrainRepo,
		eventRepo:    eventRepo,
	}
}

// CheckRestrain 检查告警是否被抑制
func (p *RestrainProcessor) CheckRestrain(event *model.AlertEvent) (bool, error) {
	restrains, err := p.restrainRepo.ListAll()
	if err != nil {
		return false, err
	}

	var tags map[string]string
	if err := json.Unmarshal(event.Tags, &tags); err != nil {
		return false, err
	}

	for _, restrain := range restrains {
		if p.matchRestrainFields(tags, event.AlertTitle, restrain.Fields) {
			// 检查是否有其他告警在抑制窗口内
			windowStart := time.Now().Add(-time.Duration(restrain.CumulativeTime) * time.Second)
			similarEvents, err := p.eventRepo.FindSimilarEvents(event.FingerPrint, windowStart)
			if err == nil && len(similarEvents) > 0 {
				return true, nil
			}
		}
	}

	return false, nil
}

// matchRestrainFields 匹配抑制字段
func (p *RestrainProcessor) matchRestrainFields(tags map[string]string, alertTitle string, fieldsJSON []byte) bool {
	if len(fieldsJSON) == 0 {
		return false
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(fieldsJSON, &fields); err != nil {
		return false
	}

	// 检查字段匹配逻辑
	for key, value := range fields {
		if key == "alertname" || key == "__alertname__" {
			if alertTitle != value {
				return false
			}
		} else {
			tagValue, exists := tags[key]
			if !exists || tagValue != value {
				return false
			}
		}
	}

	return true
}

