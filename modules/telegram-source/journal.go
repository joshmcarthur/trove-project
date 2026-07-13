package main

import (
	"context"

	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

type captureStore struct {
	core trovemodule.Core
}

func (a *captureStore) GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	return a.core.GetRecord(ctx, req)
}

func (a *captureStore) ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	return a.core.ListIncompleteRecords(ctx, req)
}

func (a *captureStore) RecordWrite(ctx context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error) {
	return a.core.RecordWrite(ctx, req)
}

func (a *captureStore) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	event, err := a.core.GetEvent(ctx, id)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, classify.ErrNotFound
		}
		return nil, err
	}
	return event, nil
}
