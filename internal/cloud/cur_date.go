package cloud

import (
	"fmt"
	"strings"
	"time"
)

// CURDayFromRow 从 CUR 行字段提取 YYYY-MM-DD（兼容 string / int64 毫秒 / 指针）
func CURDayFromRow(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok {
			if day := CURDayFromValue(v); day != "" {
				return day
			}
		}
	}
	return ""
}

// CURDayFromValue 解析 CUR 日期字段为 YYYY-MM-DD
func CURDayFromValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return curDayFromString(val)
	case *string:
		if val == nil {
			return ""
		}
		return curDayFromString(*val)
	case int64:
		return curMillisToDay(val)
	case *int64:
		if val == nil {
			return ""
		}
		return curMillisToDay(*val)
	case int:
		return curMillisToDay(int64(val))
	case int32:
		return curMillisToDay(int64(val))
	case float64:
		if val == 0 {
			return ""
		}
		return curMillisToDay(int64(val))
	case float32:
		if val == 0 {
			return ""
		}
		return curMillisToDay(int64(val))
	default:
		s := strings.TrimSpace(fmt.Sprint(val))
		if strings.HasPrefix(s, "0x") {
			return ""
		}
		return curDayFromString(s)
	}
}

func curDayFromString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "/"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	return ""
}

func curMillisToDay(ms int64) string {
	if ms <= 0 {
		return ""
	}
	sec := ms
	if ms > 1_000_000_000_000 {
		sec = ms / 1000
	}
	return time.Unix(sec, 0).UTC().Format("2006-01-02")
}

// CURRowInMonth 判断 CUR 行是否落在目标账期月
func CURRowInMonth(row map[string]interface{}, targetMonth string) bool {
	usageKeys := []string{
		"Line_item_usage_start_date",
		"lineItem/UsageStartDate",
	}
	periodKeys := []string{
		"Identity_time_interval",
		"identity/TimeInterval",
		"Bill_billing_period_start_date",
		"bill/BillingPeriodStartDate",
	}
	if day := CURDayFromRow(row, usageKeys...); day != "" {
		return strings.HasPrefix(day, targetMonth)
	}
	if day := CURDayFromRow(row, periodKeys...); day != "" {
		return strings.HasPrefix(day, targetMonth)
	}
	return false
}
