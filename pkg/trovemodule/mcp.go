package trovemodule

import (
	"context"
	"encoding/json"
)

// MCPToolHandler handles MCP tool invocations for a module.
type MCPToolHandler interface {
	CallTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error)
}

// MCPToolCaller forwards MCP tool calls to other modules via Core.
type MCPToolCaller interface {
	ListMCPTools(ctx context.Context) ([]MCPToolDescriptor, error)
	CallMCPTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error)
}

// MCPToolDescriptor describes a module-provided MCP tool.
type MCPToolDescriptor struct {
	Name        string
	Description string
	Module      string
}
