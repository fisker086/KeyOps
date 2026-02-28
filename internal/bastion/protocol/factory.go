package protocol

import (
	"fmt"
	"sync"
)

// Factory 协议工厂
type Factory struct {
	handlers map[ProtocolType]HandlerCreator
	mu       sync.RWMutex
}

// HandlerCreator 处理器创建函数
type HandlerCreator func(recorder SessionRecorder) ProtocolHandler

var (
	defaultFactory *Factory
	once           sync.Once
)

// GetFactory 获取默认工厂实例
func GetFactory() *Factory {
	once.Do(func() {
		defaultFactory = NewFactory()
	})
	return defaultFactory
}

// NewFactory 创建新的工厂
func NewFactory() *Factory {
	return &Factory{
		handlers: make(map[ProtocolType]HandlerCreator),
	}
}

// Register 注册协议处理器
func (f *Factory) Register(protocol ProtocolType, creator HandlerCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[protocol] = creator
}

// Create 创建协议处理器
func (f *Factory) Create(protocol ProtocolType, recorder SessionRecorder) (ProtocolHandler, error) {
	f.mu.RLock()
	creator, ok := f.handlers[protocol]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return creator(recorder), nil
}

// SupportedProtocols 获取支持的协议列表
func (f *Factory) SupportedProtocols() []ProtocolType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	protocols := make([]ProtocolType, 0, len(f.handlers))
	for protocol := range f.handlers {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// IsSupported 检查协议是否支持
func (f *Factory) IsSupported(protocol ProtocolType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.handlers[protocol]
	return ok
}
