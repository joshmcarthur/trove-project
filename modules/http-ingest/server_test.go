package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/blob"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockEmitter struct {
	events []*troverpc.Event
	err    error
}

func (m *mockEmitter) Emit(_ context.Context, event *troverpc.Event) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func defaultTestConfig() config {
	return config{
		MaxBodyBytes: defaultMaxBodyBytes,
		Provides:     []string{"http.ingest.received", "note.*", "shortcut.*"},
	}
}

func TestHandleIngest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		source     string
		body       string
		emitErr    error
		wantStatus int
		wantEmit   bool
		checkEvent func(t *testing.T, event *troverpc.Event)
	}{
		{
			name:       "valid object",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"title":"test"}`,
			wantStatus: http.StatusNoContent,
			wantEmit:   true,
			checkEvent: func(t *testing.T, event *troverpc.Event) {
				t.Helper()
				if event.Source != "shortcuts" {
					t.Errorf("Source = %q, want shortcuts", event.Source)
				}
				if event.Type != defaultEventType {
					t.Errorf("Type = %q, want %q", event.Type, defaultEventType)
				}
				if string(event.Payload) != `{"title":"test"}` {
					t.Errorf("Payload = %s, want %s", event.Payload, `{"title":"test"}`)
				}
			},
		},
		{
			name:       "custom type and time",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"type":"note.created","time":"2026-07-10T12:00:00Z","title":"test"}`,
			wantStatus: http.StatusNoContent,
			wantEmit:   true,
			checkEvent: func(t *testing.T, event *troverpc.Event) {
				t.Helper()
				if event.Type != "note.created" {
					t.Errorf("Type = %q, want note.created", event.Type)
				}
				if event.Time != "2026-07-10T12:00:00Z" {
					t.Errorf("Time = %q, want RFC3339 timestamp", event.Time)
				}
				if string(event.Payload) != `{"title":"test"}` {
					t.Errorf("Payload = %s, want %s", event.Payload, `{"title":"test"}`)
				}
			},
		},
		{
			name:       "array payload",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `["a","b"]`,
			wantStatus: http.StatusNoContent,
			wantEmit:   true,
			checkEvent: func(t *testing.T, event *troverpc.Event) {
				t.Helper()
				if string(event.Payload) != `["a","b"]` {
					t.Errorf("Payload = %s, want %s", event.Payload, `["a","b"]`)
				}
			},
		},
		{
			name:       "blob_ref field",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"blob_ref":"sha256-deadbeef","title":"photo note"}`,
			wantStatus: http.StatusNoContent,
			wantEmit:   true,
			checkEvent: func(t *testing.T, event *troverpc.Event) {
				t.Helper()
				if event.BlobRef != "sha256-deadbeef" {
					t.Errorf("BlobRef = %q, want sha256-deadbeef", event.BlobRef)
				}
				if string(event.Payload) != `{"title":"photo note"}` {
					t.Errorf("Payload = %s, want %s", event.Payload, `{"title":"photo note"}`)
				}
			},
		},
		{
			name:       "invalid blob_ref field",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"blob_ref":123}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{not-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid type field",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"type":123}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid time field",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"time":"not-a-time"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "disallowed type",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"type":"mqtt.sensor.temp","title":"test"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "emit invalid argument",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"title":"test"}`,
			emitErr:    status.Error(codes.InvalidArgument, "payload does not match schema for type \"http.ingest.received\": missing title"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "emit failure",
			method:     http.MethodPost,
			source:     "shortcuts",
			body:       `{"title":"test"}`,
			emitErr:    errors.New("emit failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			emit := &mockEmitter{err: tt.emitErr}
			req := httptest.NewRequest(tt.method, "/ingest/"+tt.source, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.SetPathValue("source", tt.source)

			rec := httptest.NewRecorder()
			handleIngest(rec, req, emit, defaultTestConfig())

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %q", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantEmit {
				if len(emit.events) != 1 {
					t.Fatalf("Emit calls = %d, want 1", len(emit.events))
				}
				if tt.checkEvent != nil {
					tt.checkEvent(t, emit.events[0])
				}
			} else if len(emit.events) != 0 {
				t.Fatalf("Emit calls = %d, want 0", len(emit.events))
			}
		})
	}
}

func TestIngestRouteMethodNotAllowed(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ingest/{source}", func(w http.ResponseWriter, r *http.Request) {
		handleIngest(w, r, &mockEmitter{}, defaultTestConfig())
	})
	mux.HandleFunc("/ingest/{source}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	req := httptest.NewRequest(http.MethodGet, "/ingest/shortcuts", nil)
	req.SetPathValue("source", "shortcuts")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(`name = "http-ingest"`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if cfg.Listen != defaultListen {
		t.Errorf("Listen = %q, want %q", cfg.Listen, defaultListen)
	}
	if cfg.MaxBodyBytes != defaultMaxBodyBytes {
		t.Errorf("MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, defaultMaxBodyBytes)
	}
}

func TestLoadConfigCustomMaxBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `listen = ":9090"
max_body_bytes = 2048
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	cfg, err := loadConfigFromDir(dir)
	if err != nil {
		t.Fatalf("loadConfigFromDir() error = %v", err)
	}
	if cfg.MaxBodyBytes != 2048 {
		t.Errorf("MaxBodyBytes = %d, want 2048", cfg.MaxBodyBytes)
	}
}

func TestRunHTTPServerShutdown(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	emit := &mockEmitter{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(ctx, emit, config{Listen: addr, MaxBodyBytes: defaultMaxBodyBytes, Provides: defaultTestConfig().Provides}, nil)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("runHTTPServer() error = %v", err)
	}
}

func TestReadBodyTooLarge(t *testing.T) {
	t.Parallel()

	const limit = 1024
	body := strings.Repeat("a", limit+1)
	req := httptest.NewRequest(http.MethodPost, "/ingest/test", strings.NewReader(body))
	_, err := readBody(httptest.NewRecorder(), req, limit)
	if err == nil {
		t.Fatal("readBody() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("readBody() error = %v, want body too large", err)
	}
}

func TestReadBodyUsesLimitedReader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/ingest/test", io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))))
	body, err := readBody(httptest.NewRecorder(), req, defaultMaxBodyBytes)
	if err != nil {
		t.Fatalf("readBody() error = %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body = %s, want payload", body)
	}
}

var _ trovemodule.Emitter = (*mockEmitter)(nil)

func openTestBlobStore(t *testing.T) blob.Store {
	t.Helper()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("blob.OpenFilesystem() error = %v", err)
	}
	return store
}

func TestHandleBlobPut(t *testing.T) {
	t.Parallel()

	blobData := []byte("test image bytes")

	tests := []struct {
		name       string
		method     string
		body       []byte
		maxBytes   int64
		wantStatus int
		wantRef    bool
	}{
		{
			name:       "successful upload",
			method:     http.MethodPut,
			body:       blobData,
			maxBytes:   defaultMaxBodyBytes,
			wantStatus: http.StatusOK,
			wantRef:    true,
		},
		{
			name:       "empty body",
			method:     http.MethodPut,
			body:       nil,
			maxBytes:   defaultMaxBodyBytes,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "body too large",
			method:     http.MethodPut,
			body:       bytes.Repeat([]byte("a"), 1025),
			maxBytes:   1024,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "blob store not configured",
			method:     http.MethodPut,
			body:       blobData,
			maxBytes:   defaultMaxBodyBytes,
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var blobs blob.Store
			if tt.name != "blob store not configured" {
				blobs = openTestBlobStore(t)
			}

			var bodyReader io.Reader
			if tt.body != nil {
				bodyReader = bytes.NewReader(tt.body)
			} else {
				bodyReader = strings.NewReader("")
			}

			req := httptest.NewRequest(tt.method, "/blobs", bodyReader)
			rec := httptest.NewRecorder()
			handleBlobPut(rec, req, blobs, tt.maxBytes)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if !tt.wantRef {
				return
			}

			var resp blobUploadResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !strings.HasPrefix(resp.BlobRef, "sha256-") {
				t.Fatalf("BlobRef = %q, want sha256- prefix", resp.BlobRef)
			}

			rc, err := blobs.Get(req.Context(), resp.BlobRef)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			defer rc.Close()
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !bytes.Equal(got, blobData) {
				t.Fatalf("stored bytes = %q, want %q", got, blobData)
			}
		})
	}
}

func TestHandleBlobPutDedup(t *testing.T) {
	t.Parallel()

	blobs := openTestBlobStore(t)
	data := []byte("dedup me")

	var firstRef string
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/blobs", bytes.NewReader(data))
		rec := httptest.NewRecorder()
		handleBlobPut(rec, req, blobs, defaultMaxBodyBytes)
		if rec.Code != http.StatusOK {
			t.Fatalf("upload %d status = %d, want %d", i, rec.Code, http.StatusOK)
		}
		var resp blobUploadResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if i == 0 {
			firstRef = resp.BlobRef
		} else if resp.BlobRef != firstRef {
			t.Fatalf("second BlobRef = %q, want %q", resp.BlobRef, firstRef)
		}
	}
}

func TestBlobRouteMethodNotAllowed(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /blobs", func(w http.ResponseWriter, r *http.Request) {
		handleBlobPut(w, r, openTestBlobStore(t), defaultMaxBodyBytes)
	})
	mux.HandleFunc("/blobs", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	req := httptest.NewRequest(http.MethodGet, "/blobs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
