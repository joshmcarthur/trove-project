package types

import (
	"path"
	"strings"
)

// MatchTypePattern reports whether typeURI matches pattern for trove://type/ URIs.
// The trove://type/ prefix is stripped from both values. A trailing /* on the
// pattern matches the prefix and any sub-path; otherwise path.Match is used.
func MatchTypePattern(pattern, typeURI string) bool {
	if pattern == typeURI {
		return true
	}
	patRest := strings.TrimPrefix(pattern, typeURIPrefix)
	typeRest := strings.TrimPrefix(typeURI, typeURIPrefix)
	if patRest == pattern || typeRest == typeURI {
		return false
	}
	if strings.HasSuffix(patRest, "/*") {
		prefix := strings.TrimSuffix(patRest, "/*")
		if typeRest == prefix {
			return true
		}
		return strings.HasPrefix(typeRest, prefix+"/")
	}
	matched, err := path.Match(patRest, typeRest)
	return err == nil && matched
}

// MatchAnyPattern reports whether typeURI matches any pattern. Trove type URI
// patterns use MatchTypePattern; other patterns use exact match or path.Match.
func MatchAnyPattern(patterns []string, typeURI string) bool {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, typeURIPrefix) || strings.HasPrefix(typeURI, typeURIPrefix) {
			if MatchTypePattern(pattern, typeURI) {
				return true
			}
		}
		if pattern == typeURI {
			return true
		}
		matched, err := path.Match(pattern, typeURI)
		if err == nil && matched {
			return true
		}
	}
	return false
}
