package journal

import "context"

// Journal is the append-only revision store interface.
type Journal interface {
	Append(ctx context.Context, r Revision) error
	Query(ctx context.Context, f Filter) ([]Revision, error)
	Get(ctx context.Context, id string) (Revision, error)
	Watch(ctx context.Context) (<-chan struct{}, error)
}
