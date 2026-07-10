package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// HTTPHandler handles inbound HTTP requests dispatched from the gateway.
type HTTPHandler interface {
	HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error)
}
