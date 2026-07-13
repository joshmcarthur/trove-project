package trovemodule

import (
	"context"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// RevisionAppender appends revisions to the Trove journal via Core.AppendRevision.
type RevisionAppender interface {
	AppendRevision(ctx context.Context, req *troverpc.AppendRevisionRequest) (*troverpc.AppendRevisionResponse, error)
}

// RevisionToAppendRequest builds an append request from a revision-shaped message.
func RevisionToAppendRequest(revision *troverpc.Revision) *troverpc.AppendRevisionRequest {
	if revision == nil {
		return &troverpc.AppendRevisionRequest{Operation: "apply"}
	}
	operation := revision.GetOperation()
	if operation == "" {
		operation = "apply"
	}
	return &troverpc.AppendRevisionRequest{
		Operation:  operation,
		RecordRef:  revision.GetRecordRef(),
		Type:       revision.GetType(),
		Time:       revision.GetTime(),
		Source:     revision.GetSource(),
		Payload:    revision.GetPayload(),
		Transforms: revision.GetTransforms(),
		BlobRef:    revision.GetBlobRef(),
	}
}

// AppendRevisionFromMessage appends using revision-shaped RPC fields.
func AppendRevisionFromMessage(ctx context.Context, a RevisionAppender, revision *troverpc.Revision) (*troverpc.AppendRevisionResponse, error) {
	return a.AppendRevision(ctx, RevisionToAppendRequest(revision))
}
