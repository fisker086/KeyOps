package aiassistant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScheduleManager 定时任务存储（单文件 JSON）
type ScheduleManager struct {
	filePath string
	mu       sync.RWMutex
}

// NewScheduleManager 创建
func NewScheduleManager(dataDir string) (*ScheduleManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	fpath := filepath.Join(dataDir, "schedules.json")
	return &ScheduleManager{filePath: fpath}, nil
}

func (sm *ScheduleManager) load() (map[string]map[string]interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	raw, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]map[string]interface{}), nil
		}
		return nil, err
	}
	var out map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = make(map[string]map[string]interface{})
	}
	return out, nil
}

func (sm *ScheduleManager) save(data map[string]map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sm.filePath, raw, 0644)
}

// ListSchedules 列表
func (sm *ScheduleManager) ListSchedules() ([]Schedule, error) {
	data, err := sm.load()
	if err != nil {
		return nil, err
	}
	var list []Schedule
	for id, v := range data {
		list = append(list, scheduleFromMap(id, v))
	}
	return list, nil
}

// GetSchedule 获取
func (sm *ScheduleManager) GetSchedule(scheduleID string) (*Schedule, error) {
	data, err := sm.load()
	if err != nil {
		return nil, err
	}
	v, ok := data[scheduleID]
	if !ok {
		return nil, nil
	}
	s := scheduleFromMap(scheduleID, v)
	return &s, nil
}

func scheduleFromMap(id string, v map[string]interface{}) Schedule {
	s := Schedule{ID: id}
	if x, ok := v["name"].(string); ok {
		s.Name = x
	}
	if x, ok := v["env_id"].(string); ok {
		s.EnvID = x
	}
	if x, ok := v["model_id"].(string); ok {
		s.ModelID = x
	}
	if x, ok := v["cron"].(string); ok {
		s.Cron = x
	}
	if x, ok := v["task_prompt"].(string); ok {
		s.TaskPrompt = x
	}
	if x, ok := v["role"].(string); ok {
		s.Role = x
	}
	if x, ok := v["lark_bot_id"].(string); ok {
		s.LarkBotID = x
	}
	if x, ok := v["lark_group_name"].(string); ok {
		s.LarkGroupName = x
	}
	if x, ok := v["lark_folder_id"].(string); ok {
		s.LarkFolderID = x
	}
	if x, ok := v["enabled"].(bool); ok {
		s.Enabled = x
	} else {
		s.Enabled = true
	}
	if x, ok := v["responsible_user"].(string); ok {
		s.ResponsibleUser = x
	}
	if arr, ok := v["notification_channel_ids"].([]interface{}); ok {
		for _, it := range arr {
			if n, ok := toUint(it); ok {
				s.NotificationChannelIDs = append(s.NotificationChannelIDs, n)
			}
		}
	}
	if x, ok := v["created_at"].(string); ok {
		s.CreatedAt = x
	}
	return s
}

func toUint(it interface{}) (uint, bool) {
	switch v := it.(type) {
	case float64:
		if v >= 0 {
			return uint(v), true
		}
	case int:
		if v >= 0 {
			return uint(v), true
		}
	}
	return 0, false
}

// AddSchedule 新增
func (sm *ScheduleManager) AddSchedule(name, envID, modelID, cron, taskPrompt, role, larkBotID, larkGroupName, larkFolderID, responsibleUser string, enabled bool, notificationChannelIDs []uint) (string, error) {
	data, err := sm.load()
	if err != nil {
		return "", err
	}
	scheduleID := uuid.New().String()
	payload := map[string]interface{}{
		"name":             name,
		"env_id":           envID,
		"cron":             cron,
		"task_prompt":      taskPrompt,
		"role":             role,
		"model_id":         modelID,
		"lark_bot_id":      larkBotID,
		"lark_group_name":  larkGroupName,
		"lark_folder_id":   larkFolderID,
		"enabled":          enabled,
		"responsible_user": responsibleUser,
		"created_at":       time.Now().Format(time.RFC3339),
	}
	if len(notificationChannelIDs) > 0 {
		payload["notification_channel_ids"] = notificationChannelIDs
	}
	data[scheduleID] = payload
	return scheduleID, sm.save(data)
}

// UpdateSchedule 更新
func (sm *ScheduleManager) UpdateSchedule(scheduleID string, updates map[string]interface{}) error {
	data, err := sm.load()
	if err != nil {
		return err
	}
	v, ok := data[scheduleID]
	if !ok {
		return nil
	}
	for k, val := range updates {
		if val != nil {
			v[k] = val
		}
	}
	return sm.save(data)
}

// DeleteSchedule 删除
func (sm *ScheduleManager) DeleteSchedule(scheduleID string) error {
	data, err := sm.load()
	if err != nil {
		return err
	}
	delete(data, scheduleID)
	return sm.save(data)
}
