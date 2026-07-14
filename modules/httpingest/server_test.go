package httpingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockAppender struct {
	emits []*troverpc.AppendRevisionRequest
	err   error
}

func (m *mockAppender) AppendRevision(_ context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.emits = append(m.emits, req)
	return &troverpc.AppendRevisionResponse{RevisionId: "01JTEST", RecordRef: "01JREC", Version: 1, Operation: req.GetOperation()}, nil
}

type mockBlobPutter struct {
	refs map[string][]byte
}

func (m *mockBlobPutter) Put(_ context.Context, data []byte) (string, error) {
	if m.refs == nil {
		m.refs = make(map[string][]byte)
	}
	ref := "sha256-" + string(data)
	m.refs[ref] = append([]byte(nil), data...)
	return ref, nil
}

func defaultTestConfig() config {
	return config{
		MaxBodyBytes: defaultMaxBodyBytes,
		Provides:     []string{"trove://type/http/ingest/received/1", "trove://type/note/*", "trove://type/shortcut/*"},
	}
}

func recordsRequest(body string) *troverpc.HTTPRequest {
	return &troverpc.HTTPRequest{
		Method:         http.MethodPost,
		Path:           "/records",
		MatchedPattern: "/records",
		Body:           []byte(body),
	}
}

func ingestRequest(source, body string) *troverpc.HTTPRequest {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		wrapped, _ := json.Marshal(map[string]string{"source": source, "payload": body})
		return recordsRequest(string(wrapped))
	}
	sourceBytes, _ := json.Marshal(source)
	obj["source"] = sourceBytes
	out, _ := json.Marshal(obj)
	return recordsRequest(string(out))
}

func putBlobRequest(body string) *troverpc.HTTPRequest {
	return &troverpc.HTTPRequest{
		Method:         http.MethodPut,
		Path:           "/blobs",
		MatchedPattern: "/blobs",
		Body:           []byte(body),
	}
}

func TestHandlePutBlob(t *testing.T) {
	t.Parallel()

	blobData := []byte("photo-bytes")

	tests := []struct {
		name       string
		body       string
		maxBytes   int64
		wantStatus int
		wantRef    bool
	}{
		{
			name:       "valid body",
			body:       string(blobData),
			maxBytes:   defaultMaxBodyBytes,
			wantStatus: http.StatusCreated,
			wantRef:    true,
		},
		{
			name:       "empty body",
			body:       "",
			maxBytes:   defaultMaxBodyBytes,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversize body",
			body:       strings.Repeat("x", 2049),
			maxBytes:   2048,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			blobs := &mockBlobPutter{}
			cfg := defaultTestConfig()
			cfg.MaxBodyBytes = tt.maxBytes

			resp, err := handlePutBlob(context.Background(), blobs, cfg, putBlobRequest(tt.body))
			if err != nil {
				t.Fatalf("handlePutBlob() error = %v", err)
			}

			if int(resp.Status) != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %q", resp.Status, tt.wantStatus, resp.Body)
			}

			if !tt.wantRef {
				return
			}

			var putResp putBlobResponse
			if err := json.Unmarshal(resp.Body, &putResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !strings.HasPrefix(putResp.BlobRef, "sha256-") {
				t.Errorf("BlobRef = %q, want sha256- prefix", putResp.BlobRef)
			}
			if !bytes.Equal(blobs.refs[putResp.BlobRef], blobData) {
				t.Errorf("stored data = %q, want %q", blobs.refs[putResp.BlobRef], blobData)
			}
		})
	}
}

func TestHandlePutBlobDedup(t *testing.T) {
	t.Parallel()

	blobs := &mockBlobPutter{}
	cfg := defaultTestConfig()
	body := []byte("same-content")

	var ref1, ref2 string
	for i := 0; i < 2; i++ {
		resp, err := handlePutBlob(context.Background(), blobs, cfg, putBlobRequest(string(body)))
		if err != nil {
			t.Fatalf("handlePutBlob() error = %v", err)
		}
		if int(resp.Status) != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.Status, http.StatusCreated)
		}

		var putResp putBlobResponse
		if err := json.Unmarshal(resp.Body, &putResp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if i == 0 {
			ref1 = putResp.BlobRef
		} else {
			ref2 = putResp.BlobRef
		}
	}

	if ref1 != ref2 {
		t.Errorf("refs differ: %q vs %q", ref1, ref2)
	}
}

func TestPutBlobIngestRoundTrip(t *testing.T) {
	t.Parallel()

	blobs := &mockBlobPutter{}
	cfg := defaultTestConfig()
	emit := &mockAppender{}
	blobData := []byte("round-trip-bytes")

	putResp, err := handlePutBlob(context.Background(), blobs, cfg, putBlobRequest(string(blobData)))
	if err != nil {
		t.Fatalf("handlePutBlob() error = %v", err)
	}
	if int(putResp.Status) != http.StatusCreated {
		t.Fatalf("PUT status = %d, want %d", putResp.Status, http.StatusCreated)
	}

	var putBody putBlobResponse
	if err := json.Unmarshal(putResp.Body, &putBody); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}

	ingestBody := `{"source":"shortcuts","blob_ref":"` + putBody.BlobRef + `","title":"photo note","type":"trove://type/http/ingest/received/1"}`
	ingestResp, err := handleRecords(context.Background(), emit, cfg, ingestRequest("shortcuts", ingestBody))
	if err != nil {
		t.Fatalf("handleRecords() error = %v", err)
	}

	if int(ingestResp.Status) != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d; body = %q", ingestResp.Status, http.StatusCreated, ingestResp.Body)
	}
	if len(emit.emits) != 1 {
		t.Fatalf("EmitRecord calls = %d, want 1", len(emit.emits))
	}
	if emit.emits[0].BlobRef != putBody.BlobRef {
		t.Errorf("BlobRef = %q, want %q", emit.emits[0].BlobRef, putBody.BlobRef)
	}
	if !bytes.Equal(blobs.refs[putBody.BlobRef], blobData) {
		t.Errorf("stored data = %q, want %q", blobs.refs[putBody.BlobRef], blobData)
	}
}

func TestHandleRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		body       string
		emitErr    error
		wantStatus int
		wantEmit   bool
		checkEmit  func(t *testing.T, req *troverpc.AppendRevisionRequest)
	}{
		{
			name:       "valid object",
			source:     "shortcuts",
			body:       `{"title":"test"}`,
			wantStatus: http.StatusCreated,
			wantEmit:   true,
			checkEmit: func(t *testing.T, req *troverpc.AppendRevisionRequest) {
				t.Helper()
				if req.Source != "shortcuts" {
					t.Errorf("Source = %q, want shortcuts", req.Source)
				}
				if req.Type != "" {
					t.Errorf("Type = %q, want empty", req.Type)
				}
				if string(req.Payload) != `{"title":"test"}` {
					t.Errorf("Payload = %s, want %s", req.Payload, `{"title":"test"}`)
				}
			},
		},
		{
			name:       "custom type and time",
			source:     "shortcuts",
			body:       `{"type":"trove://type/note/created/1","time":"2026-07-10T12:00:00Z","title":"test"}`,
			wantStatus: http.StatusCreated,
			wantEmit:   true,
			checkEmit: func(t *testing.T, req *troverpc.AppendRevisionRequest) {
				t.Helper()
				if req.Type != "trove://type/note/created/1" {
					t.Errorf("Type = %q, want trove://type/note/created/1", req.Type)
				}
				if req.Time != "2026-07-10T12:00:00Z" {
					t.Errorf("Time = %q, want RFC3339 timestamp", req.Time)
				}
				if string(req.Payload) != `{"title":"test"}` {
					t.Errorf("Payload = %s, want %s", req.Payload, `{"title":"test"}`)
				}
			},
		},
		{
			name:       "array payload",
			source:     "shortcuts",
			body:       `["a","b"]`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "blob_ref field",
			source:     "shortcuts",
			body:       `{"blob_ref":"sha256-deadbeef","title":"photo note"}`,
			wantStatus: http.StatusCreated,
			wantEmit:   true,
			checkEmit: func(t *testing.T, req *troverpc.AppendRevisionRequest) {
				t.Helper()
				if req.BlobRef != "sha256-deadbeef" {
					t.Errorf("BlobRef = %q, want sha256-deadbeef", req.BlobRef)
				}
				if string(req.Payload) != `{"title":"photo note"}` {
					t.Errorf("Payload = %s, want %s", req.Payload, `{"title":"photo note"}`)
				}
			},
		},
		{
			name:       "invalid blob_ref field",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","blob_ref":123}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			source:     "shortcuts",
			body:       `{not-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			source:     "shortcuts",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid type field",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","type":123}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid time field",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","time":"not-a-time"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "disallowed type",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","type":"trove://type/mqtt/sensor/temp/1","title":"test"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "emit invalid argument",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","type":"trove://type/http/ingest/received/1","title":"test"}`,
			emitErr:    status.Error(codes.InvalidArgument, "payload does not match schema for type \"trove://type/http/ingest/received/1\": missing title"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "emit failure",
			source:     "shortcuts",
			body:       `{"source":"shortcuts","type":"trove://type/http/ingest/received/1","title":"test"}`,
			emitErr:    errors.New("emit failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			emit := &mockAppender{err: tt.emitErr}
			var req *troverpc.HTTPRequest
			if tt.wantStatus == http.StatusCreated {
				req = ingestRequest(tt.source, tt.body)
			} else {
				req = recordsRequest(tt.body)
			}
			resp, err := handleRecords(context.Background(), emit, defaultTestConfig(), req)
			if err != nil {
				t.Fatalf("handleRecords() error = %v", err)
			}

			if int(resp.Status) != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %q", resp.Status, tt.wantStatus, resp.Body)
			}

			if tt.wantEmit {
				if len(emit.emits) != 1 {
					t.Fatalf("EmitRecord calls = %d, want 1", len(emit.emits))
				}
				if tt.checkEmit != nil {
					tt.checkEmit(t, emit.emits[0])
				}
			} else if len(emit.emits) != 0 {
				t.Fatalf("EmitRecord calls = %d, want 0", len(emit.emits))
			}
		})
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
	if cfg.MaxBodyBytes != defaultMaxBodyBytes {
		t.Errorf("MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, defaultMaxBodyBytes)
	}
}

func TestLoadConfigCustomMaxBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := `max_body_bytes = 2048
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

var _ trovemodule.RevisionAppender = (*mockAppender)(nil)
var _ trovemodule.BlobPutter = (*mockBlobPutter)(nil)
