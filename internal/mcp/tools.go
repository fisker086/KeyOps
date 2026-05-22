package mcp

import (
	"encoding/json"
)

type ToolHandler func(args json.RawMessage) *CallToolResult

type ToolEntry struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

type Registry struct {
	tools map[string]ToolEntry
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]ToolEntry)}
}

func (r *Registry) Register(tool ToolDefinition, handler ToolHandler) {
	r.tools[tool.Name] = ToolEntry{Definition: tool, Handler: handler}
}

func (r *Registry) Get(name string) (ToolEntry, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition)
	}
	return defs
}
