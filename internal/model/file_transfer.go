package model

import (
	"time"
)

// FileTransfer 文件传输记录
type FileTransfer struct {
	ID            string     `json:"id" gorm:"primaryKey"`
	SessionID     string     `json:"session_id" gorm:"index;not null"`
	UserID        string     `json:"user_id" gorm:"index;not null"`
	Username      string     `json:"username"`
	HostID        string     `json:"host_id" gorm:"index;not null"`
	HostIP        string     `json:"host_ip"`
	HostName      string     `json:"host_name"`
	Direction     string     `json:"direction"` // upload, download
	LocalPath     string     `json:"local_path"`
	RemotePath    string     `json:"remote_path"`
	FileName      string     `json:"file_name" gorm:"index"`
	FileSize      int64      `json:"file_size"`
	Status        string     `json:"status"` // uploading, completed, failed
	Progress      int        `json:"progress"`
	ErrorMessage  string     `json:"error_message"`
	TransferredAt time.Time  `json:"transferred_at" gorm:"autoCreateTime"`
	CompletedAt   *time.Time `json:"completed_at"`
	Duration      int        `json:"duration"` // 传输耗时（秒）
}

func (FileTransfer) TableName() string {
	return "file_transfers"
}
