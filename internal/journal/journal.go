package journal

import "context"

// Journal is the append-only event store interface.
type Journal interface {
	Append(ctx context.Context, e Event) error
	Query(ctx context.Context, f Filter) ([]Event, error)
	Get(ctx context.Context, id string) (Event, error)
	Watch(ctx context.Context) (<-chan struct{}, error)
}
