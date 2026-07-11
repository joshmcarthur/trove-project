package trovemodule

import "context"

// BlobPutter stores blobs via Core.BlobPut.
type BlobPutter interface {
	Put(ctx context.Context, data []byte) (ref string, err error)
}

// BlobRunner is a legacy entry point for modules that emit events and store
// blobs. Prefer implementing Module with Run(ctx, core Core).
type BlobRunner interface {
	RunWithBlobs(ctx context.Context, emit Emitter, blobs BlobPutter) error
}
