package modules

import (
	"context"
	"sync"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// HTTPDispatcher handles HTTP requests for a module subprocess.
type HTTPDispatcher interface {
	HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error)
}

// HTTPRegistry tracks live HTTP-capable module clients for gateway dispatch.
type HTTPRegistry struct {
	mu      sync.RWMutex
	clients map[string]HTTPDispatcher
}

// NewHTTPRegistry returns an empty HTTP module registry.
func NewHTTPRegistry() *HTTPRegistry {
	return &HTTPRegistry{clients: make(map[string]HTTPDispatcher)}
}

// Register adds a module HTTP client.
func (r *HTTPRegistry) Register(name string, client HTTPDispatcher) {
	if r == nil || client == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = client
}

// Unregister removes a module HTTP client.
func (r *HTTPRegistry) Unregister(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, name)
}

// Get returns the HTTP client for module name.
func (r *HTTPRegistry) Get(name string) (HTTPDispatcher, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[name]
	return client, ok
}
