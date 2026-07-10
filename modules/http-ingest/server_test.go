package main

import (
	"bytes"
	"context"
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
		errCh <- runHTTPServer(ctx, emit, config{Listen: addr, MaxBodyBytes: defaultMaxBodyBytes, Provides: defaultTestConfig().Provides})
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
