package modules

import (
	"bytes"
	"context"

	"github.com/joshmcarthur/trove/internal/blob"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type coreServicesServer struct {
	troverpc.UnimplementedCoreServicesServer
	blobs blob.Store
}

func (s *coreServicesServer) BlobPut(ctx context.Context, req *troverpc.BlobPutRequest) (*troverpc.BlobPutResponse, error) {
	if req == nil || len(req.Data) == 0 {
		return nil, status.Error(codes.InvalidArgument, "blob data is required")
	}
	ref, err := s.blobs.Put(ctx, bytes.NewReader(req.Data))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &troverpc.BlobPutResponse{BlobRef: ref}, nil
}
