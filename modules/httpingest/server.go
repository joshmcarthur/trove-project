package httpingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultEventType = "trove://type/http/ingest/received/1"

type putBlobResponse struct {
	BlobRef string `json:"blob_ref"`
}

func dispatchHTTP(ctx context.Context, emit trovemodule.Emitter, blobs trovemodule.BlobPutter, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if req == nil {
		return textResponse(http.StatusBadRequest, "request is required"), nil
	}

	key := req.Method + " " + req.MatchedPattern
	switch key {
	case "POST /ingest/{source}":
		return handleIngest(ctx, emit, cfg, req)
	case "PUT /blobs":
		return handlePutBlob(ctx, blobs, cfg, req)
	default:
		return textResponse(http.StatusNotFound, "not found"), nil
	}
}

func handleIngest(ctx context.Context, emit trovemodule.Emitter, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	source := req.PathValues["source"]
	if source == "" {
		return textResponse(http.StatusBadRequest, "source is required"), nil
	}

	body := req.Body
	if len(body) == 0 {
		return textResponse(http.StatusBadRequest, "request body is required"), nil
	}
	if int64(len(body)) > cfg.MaxBodyBytes {
		return textResponse(http.StatusBadRequest, "request body too large"), nil
	}
	if !json.Valid(body) {
		return textResponse(http.StatusBadRequest, "invalid JSON"), nil
	}

	event, err := buildEvent(source, body)
	if err != nil {
		return textResponse(http.StatusBadRequest, err.Error()), nil
	}

	if len(cfg.Provides) > 0 && !trovemodule.MatchType(cfg.Provides, event.Type) {
		return textResponse(http.StatusBadRequest, fmt.Sprintf("type not allowed: %s", event.Type)), nil
	}

	if err := emit.Emit(ctx, event); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			return textResponse(http.StatusBadRequest, st.Message()), nil
		}
		return textResponse(http.StatusInternalServerError, "failed to ingest event"), nil
	}

	return &troverpc.HTTPResponse{Status: http.StatusNoContent}, nil
}

func handlePutBlob(ctx context.Context, blobs trovemodule.BlobPutter, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	body := req.Body
	if len(body) == 0 {
		return textResponse(http.StatusBadRequest, "request body is required"), nil
	}
	if int64(len(body)) > cfg.MaxBodyBytes {
		return textResponse(http.StatusBadRequest, "request body too large"), nil
	}

	ref, err := blobs.Put(ctx, body)
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to store blob"), nil
	}

	payload, err := json.Marshal(putBlobResponse{BlobRef: ref})
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to encode response"), nil
	}

	return &troverpc.HTTPResponse{
		Status:  http.StatusCreated,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    payload,
	}, nil
}

func buildEvent(source string, body []byte) (*troverpc.Event, error) {
	event := &troverpc.Event{
		Type:    defaultEventType,
		Source:  source,
		Payload: body,
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return event, nil
	}

	payload := make(map[string]json.RawMessage, len(obj))
	for key, value := range obj {
		switch key {
		case "type":
			var eventType string
			if err := json.Unmarshal(value, &eventType); err != nil || eventType == "" {
				return nil, fmt.Errorf("invalid type field")
			}
			event.Type = eventType
		case "time":
			var timeStr string
			if err := json.Unmarshal(value, &timeStr); err != nil {
				return nil, fmt.Errorf("invalid time field")
			}
			if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
				return nil, fmt.Errorf("invalid time field")
			}
			event.Time = timeStr
		case "blob_ref":
			var blobRef string
			if err := json.Unmarshal(value, &blobRef); err != nil || blobRef == "" {
				return nil, fmt.Errorf("invalid blob_ref field")
			}
			event.BlobRef = blobRef
		default:
			payload[key] = value
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload")
	}
	event.Payload = payloadBytes
	return event, nil
}

func textResponse(status int, message string) *troverpc.HTTPResponse {
	return &troverpc.HTTPResponse{
		Status: int32(status), //nolint:gosec // G115: bounded HTTP status code
		Body:   []byte(message),
	}
}
