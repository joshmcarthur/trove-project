package modules

import (
	"context"
	"sync"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// MCPDispatcher handles MCP tool calls for a module subprocess.
type MCPDispatcher interface {
	CallTool(ctx context.Context, req *troverpc.MCPToolCallRequest) (*troverpc.MCPToolCallResponse, error)
}

// MCPRegistry tracks live MCP-capable module clients for tool dispatch.
type MCPRegistry struct {
	mu      sync.RWMutex
	clients map[string]MCPDispatcher
}

// NewMCPRegistry returns an empty MCP module registry.
func NewMCPRegistry() *MCPRegistry {
	return &MCPRegistry{clients: make(map[string]MCPDispatcher)}
}

// Register adds a module MCP client.
func (r *MCPRegistry) Register(name string, client MCPDispatcher) {
	if r == nil || client == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = client
}

// Unregister removes a module MCP client.
func (r *MCPRegistry) Unregister(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, name)
}

// Get returns the MCP client for module name.
func (r *MCPRegistry) Get(name string) (MCPDispatcher, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[name]
	return client, ok
}
