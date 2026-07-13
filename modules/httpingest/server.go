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

type putBlobResponse struct {
	BlobRef string `json:"blob_ref"`
}

type writeResponse struct {
	EventID      string `json:"event_id"`
	RecordRef    string `json:"record_ref"`
	Version      int32  `json:"version"`
	Completeness string `json:"completeness"`
	Operation    string `json:"operation"`
}

func dispatchHTTP(ctx context.Context, writer trovemodule.RecordEmitter, blobs trovemodule.BlobPutter, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if req == nil {
		return textResponse(http.StatusBadRequest, "request is required"), nil
	}

	key := req.Method + " " + req.MatchedPattern
	switch key {
	case "POST /records":
		return handleRecords(ctx, writer, cfg, req)
	case "PUT /blobs":
		return handlePutBlob(ctx, blobs, cfg, req)
	default:
		return textResponse(http.StatusNotFound, "not found"), nil
	}
}

func handleRecords(ctx context.Context, writer trovemodule.RecordEmitter, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
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

	writeReq, err := buildEmitRecordRequest(body)
	if err != nil {
		return textResponse(http.StatusBadRequest, err.Error()), nil
	}
	if writeReq.Source == "" {
		return textResponse(http.StatusBadRequest, "source is required"), nil
	}

	if writeReq.Operation == "" || writeReq.Operation == "apply" {
		if writeReq.Type != "" && len(cfg.Provides) > 0 && !trovemodule.MatchType(cfg.Provides, writeReq.Type) {
			return textResponse(http.StatusBadRequest, fmt.Sprintf("type not allowed: %s", writeReq.Type)), nil
		}
	}

	resp, err := writer.EmitRecord(ctx, writeReq)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				return textResponse(http.StatusBadRequest, st.Message()), nil
			case codes.NotFound:
				return textResponse(http.StatusNotFound, st.Message()), nil
			case codes.FailedPrecondition, codes.AlreadyExists:
				return textResponse(http.StatusConflict, st.Message()), nil
			}
		}
		return textResponse(http.StatusInternalServerError, "failed to write record"), nil
	}

	payload, err := json.Marshal(writeResponse{
		EventID:      resp.GetEventId(),
		RecordRef:    resp.GetRecordRef(),
		Version:      resp.GetVersion(),
		Completeness: resp.GetCompleteness(),
		Operation:    resp.GetOperation(),
	})
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to encode response"), nil
	}

	return &troverpc.HTTPResponse{
		Status:  http.StatusCreated,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    payload,
	}, nil
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

func buildEmitRecordRequest(body []byte) (*troverpc.EmitRecordRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON object")
	}

	req := &troverpc.EmitRecordRequest{
		Operation: "apply",
		Payload:   []byte("{}"),
	}

	if v, ok := raw["operation"]; ok {
		var op string
		if err := json.Unmarshal(v, &op); err != nil || op == "" {
			return nil, fmt.Errorf("invalid operation field")
		}
		req.Operation = op
		delete(raw, "operation")
	}
	if v, ok := raw["record_ref"]; ok {
		var ref string
		if err := json.Unmarshal(v, &ref); err != nil {
			return nil, fmt.Errorf("invalid record_ref field")
		}
		req.RecordRef = ref
		delete(raw, "record_ref")
	}
	if v, ok := raw["type"]; ok {
		var typ string
		if err := json.Unmarshal(v, &typ); err != nil {
			return nil, fmt.Errorf("invalid type field")
		}
		req.Type = typ
		delete(raw, "type")
	}
	if v, ok := raw["time"]; ok {
		var timeStr string
		if err := json.Unmarshal(v, &timeStr); err != nil {
			return nil, fmt.Errorf("invalid time field")
		}
		if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
			return nil, fmt.Errorf("invalid time field")
		}
		req.Time = timeStr
		delete(raw, "time")
	}
	if v, ok := raw["source"]; ok {
		var source string
		if err := json.Unmarshal(v, &source); err != nil || source == "" {
			return nil, fmt.Errorf("invalid source field")
		}
		req.Source = source
		delete(raw, "source")
	}
	if v, ok := raw["blob_ref"]; ok {
		var blobRef string
		if err := json.Unmarshal(v, &blobRef); err != nil || blobRef == "" {
			return nil, fmt.Errorf("invalid blob_ref field")
		}
		req.BlobRef = blobRef
		delete(raw, "blob_ref")
	}
	if v, ok := raw["transforms"]; ok {
		if !json.Valid(v) {
			return nil, fmt.Errorf("invalid transforms field")
		}
		req.Transforms = v
		delete(raw, "transforms")
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal payload")
	}
	if len(payload) > 0 {
		req.Payload = payload
	}
	return req, nil
}

func textResponse(status int, message string) *troverpc.HTTPResponse {
	return &troverpc.HTTPResponse{
		Status: int32(status), //nolint:gosec // G115: bounded HTTP status code
		Body:   []byte(message),
	}
}
