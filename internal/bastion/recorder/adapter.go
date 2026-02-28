package recorder

import (
	"time"

	"github.com/fisker086/keyops/internal/bastion/protocol"
)

// RecorderAdapter 适配器，让 Recorder 实现 protocol.SessionRecorder 接口
type RecorderAdapter struct {
	recorder *Recorder
}

// NewRecorderAdapter 创建录制器适配器
func NewRecorderAdapter(recorder *Recorder) protocol.SessionRecorder {
	return &RecorderAdapter{
		recorder: recorder,
	}
}

// RecordStart 记录会话开始
func (a *RecorderAdapter) RecordStart(info *protocol.SessionInfo) error {
	// Recorder 在创建时已经开始记录
	return nil
}

// RecordEnd 记录会话结束
func (a *RecorderAdapter) RecordEnd(sessionID string, endTime time.Time) error {
	// 关闭录制器
	a.recorder.Close()
	return nil
}

// RecordData 记录数据流
func (a *RecorderAdapter) RecordData(sessionID string, direction string, data []byte) error {
	if direction == "out" {
		a.recorder.RecordOutput(string(data))
	} else if direction == "in" {
		a.recorder.RecordInput(string(data))
	}
	return nil
}

// RecordCommand 记录命令
func (a *RecorderAdapter) RecordCommand(sessionID, command, output string, exitCode int) error {
	// 这里可以记录命令到日志或其他地方
	// Recorder 主要关注终端输出，命令记录可以通过其他方式
	return nil
}

// RecordError 记录错误
func (a *RecorderAdapter) RecordError(sessionID, errorMsg string) error {
	// 记录错误信息作为输出
	a.recorder.RecordOutput("\r\nError: " + errorMsg + "\r\n")
	return nil
}
