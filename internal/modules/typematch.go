package modules

import "github.com/joshmcarthur/trove/pkg/trovemodule"

// MatchType reports whether eventType matches any provides pattern.
func MatchType(patterns []string, eventType string) bool {
	return trovemodule.MatchType(patterns, eventType)
}
