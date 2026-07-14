package modules

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/joshmcarthur/trove/internal/journal"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/internal/query"
	"github.com/joshmcarthur/trove/internal/records"
	"github.com/joshmcarthur/trove/internal/references"
)

// WriteResult is returned after a successful record write.
type WriteResult struct {
	EventID      string
	RecordRef    string
	Version      int
	Completeness string
	Operation    string
}

// WriteService appends record events and materializes projections in one transaction.
type WriteService struct {
	store *journal.Store
}

// NewWriteService returns a write service bound to store.
func NewWriteService(store *journal.Store) *WriteService {
	return &WriteService{store: store}
}

// Write persists a validated record event and materializes the projection.
func (s *WriteService) Write(ctx context.Context, event journal.Revision, policy *WritePolicy) (WriteResult, error) {
	if s == nil || s.store == nil {
		return WriteResult{}, fmt.Errorf("modules: write: journal is not configured")
	}
	if err := validateWriteEvent(&event); err != nil {
		return WriteResult{}, err
	}
	if policy != nil {
		if event.Operation == journal.OpDelete {
			if err := policy.ValidateDelete(&event); err != nil {
				return WriteResult{}, err
			}
		} else if err := policy.ValidateApply(&event); err != nil {
			return WriteResult{}, err
		}
	}
	if event.Producer == "" {
		if policy != nil {
			event.Producer = policy.Producer()
		} else {
			event.Producer = "core"
		}
	}

	var head records.Head
	var written journal.Revision
	err := s.store.AppendTransactional(ctx, event, func(ctx context.Context, tx *sql.Tx, e journal.Revision) error {
		written = e
		mat := records.NewMaterializer(tx)
		if _, matErr := mat.Apply(ctx, e); matErr != nil {
			return matErr
		}
		var ok bool
		var headErr error
		head, ok, headErr = mat.Head(ctx, e.RecordRef)
		if headErr != nil {
			return headErr
		}
		if !ok {
			return fmt.Errorf("modules: write: record %q missing after materialize", e.RecordRef)
		}
		return nil
	})
	if err != nil {
		return WriteResult{}, err
	}

	return WriteResult{
		EventID:      written.ID,
		RecordRef:    head.RecordRef,
		Version:      head.Version,
		Completeness: head.Completeness,
		Operation:    event.Operation,
	}, nil
}

// AppendRevisionFromRPC converts req, applies policy when set, and appends the record event.
func (s *WriteService) AppendRevisionFromRPC(ctx context.Context, req *troverpc.AppendRevisionRequest, policy WritePolicy) (*troverpc.AppendRevisionResponse, error) {
	event, err := rpcAppendRevisionRequestToJournal(req)
	if err != nil {
		return nil, err
	}
	result, err := s.Write(ctx, event, &policy)
	if err != nil {
		return nil, err
	}
	return appendRevisionResultToProto(result), nil
}

func validateWriteEvent(event *journal.Revision) error {
	if event == nil {
		return fmt.Errorf("modules: write: event is nil")
	}
	if event.Operation == "" {
		event.Operation = journal.OpApply
	}
	switch event.Operation {
	case journal.OpApply:
		if event.Source == "" {
			return fmt.Errorf("modules: write: source is required")
		}
		if len(event.Payload) == 0 {
			event.Payload = json.RawMessage(`{}`)
		}
		if !json.Valid(event.Payload) {
			return fmt.Errorf("modules: write: payload must be valid JSON")
		}
		if len(event.Transforms) > 0 && !json.Valid(event.Transforms) {
			return fmt.Errorf("modules: write: transforms must be valid JSON")
		}
		if event.References != nil {
			if _, err := references.ParseList(event.References); err != nil {
				return fmt.Errorf("modules: write: %w", err)
			}
		}
	case journal.OpDelete:
		if event.RecordRef == "" {
			return fmt.Errorf("modules: write: record_ref is required for delete")
		}
		if event.Source == "" {
			return fmt.Errorf("modules: write: source is required")
		}
		if len(event.Payload) == 0 {
			event.Payload = json.RawMessage(`{}`)
		} else if string(event.Payload) != "{}" {
			return fmt.Errorf("modules: write: delete payload must be {}")
		}
		if len(event.Transforms) > 0 && string(event.Transforms) != "[]" {
			return fmt.Errorf("modules: write: transforms are not allowed for delete")
		}
		if event.Type != "" {
			return fmt.Errorf("modules: write: type is not allowed for delete")
		}
		if event.BlobRef != nil {
			return fmt.Errorf("modules: write: blob_ref is not allowed for delete")
		}
		if event.References != nil {
			return fmt.Errorf("modules: write: references are not allowed for delete")
		}
	default:
		return fmt.Errorf("modules: write: operation must be %q or %q", journal.OpApply, journal.OpDelete)
	}
	return nil
}

func appendRevisionResultToProto(result WriteResult) *troverpc.AppendRevisionResponse {
	return &troverpc.AppendRevisionResponse{
		RevisionId:   result.EventID,
		RecordRef:    result.RecordRef,
		Version:      int32(result.Version), //nolint:gosec // G115: record version from materializer
		Completeness: result.Completeness,
		Operation:    result.Operation,
	}
}

func writerFromJournal(j journal.Journal) *WriteService {
	store, ok := j.(*journal.Store)
	if !ok {
		return nil
	}
	return NewWriteService(store)
}

func storeFromJournal(j journal.Journal) *journal.Store {
	store, ok := j.(*journal.Store)
	if !ok {
		return nil
	}
	return store
}

func recordsFromJournal(j journal.Journal) *query.RecordService {
	store, ok := j.(*journal.Store)
	if !ok {
		return nil
	}
	return &query.RecordService{DB: store.DB()}
}
