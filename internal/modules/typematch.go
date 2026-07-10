package modules

import (
	"path"

	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

// MatchType reports whether eventType matches any provides pattern.
func MatchType(patterns []string, eventType string) bool {
	return trovemodule.MatchType(patterns, eventType)
}

// ResolveSchemaPattern returns the manifest schema key for eventType.
// Exact keys beat the longest matching wildcard pattern.
func ResolveSchemaPattern(schemaKeys []string, eventType string) (string, bool) {
	for _, key := range schemaKeys {
		if key == eventType {
			return key, true
		}
	}

	var best string
	for _, key := range schemaKeys {
		if key == eventType {
			continue
		}
		matched, err := path.Match(key, eventType)
		if err != nil || !matched {
			continue
		}
		if len(key) > len(best) {
			best = key
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}
