package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type captureClassifierModule struct {
	ready atomic.Bool
	cfg   config
	core  trovemodule.Core
}

func (m *captureClassifierModule) Run(ctx context.Context, core trovemodule.Core) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if core == nil {
		return fmt.Errorf("capture-classifier: core connection is required")
	}

	m.cfg = cfg
	m.core = core
	m.ready.Store(true)
	defer m.ready.Store(false)

	<-ctx.Done()
	return nil
}

func (m *captureClassifierModule) HandleHTTP(ctx context.Context, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if !m.ready.Load() {
		return textResponse(http.StatusServiceUnavailable, "service unavailable"), nil
	}
	return dispatchHTTP(ctx, &journalAdapter{core: m.core}, m.cfg, req)
}

func (m *captureClassifierModule) CallTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error) {
	if !m.ready.Load() {
		return nil, fmt.Errorf("capture-classifier: not ready")
	}
	j := &journalAdapter{core: m.core}
	switch name {
	case "classify_event":
		var params struct {
			SourceEventID string          `json:"source_event_id"`
			TargetType    string          `json:"target_type"`
			Payload       json.RawMessage `json:"payload"`
		}
		if len(arguments) > 0 {
			if err := json.Unmarshal(arguments, &params); err != nil {
				return nil, fmt.Errorf("capture-classifier: invalid arguments: %w", err)
			}
		}
		result, err := classify.Classify(ctx, j, classify.ClassifyRequest{
			SourceEventID: params.SourceEventID,
			TargetType:    params.TargetType,
			Payload:       params.Payload,
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	case "list_unclassified_captures":
		events, err := classify.ListUnclassified(ctx, j)
		if err != nil {
			return nil, err
		}
		return json.Marshal(events)
	default:
		return nil, fmt.Errorf("capture-classifier: unknown tool %q", name)
	}
}

func (m *captureClassifierModule) Healthcheck(context.Context) (*troverpc.HealthcheckResponse, error) {
	if m.ready.Load() {
		return &troverpc.HealthcheckResponse{Ok: true, Message: "capture-classifier ready"}, nil
	}
	return &troverpc.HealthcheckResponse{Ok: false, Message: "capture-classifier not ready"}, nil
}

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

func main() {
	trovemodule.Serve(&captureClassifierModule{})
}
