package trovemodule

import (
	"github.com/joshmcarthur/trove/internal/types"
)

// MatchType reports whether eventType matches any pattern in patterns.
// Trove type URI patterns use trove://type/... wildcards; other patterns use
// exact match or path.Match globs.
func MatchType(patterns []string, eventType string) bool {
	return types.MatchAnyPattern(patterns, eventType)
}
