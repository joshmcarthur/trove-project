package types

import (
	"context"
	"fmt"

	"github.com/joshmcarthur/trove/internal/blob"
)

// StoreTypeDefinition canonicalizes td, stores bytes in blobs, and returns schema_ref.
func StoreTypeDefinition(ctx context.Context, blobs blob.Store, td TypeDefinition) (string, error) {
	canonical, err := CanonicalBytes(td)
	if err != nil {
		return "", err
	}
	ref, err := blobs.Put(ctx, bytesReader(canonical))
	if err != nil {
		return "", fmt.Errorf("types: store %s: %w", td.ID, err)
	}
	return ref, nil
}
