package trovemodule

import (
	"context"

	"github.com/hashicorp/go-plugin"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// Core is the module's connection to the Trove parent process. Use it to append
// events, store blobs, and read the journal. The plugin runtime opens this
// connection on behalf of the module; authors do not dial brokers or handles.
type Core interface {
	Emitter
	BlobPutter
	Querier
}

// Module is the main entry contract for trovemodule.Serve. Implement Run to
// receive a Core handle when the parent starts the module.
type Module interface {
	Run(ctx context.Context, core Core) error
}

// RunFunc adapts a function to Module.
type RunFunc func(ctx context.Context, core Core) error

func (f RunFunc) Run(ctx context.Context, core Core) error {
	return f(ctx, core)
}

func connectCore(broker *plugin.GRPCBroker, handle uint32) (Core, error) {
	if handle == 0 {
		return nil, errCoreUnavailable
	}
	conn, err := broker.Dial(handle)
	if err != nil {
		return nil, err
	}
	// Connection lifetime is bound to Run; closed when Run returns.
	return &coreConn{
		client: troverpc.NewCoreServicesClient(conn),
		closer: conn,
	}, nil
}

type coreConn struct {
	client troverpc.CoreServicesClient
	closer interface{ Close() error }
}

func (c *coreConn) Close() error {
	return c.closer.Close()
}

func (c *coreConn) Emit(ctx context.Context, event *troverpc.Event) error {
	_, err := c.client.Emit(ctx, event)
	return err
}

func (c *coreConn) Put(ctx context.Context, data []byte) (string, error) {
	resp, err := c.client.BlobPut(ctx, &troverpc.BlobPutRequest{Data: data})
	if err != nil {
		return "", err
	}
	return resp.GetBlobRef(), nil
}

func (c *coreConn) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	return c.client.GetEvent(ctx, &troverpc.GetEventRequest{Id: id})
}

func (c *coreConn) SearchEvents(ctx context.Context, req *troverpc.SearchEventsRequest) ([]*troverpc.Event, error) {
	resp, err := c.client.SearchEvents(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (c *coreConn) GetEventsByType(ctx context.Context, req *troverpc.GetEventsByTypeRequest) ([]*troverpc.Event, error) {
	resp, err := c.client.GetEventsByType(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (c *coreConn) SummarizeRange(ctx context.Context, req *troverpc.SummarizeRangeRequest) (*troverpc.Summary, error) {
	return c.client.SummarizeRange(ctx, req)
}
