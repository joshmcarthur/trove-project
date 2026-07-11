package modules

import (
	"context"
	"sync"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// GatewayModuleClient handles HTTP routes and/or auth validation for a module subprocess.
type GatewayModuleClient interface {
	HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error)
	ValidateAuth(ctx context.Context, req *troverpc.AuthRequest) (*troverpc.AuthResponse, error)
}

// HTTPRegistry tracks live gateway-capable module clients keyed by module name.
type HTTPRegistry struct {
	mu      sync.RWMutex
	clients map[string]GatewayModuleClient
}

// NewHTTPRegistry returns an empty gateway module registry.
func NewHTTPRegistry() *HTTPRegistry {
	return &HTTPRegistry{clients: make(map[string]GatewayModuleClient)}
}

// Register adds a module gateway client.
func (r *HTTPRegistry) Register(name string, client GatewayModuleClient) {
	if r == nil || client == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = client
}

// Unregister removes a module gateway client.
func (r *HTTPRegistry) Unregister(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, name)
}

// Get returns the gateway client for module name.
func (r *HTTPRegistry) Get(name string) (GatewayModuleClient, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[name]
	return client, ok
}
