package classify

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/oklog/ulid"
)

const (
	// PendingType is the event type for quick captures awaiting classification.
	PendingType = "classify.pending"
	// AssignedType records a completed classification.
	AssignedType = "classify.assigned"
)

// ErrNotFound indicates the source event does not exist.
var ErrNotFound = errors.New("classify: event not found")

// ErrNotPending indicates the source event is not a pending capture.
var ErrNotPending = errors.New("classify: source event is not classify.pending")

// ErrAlreadyClassified indicates the pending event was already classified.
var ErrAlreadyClassified = errors.New("classify: event already classified")

// Journal supports classify capture and assignment operations.
type Journal interface {
	GetEvent(ctx context.Context, id string) (*troverpc.Event, error)
	GetEventsByType(ctx context.Context, eventType string) ([]*troverpc.Event, error)
	Emit(ctx context.Context, event *troverpc.Event) error
}

// ClassifyRequest identifies a pending capture and desired target type.
type ClassifyRequest struct {
	SourceEventID string
	TargetType    string
	Payload       json.RawMessage
}

// ClassifyResult returns the new event identifiers.
type ClassifyResult struct {
	TargetEventID         string `json:"target_event_id"`
	ClassificationEventID string `json:"classification_event_id"`
}

// CapturePending stores a quick capture as classify.pending.
func CapturePending(ctx context.Context, j Journal, source string, body []byte) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("classify: source is required")
	}
	if len(body) == 0 {
		return fmt.Errorf("classify: body is required")
	}
	if !json.Valid(body) {
		return fmt.Errorf("classify: body must be valid JSON")
	}

	event, err := buildPendingEvent(source, body)
	if err != nil {
		return err
	}
	return j.Emit(ctx, event)
}

// Classify creates a typed event and classify.assigned link from a pending capture.
func Classify(ctx context.Context, j Journal, req ClassifyRequest) (ClassifyResult, error) {
	req.SourceEventID = strings.TrimSpace(req.SourceEventID)
	req.TargetType = strings.TrimSpace(req.TargetType)
	if req.SourceEventID == "" {
		return ClassifyResult{}, fmt.Errorf("classify: source_event_id is required")
	}
	if req.TargetType == "" {
		return ClassifyResult{}, fmt.Errorf("classify: target_type is required")
	}

	source, err := j.GetEvent(ctx, req.SourceEventID)
	if err != nil {
		return ClassifyResult{}, err
	}
	if source == nil {
		return ClassifyResult{}, ErrNotFound
	}
	if source.Type != PendingType {
		return ClassifyResult{}, ErrNotPending
	}

	classified, err := isClassified(ctx, j, req.SourceEventID)
	if err != nil {
		return ClassifyResult{}, err
	}
	if classified {
		return ClassifyResult{}, ErrAlreadyClassified
	}

	targetPayload, err := mergePayload(source.Payload, req.Payload, req.SourceEventID)
	if err != nil {
		return ClassifyResult{}, err
	}

	target := &troverpc.Event{
		Id:      ulid.MustNew(ulid.Now(), rand.Reader).String(),
		Type:    req.TargetType,
		Source:  source.Source,
		Payload: targetPayload,
		Time:    source.Time,
		BlobRef: source.BlobRef,
	}
	if err := j.Emit(ctx, target); err != nil {
		return ClassifyResult{}, err
	}

	linkPayload, err := json.Marshal(map[string]string{
		"source_event_id": req.SourceEventID,
		"target_event_id": target.Id,
		"target_type":     req.TargetType,
	})
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("classify: marshal link payload: %w", err)
	}

	link := &troverpc.Event{
		Id:      ulid.MustNew(ulid.Now(), rand.Reader).String(),
		Type:    AssignedType,
		Source:  source.Source,
		Payload: linkPayload,
		Time:    source.Time,
	}
	if err := j.Emit(ctx, link); err != nil {
		return ClassifyResult{}, err
	}

	return ClassifyResult{
		TargetEventID:         target.Id,
		ClassificationEventID: link.Id,
	}, nil
}

// ListUnclassified returns pending captures without a classify.assigned link.
func ListUnclassified(ctx context.Context, j Journal) ([]*troverpc.Event, error) {
	pending, err := j.GetEventsByType(ctx, PendingType)
	if err != nil {
		return nil, err
	}
	assigned, err := j.GetEventsByType(ctx, AssignedType)
	if err != nil {
		return nil, err
	}

	classified := make(map[string]struct{}, len(assigned))
	for _, event := range assigned {
		var link struct {
			SourceEventID string `json:"source_event_id"`
		}
		if err := json.Unmarshal(event.Payload, &link); err != nil {
			continue
		}
		if link.SourceEventID != "" {
			classified[link.SourceEventID] = struct{}{}
		}
	}

	out := make([]*troverpc.Event, 0, len(pending))
	for _, event := range pending {
		if _, done := classified[event.Id]; !done {
			out = append(out, event)
		}
	}
	return out, nil
}

func isClassified(ctx context.Context, j Journal, sourceID string) (bool, error) {
	assigned, err := j.GetEventsByType(ctx, AssignedType)
	if err != nil {
		return false, err
	}
	for _, event := range assigned {
		var link struct {
			SourceEventID string `json:"source_event_id"`
		}
		if err := json.Unmarshal(event.Payload, &link); err != nil {
			continue
		}
		if link.SourceEventID == sourceID {
			return true, nil
		}
	}
	return false, nil
}

func buildPendingEvent(source string, body []byte) (*troverpc.Event, error) {
	event := &troverpc.Event{
		Type:    PendingType,
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
		case "time":
			var timeStr string
			if err := json.Unmarshal(value, &timeStr); err != nil {
				return nil, fmt.Errorf("classify: invalid time field")
			}
			if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
				return nil, fmt.Errorf("classify: invalid time field")
			}
			event.Time = timeStr
		case "blob_ref":
			var blobRef string
			if err := json.Unmarshal(value, &blobRef); err != nil || blobRef == "" {
				return nil, fmt.Errorf("classify: invalid blob_ref field")
			}
			event.BlobRef = blobRef
		default:
			payload[key] = value
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("classify: marshal payload: %w", err)
	}
	event.Payload = payloadBytes
	return event, nil
}

func mergePayload(source json.RawMessage, overrides json.RawMessage, derivedFrom string) ([]byte, error) {
	base := map[string]any{}
	if len(source) > 0 {
		if err := json.Unmarshal(source, &base); err != nil {
			return nil, fmt.Errorf("classify: parse source payload: %w", err)
		}
	}
	if len(overrides) > 0 {
		extra := map[string]any{}
		if err := json.Unmarshal(overrides, &extra); err != nil {
			return nil, fmt.Errorf("classify: parse override payload: %w", err)
		}
		for key, value := range extra {
			base[key] = value
		}
	}

	trove, _ := base["_trove"].(map[string]any)
	if trove == nil {
		trove = map[string]any{}
	}
	trove["derived_from"] = derivedFrom
	base["_trove"] = trove

	out, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("classify: marshal merged payload: %w", err)
	}
	return out, nil
}
