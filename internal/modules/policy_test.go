package modules

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIngestPolicyValidateEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schemas", "note.json")
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}
	schema := `{
  "type": "object",
  "required": ["title"],
  "properties": {
    "title": { "type": "string" }
  }
}`
	if err := os.WriteFile(schemaPath, []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	policy, err := LoadIngestPolicy(Manifest{
		Name:     "test-source",
		Version:  "1.0",
		Kind:     KindSource,
		Provides: []string{"note.*", "http.ingest.received"},
		Schemas: map[string]string{
			"note.*": "schemas/note.json",
		},
	}, dir, false)
	if err != nil {
		t.Fatalf("LoadIngestPolicy() error = %v", err)
	}

	tests := []struct {
		name    string
		event   journal.Event
		wantErr string
	}{
		{
			name: "allowed without schema",
			event: journal.Event{
				Type:    "http.ingest.received",
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
		},
		{
			name: "allowed with valid schema",
			event: journal.Event{
				Type:    "note.created",
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
		},
		{
			name: "disallowed type",
			event: journal.Event{
				Type:    "mqtt.foo",
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
			wantErr: `type "mqtt.foo" not allowed`,
		},
		{
			name: "schema validation failure",
			event: journal.Event{
				Type:    "note.created",
				Source:  "shortcuts",
				Payload: json.RawMessage(`{}`),
			},
			wantErr: "payload does not match schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := policy.ValidateEvent(tt.event)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateEvent() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateEvent() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreServicesEmitEnforcesPolicy(t *testing.T) {
	t.Parallel()

	store := openTestJournal(t)
	t.Cleanup(func() { _ = store.Close() })

	server := &coreServicesServer{
		journal: store,
		policy: IngestPolicy{
			patterns:   []string{"allowed.event"},
			moduleName: "test-source",
		},
	}

	_, err := server.Emit(context.Background(), &troverpc.Event{
		Type:    "denied.event",
		Source:  "src",
		Payload: []byte(`{"ok":true}`),
	})
	if err == nil {
		t.Fatal("Emit() error = nil, want InvalidArgument")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("Emit() code = %v, want InvalidArgument", err)
	}
	if !strings.Contains(st.Message(), `type "denied.event" not allowed`) {
		t.Fatalf("Emit() message = %q, want type not allowed", st.Message())
	}
}

func TestLoadIngestPolicyMissingSchemaFile(t *testing.T) {
	t.Parallel()

	_, err := LoadIngestPolicy(Manifest{
		Name:     "test-source",
		Version:  "1.0",
		Kind:     KindSource,
		Provides: []string{"note.created"},
		Schemas: map[string]string{
			"note.created": "schemas/missing.json",
		},
	}, t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "read schema") {
		t.Fatalf("LoadIngestPolicy() error = %v, want read schema error", err)
	}
}
