package types

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	typeURIPrefix = "trove://type/"
	metaURIPrefix = "trove://meta/"
)

// FormatTypeURI returns the canonical type URI for path and version.
// path uses slash segments, e.g. "note/created".
func FormatTypeURI(path string, version int) string {
	return fmt.Sprintf("%s%s/%d", typeURIPrefix, path, version)
}

// NameToPath converts dotted shorthand ("note.created") to path ("note/created").
func NameToPath(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), ".", "/")
}

// ParseTypeURI parses trove://type/{path}/{version}.
func ParseTypeURI(uri string) (path string, version int, err error) {
	if !strings.HasPrefix(uri, typeURIPrefix) {
		return "", 0, fmt.Errorf("types: invalid type URI %q: missing %q prefix", uri, typeURIPrefix)
	}
	rest := strings.TrimPrefix(uri, typeURIPrefix)
	i := strings.LastIndex(rest, "/")
	if i <= 0 || i == len(rest)-1 {
		return "", 0, fmt.Errorf("types: invalid type URI %q: missing version segment", uri)
	}
	path = rest[:i]
	verStr := rest[i+1:]
	version, err = strconv.Atoi(verStr)
	if err != nil || version < 1 {
		return "", 0, fmt.Errorf("types: invalid type URI %q: version must be positive integer", uri)
	}
	return path, version, nil
}
