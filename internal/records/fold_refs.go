package records

import (
	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/references"
)

func foldReferences(e journal.Revision, prev []references.Reference, found bool) ([]references.Reference, error) {
	if e.Operation == journal.OpDelete {
		if found {
			return prev, nil
		}
		return []references.Reference{}, nil
	}

	if e.References == nil {
		if found {
			return prev, nil
		}
		return []references.Reference{}, nil
	}
	return references.ParseList(e.References)
}
