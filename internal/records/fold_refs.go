package records

import (
	"fmt"

	"github.com/joshmcarthur/trove/internal/journal"
	"github.com/joshmcarthur/trove/internal/references"
)

func foldReferences(e journal.Revision, prev []references.Reference, found bool) ([]references.Reference, error) {
	switch e.Operation {
	case journal.OpLink:
		if e.References == nil {
			return nil, fmt.Errorf("records: link: references are required")
		}
		add, err := references.ParseList(e.References)
		if err != nil {
			return nil, err
		}
		if len(add) == 0 {
			return nil, fmt.Errorf("records: link: references must not be empty")
		}
		base := prev
		if !found {
			base = []references.Reference{}
		}
		return references.Union(base, add), nil

	case journal.OpUnlink:
		if e.References == nil {
			return nil, fmt.Errorf("records: unlink: references are required")
		}
		remove, err := references.ParseList(e.References)
		if err != nil {
			return nil, err
		}
		if len(remove) == 0 {
			return nil, fmt.Errorf("records: unlink: references must not be empty")
		}
		if !found {
			return []references.Reference{}, nil
		}
		return references.Subtract(prev, remove), nil

	case journal.OpDelete:
		if found {
			return prev, nil
		}
		return []references.Reference{}, nil

	default: // apply
		if e.References == nil {
			if found {
				return prev, nil
			}
			return []references.Reference{}, nil
		}
		return references.ParseList(e.References)
	}
}
