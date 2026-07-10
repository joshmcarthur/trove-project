package blob

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	refPrefix    = "sha256-"
	sha256HexLen = 64
)

// FormatRef returns the canonical ref for a SHA-256 digest hex string.
func FormatRef(hex string) string {
	return refPrefix + hex
}

// ParseRef validates ref and returns the lowercase hex digest.
func ParseRef(ref string) (hex string, err error) {
	if !strings.HasPrefix(ref, refPrefix) {
		return "", fmt.Errorf("blob: invalid ref %q: missing sha256- prefix", ref)
	}
	hex = ref[len(refPrefix):]
	if len(hex) != sha256HexLen {
		return "", fmt.Errorf("blob: invalid ref %q: want %d hex digits", ref, sha256HexLen)
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return "", fmt.Errorf("blob: invalid ref %q: non-lowercase hex", ref)
	}
	return hex, nil
}

func refPath(root, ref string) (string, error) {
	hex, err := ParseRef(ref)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, hex[0:2], hex[2:4], hex[4:]), nil
}

func refFromPath(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 3 {
		return "", false
	}
	if len(parts[0]) != 2 || len(parts[1]) != 2 {
		return "", false
	}
	hex := parts[0] + parts[1] + parts[2]
	if len(hex) != sha256HexLen {
		return "", false
	}
	ref := FormatRef(hex)
	if _, err := ParseRef(ref); err != nil {
		return "", false
	}
	return ref, true
}
