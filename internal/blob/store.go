package blob

import (
	"context"
	"io"
)

// Store is content-addressed blob storage.
type Store interface {
	Put(ctx context.Context, data io.Reader) (ref string, err error)
	Get(ctx context.Context, ref string) (io.ReadCloser, error)
	Range(ctx context.Context, ref string, start, end int64) (io.ReadCloser, error)
	Enumerate(ctx context.Context) (<-chan string, error)
}
