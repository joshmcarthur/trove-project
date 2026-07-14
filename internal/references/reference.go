package references

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/oklog/ulid"
)

// Reference is a directed edge { ref, rel? } on a record head.
type Reference struct {
	Ref string `json:"ref"`
	Rel string `json:"rel,omitempty"`
}

// ValidateRef checks ref against the Trove URI grammar for graph edges.
func ValidateRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("references: ref is required")
	}

	if strings.HasPrefix(ref, "trove://") {
		return validateTroveRef(ref)
	}

	u, err := url.Parse(ref)
	if err != nil {
		return fmt.Errorf("references: parse ref %q: %w", ref, err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("references: ref %q must be an absolute URI", ref)
	}
	if u.Host == "" && u.Opaque == "" && u.Path == "" {
		return fmt.Errorf("references: ref %q must be an absolute URI", ref)
	}
	return nil
}

func validateTroveRef(ref string) error {
	rest := strings.TrimPrefix(ref, "trove://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("references: invalid trove ref %q", ref)
	}

	switch parts[0] {
	case "record", "revision":
		if _, err := ulid.Parse(parts[1]); err != nil {
			return fmt.Errorf("references: %q id must be a ULID", ref)
		}
	case "blob":
		if !strings.HasPrefix(parts[1], "sha256-") {
			return fmt.Errorf("references: blob ref %q must use sha256- prefix", ref)
		}
		hex := strings.TrimPrefix(parts[1], "sha256-")
		if len(hex) == 0 || len(hex)%2 != 0 {
			return fmt.Errorf("references: blob ref %q has invalid hash", ref)
		}
		for _, c := range hex {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return fmt.Errorf("references: blob ref %q must use lowercase hex", ref)
			}
		}
	case "type":
		return fmt.Errorf("references: type URIs are not valid edge refs: %q", ref)
	default:
		return fmt.Errorf("references: unknown trove ref scheme %q", ref)
	}
	return nil
}

// ParseList unmarshals and validates a JSON array of references.
func ParseList(data json.RawMessage) ([]Reference, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("references: list is required")
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("references: list must be valid JSON")
	}

	var refs []Reference
	if err := json.Unmarshal(data, &refs); err != nil {
		return nil, fmt.Errorf("references: parse list: %w", err)
	}
	return ValidateList(refs)
}

// ValidateList validates each ref and dedupes by (ref, rel).
func ValidateList(refs []Reference) ([]Reference, error) {
	out := make([]Reference, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for i, ref := range refs {
		ref.Ref = strings.TrimSpace(ref.Ref)
		ref.Rel = strings.TrimSpace(ref.Rel)
		if err := ValidateRef(ref.Ref); err != nil {
			return nil, fmt.Errorf("references: item %d: %w", i, err)
		}
		key := ref.Ref + "\x00" + ref.Rel
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	return out, nil
}

// Union adds edges to base, deduping by (ref, rel).
func Union(base, add []Reference) []Reference {
	merged := append(append([]Reference{}, base...), add...)
	deduped, err := ValidateList(merged)
	if err != nil {
		// add/base were validated at append time; dedupe only.
		return Dedupe(merged)
	}
	return deduped
}

// Subtract removes edges from base. When rel is empty on a removal tuple, all
// edges with that ref are removed regardless of rel.
func Subtract(base, remove []Reference) []Reference {
	if len(remove) == 0 {
		return append([]Reference(nil), base...)
	}

	removeAllRef := make(map[string]struct{})
	removeExact := make(map[string]struct{})
	for _, r := range remove {
		r.Ref = strings.TrimSpace(r.Ref)
		r.Rel = strings.TrimSpace(r.Rel)
		if r.Rel == "" {
			removeAllRef[r.Ref] = struct{}{}
			continue
		}
		removeExact[r.Ref+"\x00"+r.Rel] = struct{}{}
	}

	out := make([]Reference, 0, len(base))
	for _, r := range base {
		if _, ok := removeAllRef[r.Ref]; ok {
			continue
		}
		if _, ok := removeExact[r.Ref+"\x00"+r.Rel]; ok {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Dedupe returns refs with duplicate (ref, rel) pairs removed, preserving order.
func Dedupe(refs []Reference) []Reference {
	seen := make(map[string]struct{}, len(refs))
	out := make([]Reference, 0, len(refs))
	for _, r := range refs {
		key := r.Ref + "\x00" + r.Rel
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}

// Marshal encodes refs as a JSON array.
func Marshal(refs []Reference) (json.RawMessage, error) {
	if len(refs) == 0 {
		return json.RawMessage(`[]`), nil
	}
	data, err := json.Marshal(refs)
	if err != nil {
		return nil, fmt.Errorf("references: marshal: %w", err)
	}
	return json.RawMessage(data), nil
}

// Unmarshal decodes a JSON array without validation.
func Unmarshal(data json.RawMessage) ([]Reference, error) {
	if len(data) == 0 {
		return []Reference{}, nil
	}
	var refs []Reference
	if err := json.Unmarshal(data, &refs); err != nil {
		return nil, fmt.Errorf("references: unmarshal: %w", err)
	}
	return refs, nil
}
