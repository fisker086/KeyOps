package bill

import (
	"encoding/json"
	"time"
)

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(json.Number); ok {
		f, _ := v.Float64()
		return f
	}
	return 0
}

func getMonthRange(t time.Time) (time.Time, time.Time) {
	year := t.Year()
	month := t.Month()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)
	return firstDay, lastDay
}
