package recorder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventTypeOutput EventType = "o" // 输出
	EventTypeInput  EventType = "i" // 输入
)

// Event 录制事件（asciinema 格式）
type Event struct {
	Time float64   `json:"time"` // 相对开始时间的秒数
	Type EventType `json:"type"` // 事件类型: "o" 或 "i"
	Data string    `json:"data"` // 数据内容
}

// Header asciinema 文件头
type Header struct {
	Version   int      `json:"version"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Timestamp int64    `json:"timestamp"`
	Env       *EnvInfo `json:"env,omitempty"`
}

type EnvInfo struct {
	Shell string `json:"SHELL,omitempty"`
	Term  string `json:"TERM,omitempty"`
}

// Recorder 会话录制器
type Recorder struct {
	sessionID string
	startTime time.Time
	events    []Event
	eventChan chan Event
	mu        sync.RWMutex
	closed    bool
	width     int
	height    int
}

// NewRecorder 创建新的录制器
func NewRecorder(sessionID string, width, height int) *Recorder {
	r := &Recorder{
		sessionID: sessionID,
		startTime: time.Now(),
		events:    make([]Event, 0),
		eventChan: make(chan Event, 1000), // 缓冲 1000 个事件
		width:     width,
		height:    height,
	}

	// 启动异步处理 goroutine
	go r.processEvents()

	return r
}

// RecordOutput 记录输出（异步）
func (r *Recorder) RecordOutput(data string) {
	if r.closed {
		return
	}

	select {
	case r.eventChan <- Event{
		Time: time.Since(r.startTime).Seconds(),
		Type: EventTypeOutput,
		Data: data,
	}:
	default:
		log.Printf("Recorder buffer full, dropping output event")
	}
}

// RecordInput 记录输入（异步）
func (r *Recorder) RecordInput(data string) {
	if r.closed {
		return
	}

	select {
	case r.eventChan <- Event{
		Time: time.Since(r.startTime).Seconds(),
		Type: EventTypeInput,
		Data: data,
	}:
	default:
		log.Printf("Recorder buffer full, dropping input event")
	}
}

// processEvents 异步处理事件
func (r *Recorder) processEvents() {
	for event := range r.eventChan {
		r.mu.Lock()
		r.events = append(r.events, event)
		r.mu.Unlock()
	}
}

// Close 关闭录制器
func (r *Recorder) Close() {
	if r.closed {
		return
	}

	r.closed = true
	close(r.eventChan)
}

// ToAsciinema 导出为 asciinema 格式（JSON Lines）
func (r *Recorder) ToAsciinema() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var buf bytes.Buffer

	// 写入 header
	header := Header{
		Version:   2,
		Width:     r.width,
		Height:    r.height,
		Timestamp: r.startTime.Unix(),
		Env: &EnvInfo{
			Term: "xterm-256color",
		},
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %w", err)
	}
	buf.Write(headerJSON)
	buf.WriteString("\n")

	// 写入所有事件（使用 json.Marshal 确保正确转义）
	for _, event := range r.events {
		// 构造事件数组 [time, type, data]
		eventArray := []interface{}{event.Time, string(event.Type), event.Data}
		eventJSON, err := json.Marshal(eventArray)
		if err != nil {
			log.Printf("Failed to marshal event: %v", err)
			continue
		}
		buf.Write(eventJSON)
		buf.WriteString("\n")
	}

	return buf.String(), nil
}

// GetEventCount 获取事件数量
func (r *Recorder) GetEventCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
