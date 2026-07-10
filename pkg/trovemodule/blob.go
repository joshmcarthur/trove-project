package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// BlobPutter stores blobs via the Trove core CoreServices RPC.
type BlobPutter interface {
	Put(ctx context.Context, data []byte) (ref string, err error)
}

type blobPutter struct {
	client troverpc.CoreServicesClient
}

func (b *blobPutter) Put(ctx context.Context, data []byte) (string, error) {
	resp, err := b.client.BlobPut(ctx, &troverpc.BlobPutRequest{Data: data})
	if err != nil {
		return "", err
	}
	return resp.GetBlobRef(), nil
}
