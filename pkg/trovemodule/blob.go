package trovemodule

import "context"

// BlobPutter stores blobs via Core.BlobPut.
type BlobPutter interface {
	Put(ctx context.Context, data []byte) (ref string, err error)
}
