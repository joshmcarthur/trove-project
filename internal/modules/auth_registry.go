package modules

import (
	"context"
	"sync"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// AuthValidator validates gateway requests for a declared auth validator ref.
type AuthValidator interface {
	ValidateAuth(ctx context.Context, req *troverpc.AuthRequest) (*troverpc.AuthResponse, error)
}

// AuthRegistry tracks live auth validator clients for gateway dispatch.
type AuthRegistry struct {
	mu         sync.RWMutex
	validators map[string]AuthValidator
}

// NewAuthRegistry returns an empty auth validator registry.
func NewAuthRegistry() *AuthRegistry {
	return &AuthRegistry{validators: make(map[string]AuthValidator)}
}

// Register adds validators declared by a module subprocess.
func (r *AuthRegistry) Register(moduleName string, client AuthValidator, validators []AuthValidatorDecl) {
	if r == nil || client == nil || moduleName == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, validator := range validators {
		if validator.ID == "" {
			continue
		}
		r.validators[AuthValidatorRef(moduleName, validator.ID)] = client
	}
}

// Unregister removes all validators owned by moduleName.
func (r *AuthRegistry) Unregister(moduleName string, validators []AuthValidatorDecl) {
	if r == nil || moduleName == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, validator := range validators {
		delete(r.validators, AuthValidatorRef(moduleName, validator.ID))
	}
}

// Get returns the validator client for a full ref (module.<name>.<id>).
func (r *AuthRegistry) Get(ref string) (AuthValidator, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.validators[ref]
	return client, ok
}
