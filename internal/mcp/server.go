package mcp

import (
	"encoding/json"
	"fmt"
)

type Server struct {
	registry *Registry
}

func NewServer() *Server {
	return &Server{registry: NewRegistry()}
}

func (s *Server) Registry() *Registry {
	return s.registry
}

func (s *Server) Handle(raw json.RawMessage, allowedTools []string) json.RawMessage {
	var req JSONRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.error(nil, -32700, "Parse error", err.Error())
	}

	if req.JSONRPC != "2.0" {
		return s.error(req.ID, -32600, "Invalid Request", "jsonrpc must be 2.0")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID)
	case "tools/list":
		return s.handleListTools(req.ID, allowedTools)
	case "tools/call":
		return s.handleCallTool(req.ID, req.Params, allowedTools)
	default:
		return s.error(req.ID, -32601, "Method not found", fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (s *Server) handleInitialize(id json.RawMessage) json.RawMessage {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &Capability{},
		},
		ServerInfo: Implementation{
			Name:    "k8s-mcp",
			Version: "0.1.0",
		},
	}
	return s.result(id, result)
}

func (s *Server) handleListTools(id json.RawMessage, allowedTools []string) json.RawMessage {
	tools := s.registry.List()
	if len(allowedTools) > 0 {
		allowed := make(map[string]bool, len(allowedTools))
		for _, t := range allowedTools {
			allowed[t] = true
		}
		filtered := make([]ToolDefinition, 0, len(tools))
		for _, t := range tools {
			if allowed[t.Name] {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}
	result := ListToolsResult{Tools: tools}
	return s.result(id, result)
}

func (s *Server) handleCallTool(id json.RawMessage, params json.RawMessage, allowedTools []string) json.RawMessage {
	var call struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return s.error(id, -32602, "Invalid params", err.Error())
	}

	if !s.isToolAllowed(call.Name, allowedTools) {
		return s.error(id, -32000, "Forbidden", fmt.Sprintf("tool %q is not allowed by this API key", call.Name))
	}

	tool, ok := s.registry.Get(call.Name)
	if !ok {
		return s.error(id, -32602, "Tool not found", fmt.Sprintf("unknown tool: %s", call.Name))
	}

	result := tool.Handler(call.Args)
	return s.result(id, result)
}

func (s *Server) isToolAllowed(name string, allowedTools []string) bool {
	if len(allowedTools) == 0 {
		return true
	}
	for _, t := range allowedTools {
		if t == name {
			return true
		}
	}
	return false
}

func (s *Server) result(id json.RawMessage, v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return s.error(id, -32603, "Internal error", err.Error())
	}
	resp, _ := json.Marshal(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	})
	return resp
}

func (s *Server) error(id json.RawMessage, code int, message string, data string) json.RawMessage {
	var dataRaw json.RawMessage
	if data != "" {
		dataRaw, _ = json.Marshal(data)
	}
	resp, _ := json.Marshal(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    dataRaw,
		},
	})
	return resp
}
