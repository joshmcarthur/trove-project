package main

import (
	"context"

	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

type journalAdapter struct {
	core trovemodule.Core
}

func (a *journalAdapter) GetEvent(ctx context.Context, id string) (*troverpc.Event, error) {
	event, err := a.core.GetEvent(ctx, id)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, classify.ErrNotFound
		}
		return nil, err
	}
	return event, nil
}

func (a *journalAdapter) GetEventsByType(ctx context.Context, eventType string) ([]*troverpc.Event, error) {
	return a.core.GetEventsByType(ctx, &troverpc.GetEventsByTypeRequest{Type: eventType})
}

func (a *journalAdapter) Emit(ctx context.Context, event *troverpc.Event) error {
	return a.core.Emit(ctx, event)
}
