package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RecordEmitter appends record events to the Trove journal via Core.EmitRecord.
type RecordEmitter interface {
	EmitRecord(ctx context.Context, req *troverpc.EmitRecordRequest) (*troverpc.EmitRecordResponse, error)
}

// Emitter appends a record event using event-shaped fields (wraps EmitRecord).
type Emitter interface {
	Emit(ctx context.Context, event *troverpc.Event) error
}

// EmitRecordFromEvent emits a record event using journal event-shaped fields.
func EmitRecordFromEvent(ctx context.Context, e RecordEmitter, event *troverpc.Event) (*troverpc.EmitRecordResponse, error) {
	return e.EmitRecord(ctx, eventToEmitRecordRequest(event))
}

func eventToEmitRecordRequest(event *troverpc.Event) *troverpc.EmitRecordRequest {
	if event == nil {
		return &troverpc.EmitRecordRequest{Operation: "apply"}
	}
	operation := event.GetOperation()
	if operation == "" {
		operation = "apply"
	}
	return &troverpc.EmitRecordRequest{
		Operation:  operation,
		RecordRef:  event.GetRecordRef(),
		Type:       event.GetType(),
		Time:       event.GetTime(),
		Source:     event.GetSource(),
		Payload:    event.GetPayload(),
		Transforms: event.GetTransforms(),
		BlobRef:    event.GetBlobRef(),
	}
}

// HealthChecker reports module liveness for periodic parent healthchecks.
type HealthChecker interface {
	Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error)
}
