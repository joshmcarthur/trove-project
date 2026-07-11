package modules

import (
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

// DispatchContext carries routing metadata for a processing chain.
type DispatchContext struct {
	RootID string
	Seen   []string
}

func dispatchContextToProto(d DispatchContext) *troverpc.DispatchContext {
	if d.RootID == "" && len(d.Seen) == 0 {
		return nil
	}
	return &troverpc.DispatchContext{
		RootId: d.RootID,
		Seen:   append([]string(nil), d.Seen...),
	}
}

func withSeen(seen []string, moduleName string) []string {
	for _, name := range seen {
		if name == moduleName {
			return seen
		}
	}
	return append(append([]string(nil), seen...), moduleName)
}

func seenContains(seen []string, moduleName string) bool {
	for _, name := range seen {
		if name == moduleName {
			return true
		}
	}
	return false
}
