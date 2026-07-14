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
	core    trovemodule.Core
	records trovemodule.RecordProjectionReader
}

func newCaptureStore(core trovemodule.Core) (*captureStore, error) {
	records, err := trovemodule.RecordProjection(core)
	if err != nil {
		return nil, err
	}
	return &captureStore{core: core, records: records}, nil
}

func (a *captureStore) GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error) {
	return a.records.GetRecord(ctx, req)
}

func (a *captureStore) ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error) {
	return a.records.ListIncompleteRecords(ctx, req)
}

func (a *captureStore) AppendRevision(ctx context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error) {
	return a.core.AppendRevision(ctx, req)
}

func (a *captureStore) GetRevision(ctx context.Context, id string) (*troverpc.Revision, error) {
	revision, err := a.core.GetRevision(ctx, id)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, classify.ErrNotFound
		}
		return nil, err
	}
	return revision, nil
}
