package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/classify"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func dispatchHTTP(ctx context.Context, j classify.Journal, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	if req == nil {
		return textResponse(http.StatusBadRequest, "request is required"), nil
	}

	key := req.Method + " " + req.MatchedPattern
	switch key {
	case "POST /capture/{source}":
		return handleCapture(ctx, j, cfg, req)
	case "POST /classify":
		return handleClassify(ctx, j, cfg, req)
	case "GET /pending":
		return handlePending(ctx, j)
	default:
		return textResponse(http.StatusNotFound, "not found"), nil
	}
}

func handleCapture(ctx context.Context, j classify.Journal, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
	source := req.PathValues["source"]
	body := req.Body
	if len(body) == 0 {
		return textResponse(http.StatusBadRequest, "request body is required"), nil
	}
	if int64(len(body)) > cfg.MaxBodyBytes {
		return textResponse(http.StatusBadRequest, "request body too large"), nil
	}

	if err := classify.CapturePending(ctx, j, source, body); err != nil {
		if isClientError(err) {
			return textResponse(http.StatusBadRequest, err.Error()), nil
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			return textResponse(http.StatusBadRequest, st.Message()), nil
		}
		return textResponse(http.StatusInternalServerError, "failed to capture event"), nil
	}
	return &troverpc.HTTPResponse{Status: http.StatusNoContent}, nil
}

func handleClassify(ctx context.Context, j classify.Journal, cfg config, req *troverpc.HTTPRequest) (*troverpc.HTTPResponse, error) {
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

	var params struct {
		SourceEventID string          `json:"source_event_id"`
		TargetType    string          `json:"target_type"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(body, &params); err != nil {
		return textResponse(http.StatusBadRequest, "invalid JSON"), nil
	}

	result, err := classify.Classify(ctx, j, classify.ClassifyRequest{
		SourceEventID: params.SourceEventID,
		TargetType:    params.TargetType,
		Payload:       params.Payload,
	})
	if err != nil {
		if errors.Is(err, classify.ErrNotFound) {
			return textResponse(http.StatusNotFound, err.Error()), nil
		}
		if errors.Is(err, classify.ErrNotPending) || errors.Is(err, classify.ErrAlreadyClassified) {
			return textResponse(http.StatusConflict, err.Error()), nil
		}
		if isClientError(err) {
			return textResponse(http.StatusBadRequest, err.Error()), nil
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			return textResponse(http.StatusBadRequest, st.Message()), nil
		}
		return textResponse(http.StatusInternalServerError, "failed to classify event"), nil
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to encode response"), nil
	}
	return &troverpc.HTTPResponse{
		Status:  http.StatusCreated,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    payload,
	}, nil
}

func handlePending(ctx context.Context, j classify.Journal) (*troverpc.HTTPResponse, error) {
	events, err := classify.ListUnclassified(ctx, j)
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to list pending captures"), nil
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return textResponse(http.StatusInternalServerError, "failed to encode response"), nil
	}
	return &troverpc.HTTPResponse{
		Status:  http.StatusOK,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    payload,
	}, nil
}

func isClientError(err error) bool {
	return len(err.Error()) > 0 && (errors.Is(err, classify.ErrNotPending) ||
		errors.Is(err, classify.ErrAlreadyClassified) ||
		err.Error() == "classify: source is required" ||
		err.Error() == "classify: body is required" ||
		err.Error() == "classify: body must be valid JSON" ||
		err.Error() == "classify: source_event_id is required" ||
		err.Error() == "classify: target_type is required")
}

func textResponse(status int, message string) *troverpc.HTTPResponse {
	return &troverpc.HTTPResponse{
		Status: int32(status), //nolint:gosec // G115: bounded HTTP status code
		Body:   []byte(message),
	}
}
