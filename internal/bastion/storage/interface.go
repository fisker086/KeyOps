package storage

import "time"

// Storage interface for session and command storage
// Supports real-time database writes and API reporting
type Storage interface {
	// Session operations
	SaveSession(session *SessionRecord) error
	CloseSession(sessionID string, recording string) error
	MarkSessionFailed(sessionID string, reason string) error

	// Login record operations
	SaveLoginRecord(record *LoginRecord) error
	UpdateLoginRecordStatus(sessionID string, status string, logoutTime time.Time) error

	// Command operations
	SaveCommand(cmd *CommandRecord) error

	// Cleanup
	Close() error
}
