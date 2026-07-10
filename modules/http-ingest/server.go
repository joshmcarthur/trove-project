package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultEventType = "http.ingest.received"

type putBlobResponse struct {
	BlobRef string `json:"blob_ref"`
}

func runHTTPServer(ctx context.Context, emit trovemodule.Emitter, cfg config, blobs blob.Store) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ingest/{source}", func(w http.ResponseWriter, r *http.Request) {
		handleIngest(w, r, emit, cfg)
	})
	mux.HandleFunc("/ingest/{source}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("PUT /blobs", func(w http.ResponseWriter, r *http.Request) {
		handlePutBlob(w, r, blobs, cfg)
	})
	mux.HandleFunc("/blobs", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func handleIngest(w http.ResponseWriter, r *http.Request, emit trovemodule.Emitter, cfg config) {
	source := r.PathValue("source")
	if source == "" {
		http.Error(w, "source is required", http.StatusBadRequest)
		return
	}

	body, err := readBody(w, r, cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := buildEvent(source, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(cfg.Provides) > 0 && !trovemodule.MatchType(cfg.Provides, event.Type) {
		http.Error(w, fmt.Sprintf("type not allowed: %s", event.Type), http.StatusBadRequest)
		return
	}

	if err := emit.Emit(r.Context(), event); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			http.Error(w, st.Message(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to ingest event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handlePutBlob(w http.ResponseWriter, r *http.Request, blobs blob.Store, cfg config) {
	body, err := readRawBody(w, r, cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ref, err := blobs.Put(r.Context(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to store blob", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(putBlobResponse{BlobRef: ref})
}

func readRawBody(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) ([]byte, error) {
	limited := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer limited.Close()

	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, fmt.Errorf("request body too large")
		}
		return nil, fmt.Errorf("read body")
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is required")
	}
	return body, nil
}

func readBody(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) ([]byte, error) {
	limited := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer limited.Close()

	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, fmt.Errorf("request body too large")
		}
		return nil, fmt.Errorf("read body")
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is required")
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid JSON")
	}
	return body, nil
}

func buildEvent(source string, body []byte) (*troverpc.Event, error) {
	event := &troverpc.Event{
		Type:    defaultEventType,
		Source:  source,
		Payload: body,
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		// Arrays and primitives use the entire body as payload.
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
