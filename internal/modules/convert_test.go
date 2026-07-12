package modules

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joshmcarthur/trove/internal/journal"
)

func TestRPCEventRoundTripSchemaRef(t *testing.T) {
	t.Parallel()

	ref := "sha256-" + strings.Repeat("b", 64)
	when := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	blobRef := "sha256:abc"
	in := journal.Event{
		ID:        "01JTEST",
		Time:      when,
		Type:      "trove://type/note/created/1",
		SchemaRef: ref,
		Source:    "test",
		Payload:   json.RawMessage(`{"title":"x"}`),
		BlobRef:   &blobRef,
	}

	out, err := rpcEventToJournal(journalEventToRPC(in))
	if err != nil {
		t.Fatalf("rpcEventToJournal() error = %v", err)
	}
	if out.SchemaRef != ref {
		t.Fatalf("SchemaRef = %q, want %q", out.SchemaRef, ref)
	}
}
