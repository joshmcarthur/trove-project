package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// AuthHandler validates inbound HTTP requests before gateway dispatch.
type AuthHandler interface {
	ValidateAuth(ctx context.Context, req *troverpc.AuthRequest) (*troverpc.AuthResponse, error)
}
