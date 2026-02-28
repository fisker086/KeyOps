package websocket

import (
	"github.com/fisker086/keyops/pkg/logger"
	"sync"

	"github.com/gorilla/websocket"
)

// SessionManager 管理活跃的 WebSocket 会话
type SessionManager struct {
	sessions sync.Map // sessionID -> *websocket.Conn
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// AddSession 添加会话
func (sm *SessionManager) AddSession(sessionID string, conn *websocket.Conn) {
	sm.sessions.Store(sessionID, conn)
	logger.Infof("[SessionManager] Session added: %s", sessionID)
}

// RemoveSession 移除会话
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.sessions.Delete(sessionID)
	logger.Infof("[SessionManager] Session removed: %s", sessionID)
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(sessionID string) (*websocket.Conn, bool) {
	if conn, ok := sm.sessions.Load(sessionID); ok {
		return conn.(*websocket.Conn), true
	}
	return nil, false
}

// TerminateSession 终止会话
func (sm *SessionManager) TerminateSession(sessionID string) error {
	conn, ok := sm.GetSession(sessionID)
	if !ok {
		logger.Infof("[SessionManager] Session not found: %s", sessionID)
		return nil // 会话不存在，可能已经关闭
	}

	// 发送关闭消息给客户端
	closeMsg := map[string]interface{}{
		"type":    "error",
		"message": "会话已被管理员终止",
	}

	conn.WriteJSON(closeMsg)

	// 关闭 WebSocket 连接
	err := conn.Close()
	if err != nil {
		logger.Infof("[SessionManager] Error closing session %s: %v", sessionID, err)
	}

	// 从管理器中移除
	sm.RemoveSession(sessionID)

	logger.Infof("[SessionManager] Session terminated: %s", sessionID)
	return err
}

// GetActiveSessions 获取所有活跃会话ID
func (sm *SessionManager) GetActiveSessions() []string {
	var sessions []string
	sm.sessions.Range(func(key, value interface{}) bool {
		sessions = append(sessions, key.(string))
		return true
	})
	return sessions
}

// GetActiveSessionCount 获取活跃会话数量
func (sm *SessionManager) GetActiveSessionCount() int {
	count := 0
	sm.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
