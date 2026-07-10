package trovemodule

import "path"

// MatchType reports whether eventType matches any pattern in patterns.
// Each pattern is either an exact type string or a glob using path.Match rules.
func MatchType(patterns []string, eventType string) bool {
	for _, pattern := range patterns {
		if pattern == eventType {
			return true
		}
		matched, err := path.Match(pattern, eventType)
		if err == nil && matched {
			return true
		}
	}
	return false
}
