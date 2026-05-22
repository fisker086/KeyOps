package audit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DatabaseAuditor 统一的数据库审计器（SQL + 可选堡垒机 Mongo）
type DatabaseAuditor struct {
	db      *gorm.DB
	session *repository.SessionRepository
}

// NewDatabaseAuditor 创建数据库审计器；session 可为 nil（仅 SQL）。
func NewDatabaseAuditor(db *gorm.DB, session *repository.SessionRepository) Auditor {
	return &DatabaseAuditor{db: db, session: session}
}

func (a *DatabaseAuditor) bastionMongo() bool {
	return a.session != nil && a.session.UsesMongo()
}

// AuditLoginStart 审计登录开始（连接尝试）
func (a *DatabaseAuditor) AuditLoginStart(ctx context.Context, session *SessionInfo) error {
	log.Printf("[UnifiedAudit] Login attempt: session=%s, user=%s, target=%s@%s, type=%s",
		session.SessionID, session.Username, session.HostUsername, session.HostIP, session.ConnectionType)

	loginRecord := &model.LoginRecord{
		ID:        session.SessionID,
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

	if a.bastionMongo() {
		if err := a.session.CreateLoginRecord(loginRecord); err != nil {
			log.Printf("[UnifiedAudit] Failed to create login record (mongo): %v", err)
			return err
		}
		log.Printf("[UnifiedAudit]  Login attempt recorded: %s (status: connecting, mongo)", session.SessionID)
		return nil
	}

	if err := a.db.Create(loginRecord).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to create login record: %v", err)
		return err
	}
	log.Printf("[UnifiedAudit]  Login attempt recorded: %s (status: connecting)", session.SessionID)
	return nil
}

// AuditLoginSuccess 审计登录成功
func (a *DatabaseAuditor) AuditLoginSuccess(ctx context.Context, session *SessionInfo) error {
	log.Printf("[UnifiedAudit] Login success: session=%s, user=%s, target=%s@%s, type=%s",
		session.SessionID, session.Username, session.HostUsername, session.HostIP, session.ConnectionType)

	if a.bastionMongo() {
		if err := a.session.UpdateLoginStatusBySessionID(session.SessionID, "active"); err != nil {
			log.Printf("[UnifiedAudit] Failed to update login record status (mongo): %v", err)
		}
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
		if err := a.session.CreateSessionRecording(recording); err != nil {
			log.Printf("[UnifiedAudit] Failed to create session recording (mongo): %v", err)
			return err
		}
		log.Printf("[UnifiedAudit]  Login success recorded: %s (type: %s, mongo)", session.SessionID, session.ConnectionType)
		return nil
	}

	if err := a.db.Model(&model.LoginRecord{}).
		Where("session_id = ?", session.SessionID).
		Update("status", "active").Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update login record status: %v", err)
	}

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
		return err
	}

	log.Printf("[UnifiedAudit]  Login success recorded: %s (type: %s)", session.SessionID, session.ConnectionType)
	return nil
}

// AuditLoginFailed 审计登录失败
func (a *DatabaseAuditor) AuditLoginFailed(ctx context.Context, sessionID string, endTime time.Time, reason string) error {
	log.Printf("[UnifiedAudit] Login failed: session=%s, reason=%s", sessionID, reason)

	updates := map[string]interface{}{
		"status":      "failed",
		"logout_time": endTime,
	}

	if a.bastionMongo() {
		if err := a.session.UpdateLoginBySessionID(sessionID, updates); err != nil {
			log.Printf("[UnifiedAudit] Failed to update login record (mongo): %v", err)
			return err
		}
		log.Printf("[UnifiedAudit]  Login failure recorded: %s (mongo)", sessionID)
		return nil
	}

	if err := a.db.Model(&model.LoginRecord{}).
		Where("session_id = ?", sessionID).
		Updates(updates).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update login record: %v", err)
		return err
	}

	log.Printf("[UnifiedAudit]  Login failure recorded: %s", sessionID)
	return nil
}

// AuditSessionEnd 审计会话结束
func (a *DatabaseAuditor) AuditSessionEnd(ctx context.Context, sessionID string, endTime time.Time, recording string) error {
	log.Printf("[UnifiedAudit] Session ending: %s", sessionID)

	if a.bastionMongo() {
		sessionRec, err := a.session.FindSessionRecordingBySessionID(sessionID)
		if err != nil {
			log.Printf("[UnifiedAudit] Session not found: %s, %v", sessionID, err)
			return err
		}
		diff := endTime.Sub(sessionRec.StartTime)
		minutes := int(diff.Minutes())
		seconds := int(diff.Seconds()) % 60
		duration := fmt.Sprintf("%dm %ds", minutes, seconds)
		durationSec := int(diff.Seconds())

		sessionUpdates := map[string]interface{}{
			"end_time":  endTime,
			"status":    "closed",
			"duration":  duration,
			"recording": recording,
		}
		if err := a.session.UpdateSessionRecordingFields(sessionID, sessionUpdates); err != nil {
			log.Printf("[UnifiedAudit] Failed to update session recording (mongo): %v", err)
			return err
		}
		loginUpdates := map[string]interface{}{
			"logout_time": endTime,
			"status":      "completed",
			"duration":    durationSec,
		}
		if err := a.session.UpdateLoginBySessionID(sessionID, loginUpdates); err != nil {
			log.Printf("[UnifiedAudit] Failed to update login record (mongo): %v", err)
		}
		log.Printf("[UnifiedAudit]  Session ended: %s (duration: %s, mongo)", sessionID, duration)
		return nil
	}

	var sessionRec model.SessionRecording
	if err := a.db.Where("session_id = ?", sessionID).First(&sessionRec).Error; err != nil {
		log.Printf("[UnifiedAudit] Session not found: %s", sessionID)
		return err
	}

	diff := endTime.Sub(sessionRec.StartTime)
	minutes := int(diff.Minutes())
	seconds := int(diff.Seconds()) % 60
	duration := fmt.Sprintf("%dm %ds", minutes, seconds)
	durationSec := int(diff.Seconds())

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
		return err
	}

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

	if a.bastionMongo() {
		commandRecord.CreatedAt = time.Now()
		if err := a.session.CreateCommandRecord(commandRecord); err != nil {
			log.Printf("[UnifiedAudit] Failed to create command record (mongo): %v", err)
			return err
		}
		if err := a.session.IncrementSessionCommandCount(cmd.SessionID); err != nil {
			log.Printf("[UnifiedAudit] Failed to update command count (mongo): %v", err)
		}
		return nil
	}

	if err := a.db.Create(commandRecord).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to create command record: %v", err)
		return err
	}

	if err := a.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", cmd.SessionID).
		Update("command_count", gorm.Expr("command_count + 1")).Error; err != nil {
		log.Printf("[UnifiedAudit] Failed to update command count: %v", err)
	}

	return nil
}

// AuditData 审计数据流（用于实时监控）
func (a *DatabaseAuditor) AuditData(ctx context.Context, sessionID string, direction string, data []byte) error {
	return nil
}
