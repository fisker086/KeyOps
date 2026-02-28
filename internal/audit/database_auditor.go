package audit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DatabaseAuditor 统一的数据库审计器
// 支持SSH Gateway和WebShell两种连接方式
type DatabaseAuditor struct {
	db *gorm.DB
}

// NewDatabaseAuditor 创建数据库审计器
func NewDatabaseAuditor(db *gorm.DB) Auditor {
	return &DatabaseAuditor{db: db}
}

// AuditLoginStart 审计登录开始（连接尝试）
func (a *DatabaseAuditor) AuditLoginStart(ctx context.Context, session *SessionInfo) error {
	log.Printf("[UnifiedAudit] Login attempt: session=%s, user=%s, target=%s@%s, type=%s",
		session.SessionID, session.Username, session.HostUsername, session.HostIP, session.ConnectionType)

	// 创建登录记录（status: connecting）
	loginRecord := &model.LoginRecord{
		ID:        session.SessionID, // 直接使用session_id作为ID，不需要加后缀
		SessionID: session.SessionID,
		UserID:    session.UserID,
		HostID:    session.HostID,
		HostName:  session.HostName,
		HostIP:    session.HostIP,
		Username:  session.Username,
		LoginIP:   session.ClientIP,
		LoginTime: session.StartTime,
		Status:    "connecting",
	}

	if err := a.db.Create(loginRecord).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to create login record: %v", err)
		return fmt.Errorf("failed to create login record: %w", err)
	}

	log.Printf("[UnifiedAudit]  Login attempt recorded: %s (status: connecting)", session.SessionID)
	return nil
}

// AuditLoginSuccess 审计登录成功
func (a *DatabaseAuditor) AuditLoginSuccess(ctx context.Context, session *SessionInfo) error {
	log.Printf("[UnifiedAudit] Login success: session=%s, user=%s, target=%s@%s, type=%s",
		session.SessionID, session.Username, session.HostUsername, session.HostIP, session.ConnectionType)

	// 1. 更新登录记录状态为 active
	if err := a.db.Model(&model.LoginRecord{}).
		Where("session_id = ?", session.SessionID).
		Update("status", "active").Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update login record status: %v", err)
	}

	// 2. 创建会话录制记录（status: active）
	recording := &model.SessionRecording{
		ID:             uuid.New().String(),
		SessionID:      session.SessionID,
		ConnectionType: string(session.ConnectionType),
		ProxyID:        session.ProxyID,
		UserID:         session.UserID,
		Username:       session.Username,
		HostID:         session.HostID,
		HostName:       session.HostName,
		HostIP:         session.HostIP,
		StartTime:      session.StartTime,
		Status:         "active",
		Duration:       "进行中",
		TerminalCols:   session.TerminalCols,
		TerminalRows:   session.TerminalRows,
		CommandCount:   0,
	}

	if err := a.db.Create(recording).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to create session recording: %v", err)
		return fmt.Errorf("failed to create session recording: %w", err)
	}

	log.Printf("[UnifiedAudit]  Login success recorded: %s (type: %s)", session.SessionID, session.ConnectionType)
	return nil
}

// AuditLoginFailed 审计登录失败
func (a *DatabaseAuditor) AuditLoginFailed(ctx context.Context, sessionID string, endTime time.Time, reason string) error {
	log.Printf("[UnifiedAudit] Login failed: session=%s, reason=%s", sessionID, reason)

	// 更新登录记录状态为 failed
	updates := map[string]interface{}{
		"status":      "failed",
		"logout_time": endTime,
	}

	if err := a.db.Model(&model.LoginRecord{}).
		Where("session_id = ?", sessionID).
		Updates(updates).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update login record: %v", err)
		return fmt.Errorf("failed to update login record: %w", err)
	}

	log.Printf("[UnifiedAudit]  Login failure recorded: %s", sessionID)
	return nil
}

// AuditSessionEnd 审计会话结束
func (a *DatabaseAuditor) AuditSessionEnd(ctx context.Context, sessionID string, endTime time.Time, recording string) error {
	log.Printf("[UnifiedAudit] Session ending: %s", sessionID)

	// 1. 查询会话开始时间，计算持续时间
	var sessionRec model.SessionRecording
	if err := a.db.Where("session_id = ?", sessionID).First(&sessionRec).Error; err != nil {
		log.Printf("[UnifiedAudit] Session not found: %s", sessionID)
		return fmt.Errorf("session not found: %w", err)
	}

	// 计算持续时间
	diff := endTime.Sub(sessionRec.StartTime)
	minutes := int(diff.Minutes())
	seconds := int(diff.Seconds()) % 60
	duration := fmt.Sprintf("%dm %ds", minutes, seconds)
	durationSec := int(diff.Seconds())

	// 2. 更新会话录制记录
	sessionUpdates := map[string]interface{}{
		"end_time":  endTime,
		"status":    "closed",
		"duration":  duration,
		"recording": recording,
	}

	if err := a.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", sessionID).
		Updates(sessionUpdates).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update session recording: %v", err)
		return fmt.Errorf("failed to update session recording: %w", err)
	}

	// 3. 更新登录记录
	loginUpdates := map[string]interface{}{
		"logout_time": endTime,
		"status":      "completed",
		"duration":    durationSec,
	}

	if err := a.db.Model(&model.LoginRecord{}).
		Where("session_id = ?", sessionID).
		Updates(loginUpdates).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update login record: %v", err)
	}

	log.Printf("[UnifiedAudit]  Session ended: %s (duration: %s)", sessionID, duration)
	return nil
}

// AuditCommand 审计命令执行
func (a *DatabaseAuditor) AuditCommand(ctx context.Context, cmd *CommandInfo) error {
	log.Printf("[UnifiedAudit] Command: %s (session: %s)", cmd.Command, cmd.SessionID)

	// 创建命令记录
	commandRecord := &model.CommandRecord{
		ProxyID:    cmd.ProxyID,
		SessionID:  cmd.SessionID,
		HostID:     cmd.HostID,
		UserID:     cmd.UserID,
		Username:   cmd.Username,
		HostIP:     cmd.HostIP,
		Command:    cmd.Command,
		Output:     cmd.Output,
		ExitCode:   cmd.ExitCode,
		ExecutedAt: cmd.ExecutedAt,
		DurationMs: cmd.DurationMs,
	}

	if err := a.db.Create(commandRecord).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to create command record: %v", err)
		return fmt.Errorf("failed to create command record: %w", err)
	}

	// 更新会话的命令计数
	if err := a.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", cmd.SessionID).
		Update("command_count", gorm.Expr("command_count + 1")).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update command count: %v", err)
	}

	return nil
}

// AuditData 审计数据流（用于实时监控）
func (a *DatabaseAuditor) AuditData(ctx context.Context, sessionID string, direction string, data []byte) error {
	// 数据流审计通常是实时的，不需要每次都写入数据库
	// 这里可以实现更细粒度的审计逻辑，如：
	// 1. 检测敏感命令
	// 2. 监控异常行为
	// 3. 实时告警
	return nil
}
