package modules

import (
	"context"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ingestServer struct {
	troverpc.UnimplementedSourceServer
	journal journal.Journal
	policy  IngestPolicy
}

func (s *ingestServer) Emit(ctx context.Context, e *troverpc.Event) (*troverpc.EmitResponse, error) {
	event, err := rpcEventToJournal(e)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.policy.ValidateEvent(event); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.journal.Append(ctx, event); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &troverpc.EmitResponse{}, nil
}
