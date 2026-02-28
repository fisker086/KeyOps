package aiassistant

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionManager 会话存储（文件）
type SessionManager struct {
	storagePath string
	secretKey   string
}

// NewSessionManager 创建会话管理器
func NewSessionManager(storagePath, secretKey string) (*SessionManager, error) {
	if secretKey == "" {
		secretKey = "default_session_secret"
	}
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, err
	}
	return &SessionManager{storagePath: storagePath, secretKey: secretKey}, nil
}

func (m *SessionManager) sign(u string) string {
	h := hmac.New(sha256.New, []byte(m.secretKey))
	h.Write([]byte(u))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// GenerateSessionID 生成带签名的 session_id
func (m *SessionManager) GenerateSessionID() string {
	u := uuid.New().String()
	return m.sign(u) + "_" + u
}

// IsValidSessionID 校验 session_id 格式与签名
func (m *SessionManager) IsValidSessionID(sessionID string) bool {
	if sessionID == "" || !strings.Contains(sessionID, "_") {
		return false
	}
	parts := strings.SplitN(sessionID, "_", 2)
	if len(parts) != 2 {
		return false
	}
	signature, u := parts[0], parts[1]
	if _, err := uuid.Parse(u); err != nil {
		return false
	}
	expected := m.sign(u)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// CreateSession 创建新会话，modelID/modelName 可选，为空时引擎使用 settings 默认
func (m *SessionManager) CreateSession(task, envID, role, modelID, modelName, scheduleID, createdBy string) (string, error) {
	sessionID := m.GenerateSessionID()
	s := Session{
		SessionID:   sessionID,
		Task:        task,
		EnvID:       envID,
		Role:        role,
		ModelID:     modelID,
		ModelName:   modelName,
		ScheduleID:  scheduleID,
		CreatedBy:   createdBy,
		Status:      "running",
		StartTime:   time.Now().Format(time.RFC3339),
		Steps:       []SessionStep{},
	}
	return sessionID, m.SaveSession(sessionID, &s)
}

// SaveSession 保存会话
func (m *SessionManager) SaveSession(sessionID string, data *Session) error {
	if !m.IsValidSessionID(sessionID) {
		return nil
	}
	fpath := filepath.Join(m.storagePath, sessionID+".json")
	abs, _ := filepath.Abs(fpath)
	base, _ := filepath.Abs(m.storagePath)
	if !strings.HasPrefix(abs, base) {
		return nil
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fpath, raw, 0644)
}

// DeleteSession 删除会话
func (m *SessionManager) DeleteSession(sessionID string) error {
	if !m.IsValidSessionID(sessionID) {
		return nil
	}
	fpath := filepath.Join(m.storagePath, sessionID+".json")
	abs, _ := filepath.Abs(fpath)
	base, _ := filepath.Abs(m.storagePath)
	if !strings.HasPrefix(abs, base) {
		return nil
	}
	return os.Remove(fpath)
}

// GetSession 获取会话
func (m *SessionManager) GetSession(sessionID string) (*Session, error) {
	if !m.IsValidSessionID(sessionID) {
		return nil, nil
	}
	fpath := filepath.Join(m.storagePath, sessionID+".json")
	abs, _ := filepath.Abs(fpath)
	base, _ := filepath.Abs(m.storagePath)
	if !strings.HasPrefix(abs, base) {
		return nil, nil
	}
	raw, err := os.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSessions 列表（可选按 schedule_id、created_by 过滤；createdBy 非空时仅返回该用户创建的会话）
func (m *SessionManager) ListSessions(scheduleID, createdBy string) ([]SessionSummary, error) {
	entries, err := os.ReadDir(m.storagePath)
	if err != nil {
		return nil, err
	}
	var list []SessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		sid := strings.TrimSuffix(e.Name(), ".json")
		s, err := m.GetSession(sid)
		if err != nil || s == nil {
			continue
		}
		if scheduleID != "" && s.ScheduleID != scheduleID {
			continue
		}
		if createdBy != "" && s.CreatedBy != createdBy {
			continue
		}
		list = append(list, SessionSummary{
			SessionID:  s.SessionID,
			Task:       s.Task,
			StartTime:  s.StartTime,
			Status:     s.Status,
			EnvID:      s.EnvID,
			Role:       s.Role,
			ModelID:    s.ModelID,
			ModelName:  s.ModelName,
			ScheduleID: s.ScheduleID,
			CreatedBy:  s.CreatedBy,
		})
	}
	sortSessionsByTime(list)
	return list, nil
}

func sortSessionsByTime(list []SessionSummary) {
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].StartTime > list[i].StartTime {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
}

// ContinueSession 将已完成的会话置为 running，并追加用户 follow-up 步骤，用于多轮对话
func (m *SessionManager) ContinueSession(sessionID, task string) error {
	s, err := m.GetSession(sessionID)
	if err != nil || s == nil {
		return err
	}
	if s.Status != "completed" && s.Status != "error" {
		return nil
	}
	s.Status = "running"
	s.Steps = append(s.Steps, SessionStep{
		Type:      "user_message",
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      map[string]interface{}{"content": task},
	})
	return m.SaveSession(sessionID, s)
}

// AddStep 追加一步并更新状态
func (m *SessionManager) AddStep(sessionID, stepType string, data map[string]interface{}) error {
	s, err := m.GetSession(sessionID)
	if err != nil || s == nil {
		return err
	}
	s.Steps = append(s.Steps, SessionStep{
		Type:      stepType,
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      data,
	})
	if stepType == "final_answer" {
		s.Status = "completed"
		if c, ok := data["content"].(string); ok {
			s.FinalAnswer = c
		}
	} else if stepType == "error" {
		s.Status = "error"
	}
	return m.SaveSession(sessionID, s)
}
