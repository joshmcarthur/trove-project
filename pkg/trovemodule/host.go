package trovemodule

import (
	"context"
	"fmt"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RecordProjectionReader reads folded record projections via host CoreServices RPCs.
// This is not part of the public module SDK; first-party modules (mcp-query, classify)
// use it for record-shaped host APIs.
type RecordProjectionReader interface {
	GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error)
	SearchRecords(ctx context.Context, req *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error)
	ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error)
}

// RecordProjection returns host-only record read RPCs from a Core connection.
func RecordProjection(c Core) (RecordProjectionReader, error) {
	conn, ok := c.(*coreConn)
	if !ok {
		return nil, fmt.Errorf("trovemodule: record projection requires plugin Core connection")
	}
	return conn, nil
}

func (c *coreConn) GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	return c.client.GetRecord(ctx, req)
}

func (c *coreConn) SearchRecords(ctx context.Context, req *troverpc.SearchRecordsRequest) ([]*troverpc.Record, error) {
	resp, err := c.client.SearchRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRecords(), nil
}

func (c *coreConn) ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	resp, err := c.client.ListIncompleteRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetRecords(), nil
}
