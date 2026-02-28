package audit

import (
	"context"
	"time"

	"github.com/fisker086/keyops/internal/bastion/storage"
	ssht "github.com/fisker086/keyops/internal/sshserver/types"
)

// =============================================================================
// SSH Gateway Adapter - 适配 sshserver/types.Auditor 接口
// =============================================================================

// SSHGatewayAuditorAdapter 将统一审计器适配为SSH Gateway的Auditor接口
type SSHGatewayAuditorAdapter struct {
	auditor Auditor
}

// NewSSHGatewayAuditorAdapter 创建SSH Gateway审计器适配器
func NewSSHGatewayAuditorAdapter(auditor Auditor) ssht.Auditor {
	return &SSHGatewayAuditorAdapter{auditor: auditor}
}

// AuditLoginStart 审计登录开始
func (a *SSHGatewayAuditorAdapter) AuditLoginStart(ctx context.Context, session *ssht.SessionInfo) error {
	unified := &SessionInfo{
		SessionID:      session.SessionID,
		ConnectionType: ConnectionTypeSSHGateway,
		ProxyID:        "ssh-gateway",
		UserID:         session.UserID,
		Username:       session.Username,
		ClientIP:       session.ClientIP,
		HostID:         session.HostID,
		HostName:       session.HostName, // 使用session中的HostName
		HostIP:         session.HostIP,
		HostPort:       session.HostPort,
		HostUsername:   session.HostUsername,
		StartTime:      session.StartTime,
		Status:         session.Status,
		TerminalCols:   session.TerminalCols,
		TerminalRows:   session.TerminalRows,
	}
	return a.auditor.AuditLoginStart(ctx, unified)
}

// AuditConnectionSuccess 审计连接成功
func (a *SSHGatewayAuditorAdapter) AuditConnectionSuccess(ctx context.Context, session *ssht.SessionInfo) error {
	unified := &SessionInfo{
		SessionID:      session.SessionID,
		ConnectionType: ConnectionTypeSSHGateway,
		ProxyID:        "ssh-gateway",
		UserID:         session.UserID,
		Username:       session.Username,
		ClientIP:       session.ClientIP,
		HostID:         session.HostID,
		HostName:       session.HostName, // 添加主机名称
		HostIP:         session.HostIP,
		HostPort:       session.HostPort,
		HostUsername:   session.HostUsername,
		StartTime:      session.StartTime,
		Status:         "active",
		TerminalCols:   session.TerminalCols,
		TerminalRows:   session.TerminalRows,
	}
	return a.auditor.AuditLoginSuccess(ctx, unified)
}

// AuditSessionStart 审计会话开始（向后兼容）
func (a *SSHGatewayAuditorAdapter) AuditSessionStart(ctx context.Context, session *ssht.SessionInfo) error {
	// 先创建登录记录
	if err := a.AuditLoginStart(ctx, session); err != nil {
		return err
	}
	// 立即标记为成功（兼容旧逻辑）
	return a.AuditConnectionSuccess(ctx, session)
}

// AuditSessionEnd 审计会话结束
func (a *SSHGatewayAuditorAdapter) AuditSessionEnd(ctx context.Context, sessionID string, endTime time.Time) error {
	// 录制内容在adapter中无法获取，传空字符串
	// 实际录制内容由Recorder负责
	return a.auditor.AuditSessionEnd(ctx, sessionID, endTime, "")
}

// AuditSessionFailed 审计会话失败
func (a *SSHGatewayAuditorAdapter) AuditSessionFailed(ctx context.Context, sessionID string, endTime time.Time, reason string) error {
	return a.auditor.AuditLoginFailed(ctx, sessionID, endTime, reason)
}

// AuditCommand 审计命令
func (a *SSHGatewayAuditorAdapter) AuditCommand(ctx context.Context, cmd *ssht.CommandInfo) error {
	unified := &CommandInfo{
		SessionID:  cmd.SessionID,
		ProxyID:    "ssh-gateway",
		HostID:     cmd.HostID,
		HostIP:     cmd.HostIP,
		UserID:     cmd.UserID,
		Username:   cmd.Username,
		Command:    cmd.Command,
		Output:     cmd.Output,
		ExitCode:   cmd.ExitCode,
		ExecutedAt: cmd.ExecutedAt,
		DurationMs: cmd.DurationMs,
	}
	return a.auditor.AuditCommand(ctx, unified)
}

// AuditData 审计数据流
func (a *SSHGatewayAuditorAdapter) AuditData(ctx context.Context, sessionID string, direction string, data []byte) error {
	return a.auditor.AuditData(ctx, sessionID, direction, data)
}

// =============================================================================
// WebShell Storage Adapter - 适配 bastion/storage.Storage 接口
// =============================================================================

// WebShellStorageAdapter 将统一审计器适配为WebShell的Storage接口
type WebShellStorageAdapter struct {
	auditor Auditor
}

// NewWebShellStorageAdapter 创建WebShell存储适配器
func NewWebShellStorageAdapter(auditor Auditor) storage.Storage {
	return &WebShellStorageAdapter{auditor: auditor}
}

// SaveSession 保存会话记录
func (a *WebShellStorageAdapter) SaveSession(session *storage.SessionRecord) error {
	unified := &SessionInfo{
		SessionID:      session.SessionID,
		ConnectionType: ConnectionTypeAPIServer,
		ProxyID:        session.ProxyID,
		UserID:         session.UserID,
		Username:       session.Username,
		HostID:         session.HostID,
		HostName:       session.HostName,
		HostIP:         session.HostIP,
		StartTime:      session.StartTime,
		Status:         session.Status,
		TerminalCols:   session.TerminalCols,
		TerminalRows:   session.TerminalRows,
	}
	return a.auditor.AuditLoginSuccess(context.Background(), unified)
}

// CloseSession 关闭会话
func (a *WebShellStorageAdapter) CloseSession(sessionID string, recording string) error {
	return a.auditor.AuditSessionEnd(context.Background(), sessionID, time.Now(), recording)
}

// MarkSessionFailed 标记会话失败
func (a *WebShellStorageAdapter) MarkSessionFailed(sessionID string, reason string) error {
	return a.auditor.AuditLoginFailed(context.Background(), sessionID, time.Now(), reason)
}

// SaveLoginRecord 保存登录记录
func (a *WebShellStorageAdapter) SaveLoginRecord(record *storage.LoginRecord) error {
	unified := &SessionInfo{
		SessionID:      record.SessionID,
		ConnectionType: ConnectionTypeAPIServer,
		ProxyID:        "api-server-direct",
		UserID:         record.UserID,
		Username:       record.Username,
		HostID:         record.HostID,
		HostName:       record.HostName,
		HostIP:         record.HostIP,
		ClientIP:       "",
		StartTime:      record.LoginTime,
		Status:         record.Status,
	}
	return a.auditor.AuditLoginStart(context.Background(), unified)
}

// UpdateLoginRecordStatus 更新登录记录状态
func (a *WebShellStorageAdapter) UpdateLoginRecordStatus(sessionID string, status string, logoutTime time.Time) error {
	if status == "failed" {
		return a.auditor.AuditLoginFailed(context.Background(), sessionID, logoutTime, "Connection failed")
	}
	// 对于 active 和 completed 状态，不需要特别处理
	// 因为在SaveSession和CloseSession中已经处理了
	return nil
}

// SaveCommand 保存命令记录
func (a *WebShellStorageAdapter) SaveCommand(cmd *storage.CommandRecord) error {
	unified := &CommandInfo{
		SessionID:  cmd.SessionID,
		ProxyID:    cmd.ProxyID,
		HostID:     cmd.HostID,
		HostIP:     cmd.HostIP,
		UserID:     cmd.UserID,
		Username:   cmd.Username,
		Command:    cmd.Command,
		Output:     cmd.Output,
		ExitCode:   cmd.ExitCode,
		ExecutedAt: cmd.ExecutedAt,
		DurationMs: cmd.DurationMs,
	}
	return a.auditor.AuditCommand(context.Background(), unified)
}

// Close 关闭适配器
func (a *WebShellStorageAdapter) Close() error {
	// 适配器本身不需要关闭
	return nil
}
