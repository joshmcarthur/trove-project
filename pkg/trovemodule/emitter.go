package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RecordWriter appends record events to the Trove journal via Core.RecordWrite.
type RecordWriter interface {
	RecordWrite(ctx context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error)
}

// ApplyRecord writes an apply operation using event-shaped fields.
func ApplyRecord(ctx context.Context, w RecordWriter, event *troverpc.Event) (*troverpc.WriteResponse, error) {
	return w.RecordWrite(ctx, applyWriteRequest(event))
}

func applyWriteRequest(event *troverpc.Event) *troverpc.WriteRequest {
	if event == nil {
		return &troverpc.WriteRequest{Operation: "apply"}
	}
	operation := event.GetOperation()
	if operation == "" {
		operation = "apply"
	}
	return &troverpc.WriteRequest{
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
