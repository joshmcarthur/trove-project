package modules

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testNoteCreatedURI        = "trove://type/note/created/1"
	testHTTPIngestReceivedURI = "trove://type/http/ingest/received/1"
)

func testCatalog(t *testing.T) *types.Catalog {
	t.Helper()
	c := types.NewCatalog()

	noteTD, err := types.ParseTypeDefinition([]byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": {
	    "properties": { "title": { "type": "string" } }
	  }
	}`))
	if err != nil {
		t.Fatalf("ParseTypeDefinition(note) error = %v", err)
	}
	noteCT, err := types.Compile(noteTD)
	if err != nil {
		t.Fatalf("Compile(note) error = %v", err)
	}
	if _, err := c.Register(types.Entry{
		URI:       testNoteCreatedURI,
		SchemaRef: "blob:note-created",
		Compiled:  noteCT,
		Source:    "test",
	}); err != nil {
		t.Fatalf("Register(note) error = %v", err)
	}

	httpTD := types.TypeDefinition{
		ID:         testHTTPIngestReceivedURI,
		Definition: json.RawMessage(`{}`),
	}
	httpCT, err := types.Compile(httpTD)
	if err != nil {
		t.Fatalf("Compile(http) error = %v", err)
	}
	if _, err := c.Register(types.Entry{
		URI:       testHTTPIngestReceivedURI,
		SchemaRef: "blob:http-ingest",
		Compiled:  httpCT,
		Source:    "test",
	}); err != nil {
		t.Fatalf("Register(http) error = %v", err)
	}

	return c
}

func registerPermissiveCatalogType(t *testing.T, c *types.Catalog, uri string) {
	t.Helper()
	if err := c.RegisterPermissive(uri); err != nil {
		t.Fatalf("RegisterPermissive(%q) error = %v", uri, err)
	}
}

func TestEmitPolicyValidateEvent(t *testing.T) {
	t.Parallel()

	catalog := testCatalog(t)
	policy, err := LoadIngestPolicy(Manifest{
		Name:     "test-source",
		Version:  "1.0",
		Kind:     KindSource,
		Provides: []string{"trove://type/note/*", "trove://type/http/ingest/received/1"},
	}, catalog, false)
	if err != nil {
		t.Fatalf("LoadIngestPolicy() error = %v", err)
	}

	tests := []struct {
		name    string
		event   journal.Event
		wantErr string
	}{
		{
			name: "allowed without schema requirement on pattern only type",
			event: journal.Event{
				Type:    testHTTPIngestReceivedURI,
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
		},
		{
			name: "allowed with valid schema",
			event: journal.Event{
				Type:    testNoteCreatedURI,
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
		},
		{
			name: "disallowed type",
			event: journal.Event{
				Type:    "trove://type/mqtt/foo/1",
				Source:  "shortcuts",
				Payload: json.RawMessage(`{"title":"ok"}`),
			},
			wantErr: `type "trove://type/mqtt/foo/1" not allowed`,
		},
		{
			name: "schema validation failure",
			event: journal.Event{
				Type:    testNoteCreatedURI,
				Source:  "shortcuts",
				Payload: json.RawMessage(`{}`),
			},
			wantErr: "payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			event := tt.event
			err := policy.ValidateEvent(&event)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateEvent() error = %v, want nil", err)
				}
				if event.SchemaRef == "" {
					t.Fatal("SchemaRef is empty after validation")
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

	catalog := types.NewCatalog()
	registerPermissiveCatalogType(t, catalog, "trove://type/allowed/event/1")

	policy, err := NewEmitPolicy([]string{"trove://type/allowed/event/1"}, catalog, "test-source")
	if err != nil {
		t.Fatalf("NewEmitPolicy() error = %v", err)
	}

	server := &coreServicesServer{
		journal: store,
		policy:  policy,
	}

	_, err = server.Emit(context.Background(), &troverpc.Event{
		Type:    "trove://type/denied/event/1",
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
	if !strings.Contains(st.Message(), `type "trove://type/denied/event/1" not allowed`) {
		t.Fatalf("Emit() message = %q, want type not allowed", st.Message())
	}
}

func TestLoadIngestPolicyRequiresCatalog(t *testing.T) {
	t.Parallel()

	_, err := LoadIngestPolicy(Manifest{
		Name:     "test-source",
		Version:  "1.0",
		Kind:     KindSource,
		Provides: []string{"trove://type/note/created/1"},
	}, nil, false)
	if err == nil || !strings.Contains(err.Error(), "catalog is required") {
		t.Fatalf("LoadIngestPolicy() error = %v, want catalog required", err)
	}
}
