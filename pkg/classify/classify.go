package classify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/records"
)

// ErrNotFound indicates the record does not exist.
var ErrNotFound = errors.New("capture: record not found")

// ErrNotIncomplete indicates the record is not awaiting classification.
var ErrNotIncomplete = errors.New("capture: record is not incomplete")

// Store supports incomplete capture and classification.
type Store interface {
	GetRecord(ctx context.Context, req *troverpc.GetRecordRequest) (*troverpc.Record, error)
	ListIncompleteRecords(ctx context.Context, req *troverpc.ListIncompleteRecordsRequest) ([]*troverpc.Record, error)
	RecordWrite(ctx context.Context, req *troverpc.WriteRequest) (*troverpc.WriteResponse, error)
}

// ClassifyRequest identifies an incomplete record and desired target type.
type ClassifyRequest struct {
	RecordRef  string
	TargetType string
	Payload    json.RawMessage
}

// ClassifyResult returns the write response identifiers.
type ClassifyResult struct {
	RecordRef string `json:"record_ref"`
	EventID   string `json:"event_id"`
	Version   int32  `json:"version"`
}

// CaptureResult is returned when an incomplete capture is stored.
type CaptureResult struct {
	RecordRef string
	EventID   string
}

// Capture stores a quick capture without a record type.
func Capture(ctx context.Context, s Store, source string, body []byte) (CaptureResult, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return CaptureResult{}, fmt.Errorf("capture: source is required")
	}
	if len(body) == 0 {
		return CaptureResult{}, fmt.Errorf("capture: body is required")
	}
	if !json.Valid(body) {
		return CaptureResult{}, fmt.Errorf("capture: body must be valid JSON")
	}

	req, err := buildCaptureRequest(source, body)
	if err != nil {
		return CaptureResult{}, err
	}
	resp, err := s.RecordWrite(ctx, req)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		RecordRef: resp.GetRecordRef(),
		EventID:   resp.GetEventId(),
	}, nil
}

// Classify sets the type on an incomplete record via apply.
func Classify(ctx context.Context, s Store, req ClassifyRequest) (ClassifyResult, error) {
	req.RecordRef = strings.TrimSpace(req.RecordRef)
	req.TargetType = strings.TrimSpace(req.TargetType)
	if req.RecordRef == "" {
		return ClassifyResult{}, fmt.Errorf("capture: record_ref is required")
	}
	if req.TargetType == "" {
		return ClassifyResult{}, fmt.Errorf("capture: target_type is required")
	}

	rec, err := s.GetRecord(ctx, &troverpc.GetRecordRequest{RecordRef: req.RecordRef})
	if err != nil {
		return ClassifyResult{}, err
	}
	if rec == nil {
		return ClassifyResult{}, ErrNotFound
	}
	if rec.GetCompleteness() != records.CompletenessIncomplete {
		return ClassifyResult{}, ErrNotIncomplete
	}

	payload, err := mergePayload(rec.GetBody(), req.Payload)
	if err != nil {
		return ClassifyResult{}, err
	}

	resp, err := s.RecordWrite(ctx, &troverpc.WriteRequest{
		Operation: "apply",
		RecordRef: req.RecordRef,
		Type:      req.TargetType,
		Source:    rec.GetSource(),
		Payload:   payload,
	})
	if err != nil {
		return ClassifyResult{}, err
	}
	return ClassifyResult{
		RecordRef: resp.GetRecordRef(),
		EventID:   resp.GetEventId(),
		Version:   resp.GetVersion(),
	}, nil
}

// ListIncomplete returns records awaiting classification.
func ListIncomplete(ctx context.Context, s Store, source string, limit int32) ([]*troverpc.Record, error) {
	return s.ListIncompleteRecords(ctx, &troverpc.ListIncompleteRecordsRequest{
		Source: source,
		Limit:  limit,
	})
}

func buildCaptureRequest(source string, body []byte) (*troverpc.WriteRequest, error) {
	req := &troverpc.WriteRequest{
		Operation: "apply",
		Source:    source,
		Payload:   body,
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return req, nil
	}

	payload := make(map[string]json.RawMessage, len(obj))
	for key, value := range obj {
		switch key {
		case "time":
			var timeStr string
			if err := json.Unmarshal(value, &timeStr); err != nil {
				return nil, fmt.Errorf("capture: invalid time field")
			}
			if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
				return nil, fmt.Errorf("capture: invalid time field")
			}
			req.Time = timeStr
		case "blob_ref":
			var blobRef string
			if err := json.Unmarshal(value, &blobRef); err != nil || blobRef == "" {
				return nil, fmt.Errorf("capture: invalid blob_ref field")
			}
			req.BlobRef = blobRef
		default:
			payload[key] = value
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("capture: marshal payload: %w", err)
	}
	req.Payload = payloadBytes
	return req, nil
}

func mergePayload(body []byte, overrides json.RawMessage) ([]byte, error) {
	base := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &base); err != nil {
			return nil, fmt.Errorf("capture: parse record body: %w", err)
		}
	}
	if len(overrides) > 0 {
		extra := map[string]any{}
		if err := json.Unmarshal(overrides, &extra); err != nil {
			return nil, fmt.Errorf("capture: parse override payload: %w", err)
		}
		for key, value := range extra {
			base[key] = value
		}
	}
	out, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("capture: marshal merged payload: %w", err)
	}
	return out, nil
}
