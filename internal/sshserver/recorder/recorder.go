package recorder

import (
	"fmt"
	"github.com/fisker086/keyops/pkg/logger"
	"sync"
	"time"

	"github.com/fisker086/keyops/internal/bastion/recorder"
	"github.com/fisker086/keyops/internal/bastion/storage"
	"github.com/fisker086/keyops/internal/sshserver/types"
)

// AdapterRecorder 适配器模式：复用现有的Recorder
type AdapterRecorder struct {
	storage   storage.Storage
	recorders map[string]*recorder.Recorder // sessionID -> recorder
	mu        sync.RWMutex
}

// NewAdapterRecorder 创建适配器录制器
func NewAdapterRecorder(storage storage.Storage) types.SessionRecorder {
	return &AdapterRecorder{
		storage:   storage,
		recorders: make(map[string]*recorder.Recorder),
	}
}

// RecordStart 记录会话开始
func (r *AdapterRecorder) RecordStart(session *types.SessionInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.Infof("[Recorder] Starting recording for session: %s", session.SessionID)

	// 创建新的录制器
	rec := recorder.NewRecorder(session.SessionID, session.TerminalCols, session.TerminalRows)
	r.recorders[session.SessionID] = rec

	logger.Infof("[Recorder] Recording started for session: %s (cols: %d, rows: %d)",
		session.SessionID, session.TerminalCols, session.TerminalRows)
}

// RecordData 记录数据
func (r *AdapterRecorder) RecordData(sessionID string, direction string, data []byte) {
	r.mu.RLock()
	rec, exists := r.recorders[sessionID]
	r.mu.RUnlock()

	if !exists {
		logger.Infof("[Recorder] Warning: No recorder found for session: %s", sessionID)
		return
	}

	// 根据方向记录数据
	switch direction {
	case "in":
		rec.RecordInput(string(data))
	case "out":
		rec.RecordOutput(string(data))
	default:
		logger.Infof("[Recorder] Unknown direction: %s", direction)
	}
}

// RecordEnd 记录会话结束
func (r *AdapterRecorder) RecordEnd(sessionID string, endTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.Infof("[Recorder] Ending recording for session: %s", sessionID)

	rec, exists := r.recorders[sessionID]
	if !exists {
		logger.Infof("[Recorder] Warning: No recorder found for session: %s", sessionID)
		return
	}

	// 关闭录制器
	rec.Close()

	// 生成asciinema格式
	asciinemaData, err := rec.ToAsciinema()
	if err != nil {
		logger.Infof("[Recorder] Failed to convert to asciinema format: %v", err)
		return
	}

	// 保存录制数据到存储（使用CloseSession方法）
	if err := r.storage.CloseSession(sessionID, asciinemaData); err != nil {
		logger.Infof("[Recorder] Failed to save recording: %v", err)
		return
	}

	logger.Infof("[Recorder] Recording saved for session: %s (events: %d)",
		sessionID, rec.GetEventCount())

	// 清理录制器
	delete(r.recorders, sessionID)
}

// RecordError 记录错误
func (r *AdapterRecorder) RecordError(sessionID string, errMsg string) {
	logger.Infof("[Recorder] Error for session %s: %s", sessionID, errMsg)

	// 可以将错误信息也记录到录制中
	r.RecordData(sessionID, "out", []byte("\r\n[ERROR] "+errMsg+"\r\n"))
}

// GetRecording 获取录制内容
func (r *AdapterRecorder) GetRecording(sessionID string) (string, error) {
	// 从数据库读取录制内容
	// 注意：storage.Storage接口没有GetRecording方法
	// 这里返回空字符串和错误，如果需要实现可以直接查询数据库
	logger.Infof("[Recorder] GetRecording called for session: %s (not implemented)", sessionID)
	return "", fmt.Errorf("GetRecording not implemented yet")
}

// Close 关闭录制器
func (r *AdapterRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.Infof("[Recorder] Closing all recorders (%d active)", len(r.recorders))

	// 关闭所有活动的录制器
	for sessionID, rec := range r.recorders {
		rec.Close()
		logger.Infof("[Recorder] Closed recorder for session: %s", sessionID)
	}

	// 清空映射
	r.recorders = make(map[string]*recorder.Recorder)

	// 关闭存储
	if r.storage != nil {
		return r.storage.Close()
	}

	return nil
}
