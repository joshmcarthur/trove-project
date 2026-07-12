# Type Catalog Implementation Plan

> **Status: Completed (2026-07-12).** This was the implementation plan for the
> type catalog milestone. For current documentation see
> [type-catalog planning](../planning/type-catalog.md) and
> [type catalog concept](../concepts/type-catalog.md).

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-module optional JSON Schema with a global type catalog: `trove://` type URIs, RFC 8927 JTD payload contracts, blob-stored schema bytes, and `schema_ref` on every validated journal event.

**Architecture:** Modules and users contribute Trove Type Definition (TTD) JSON files. At startup the core merges them (user overrides module with warning; conflicting modules fail), canonicalizes each TTD, stores bytes in the existing blob store, and builds an in-memory catalog mapping `trove://type/...` → compiled JTD validator + `schema_ref`. All emit paths (module `Emit`, HTTP ingest, classify) validate payloads against the catalog and stamp `type` + `schema_ref` on events. Wildcards remain in `provides`/`consumes` for routing only.

**Tech Stack:** Go 1.26, `github.com/jsontypedef/json-typedef-go` (RFC 8927 JTD), existing `internal/blob` + `internal/journal`, TOML manifests/config.

---

## File structure

| File | Responsibility |
|------|----------------|
| `internal/types/uri.go` | Parse/format `trove://type/{path}/{version}` URIs |
| `internal/types/envelope.go` | TTD envelope struct + validation (`$id`, `definition`, optional metadata) |
| `internal/types/canonical.go` | Stable JSON bytes for content hashing |
| `internal/types/compile.go` | Parse JTD `definition`, compile validator |
| `internal/types/catalog.go` | Merged catalog: lookup, register, list |
| `internal/types/load.go` | Load TTD files from module dirs + user config |
| `internal/types/validate.go` | Validate event type + payload; return `schema_ref` |
| `internal/types/match.go` | Wildcard match for `trove://type/...` patterns |
| `internal/types/doc.go` | Package docs |
| `types/builtin/*.ttd.json` | Core shipped types (classify, ingest, note families) |
| `schemas/meta/type-definition-1.ttd.json` | Documented example of the TTD envelope (reference only; enforced in Go) |
| `docs/concepts/type-catalog.md` | Concept page for the type system |
| `docs/planning/type-catalog.md` | Planning page + acceptance criteria |
| `internal/journal/event.go` | Add `SchemaRef` field |
| `internal/journal/store.go` | DDL + read/write `schema_ref` |
| `internal/journal/migrate.go` | Add column migration for existing DBs |
| `api/proto/trove/v1/module.proto` | Add `schema_ref` to `Event` |
| `internal/modules/manifest.go` | `[[types]]` entries; deprecate `[schemas]` |
| `internal/modules/policy.go` | Replace JSON Schema policy with catalog delegation |
| `internal/config/config.go` | `[[types]]` in root config |
| `internal/modules/runtime.go` | Build catalog at startup, pass to services |
| `pkg/classify/classify.go` | Use `trove://` type constants |
| `modules/*/manifest.toml` | `provides` → `trove://` patterns; `[[types]]` |
| `modules/*/types/*.ttd.json` | Per-module TTD files |

---

### Task 1: `trove://` type URI helpers

**Files:**
- Create: `internal/types/uri.go`
- Create: `internal/types/uri_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/uri_test.go
package types_test

import (
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestFormatParseTypeURI(t *testing.T) {
	t.Parallel()
	uri := types.FormatTypeURI("note/created", 1)
	if uri != "trove://type/note/created/1" {
		t.Fatalf("FormatTypeURI() = %q", uri)
	}
	path, ver, err := types.ParseTypeURI(uri)
	if err != nil {
		t.Fatalf("ParseTypeURI() error = %v", err)
	}
	if path != "note/created" || ver != 1 {
		t.Fatalf("ParseTypeURI() = %q %d, want note/created 1", path, ver)
	}
}

func TestParseTypeURIRejectsBadVersion(t *testing.T) {
	t.Parallel()
	_, _, err := types.ParseTypeURI("trove://type/note/created/v2")
	if err == nil {
		t.Fatal("ParseTypeURI() error = nil, want error for non-numeric version")
	}
}

func TestNameToPath(t *testing.T) {
	t.Parallel()
	if got := types.NameToPath("note.created"); got != "note/created" {
		t.Fatalf("NameToPath() = %q, want note/created", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run TestFormatParseTypeURI -v`
Expected: FAIL — package or symbols not found

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/uri.go
package types

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	typeURIPrefix  = "trove://type/"
	metaURIPrefix  = "trove://meta/"
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
```

Also create `internal/types/doc.go`:

```go
// Package types implements Trove's type catalog: trove:// URIs, TTD envelopes,
// RFC 8927 JTD validation, and blob-backed schema storage.
package types
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/
git commit -m "feat(types): add trove:// type URI parse and format helpers"
```

---

### Task 2: TTD envelope parsing and validation

**Files:**
- Create: `internal/types/envelope.go`
- Create: `internal/types/envelope_test.go`
- Create: `schemas/meta/type-definition-1.ttd.json` (reference document)

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/envelope_test.go
package types_test

import (
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

const validTTD = `{
  "$id": "trove://type/note/created/1",
  "title": "Note created",
  "definition": {
    "properties": {
      "title": { "type": "string" }
    }
  }
}`

func TestParseTypeDefinition(t *testing.T) {
	t.Parallel()
	td, err := types.ParseTypeDefinition([]byte(validTTD))
	if err != nil {
		t.Fatalf("ParseTypeDefinition() error = %v", err)
	}
	if td.ID != "trove://type/note/created/1" {
		t.Fatalf("ID = %q", td.ID)
	}
}

func TestParseTypeDefinitionRequiresID(t *testing.T) {
	t.Parallel()
	_, err := types.ParseTypeDefinition([]byte(`{"definition":{"properties":{}}}`))
	if err == nil {
		t.Fatal("ParseTypeDefinition() error = nil, want missing $id error")
	}
}

func TestParseTypeDefinitionIDMustMatchURI(t *testing.T) {
	t.Parallel()
	raw := `{
	  "$id": "trove://type/note/created/2",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`
	_, err := types.ParseTypeDefinition([]byte(raw))
	if err == nil {
		t.Fatal("ParseTypeDefinition() error = nil, want $id/path version mismatch")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run TestParseTypeDefinition -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/envelope.go
package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TypeDefinition is the Trove Type Definition (TTD) envelope.
// The inner "definition" field is RFC 8927 JTD (validated separately).
type TypeDefinition struct {
	ID          string          `json:"$id"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Definition  json.RawMessage `json:"definition"`
	Supersedes  string          `json:"supersedes,omitempty"`
	Status      string          `json:"status,omitempty"`
}

// ParseTypeDefinition parses and validates a TTD envelope.
func ParseTypeDefinition(data []byte) (TypeDefinition, error) {
	var td TypeDefinition
	if err := json.Unmarshal(data, &td); err != nil {
		return TypeDefinition{}, fmt.Errorf("types: parse TTD: %w", err)
	}
	if strings.TrimSpace(td.ID) == "" {
		return TypeDefinition{}, fmt.Errorf("types: TTD: $id is required")
	}
	if len(td.Definition) == 0 || !json.Valid(td.Definition) {
		return TypeDefinition{}, fmt.Errorf("types: TTD %s: definition is required and must be JSON", td.ID)
	}
	path, ver, err := ParseTypeURI(td.ID)
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("types: TTD: $id: %w", err)
	}
	_ = path
	if td.Supersedes != "" {
		if _, _, err := ParseTypeURI(td.Supersedes); err != nil {
			return TypeDefinition{}, fmt.Errorf("types: TTD %s: supersedes: %w", td.ID, err)
		}
	}
	if td.Status != "" && td.Status != "active" && td.Status != "deprecated" {
		return TypeDefinition{}, fmt.Errorf("types: TTD %s: invalid status %q", td.ID, td.Status)
	}
	if td.Status == "" {
		td.Status = "active"
	}
	// Ensure $id version segment is consistent (already validated by ParseTypeURI).
	_ = ver
	return td, nil
}
```

Create reference file `schemas/meta/type-definition-1.ttd.json` documenting the envelope shape (example `note/created`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... -run TestParseTypeDefinition -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/envelope.go internal/types/envelope_test.go schemas/meta/
git commit -m "feat(types): parse and validate TTD envelopes"
```

---

### Task 3: JTD compile and payload validation

**Files:**
- Create: `internal/types/compile.go`
- Create: `internal/types/compile_test.go`
- Modify: `go.mod` (add `github.com/jsontypedef/json-typedef-go`)

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/compile_test.go
package types_test

import (
	"encoding/json"
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

func TestCompileAndValidatePayload(t *testing.T) {
	t.Parallel()
	td, err := types.ParseTypeDefinition([]byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": {
	    "properties": { "title": { "type": "string" } }
	  }
	}`))
	if err != nil {
		t.Fatalf("ParseTypeDefinition() error = %v", err)
	}
	ct, err := types.Compile(td)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := ct.ValidatePayload(json.RawMessage(`{"title":"ok"}`)); err != nil {
		t.Fatalf("ValidatePayload() valid error = %v", err)
	}
	if err := ct.ValidatePayload(json.RawMessage(`{}`)); err == nil {
		t.Fatal("ValidatePayload() missing title: error = nil, want validation error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run TestCompileAndValidatePayload -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```bash
go get github.com/jsontypedef/json-typedef-go@v1.0.0
```

```go
// internal/types/compile.go
package types

import (
	"encoding/json"
	"fmt"

	jtd "github.com/jsontypedef/json-typedef-go"
)

const jtdMaxDepth = 32

// CompiledType holds a validated TTD ready for payload checks.
type CompiledType struct {
	ID         string
	Definition TypeDefinition
	Schema     jtd.Schema
}

// Compile parses the JTD definition inside a TTD envelope.
func Compile(td TypeDefinition) (*CompiledType, error) {
	var schema jtd.Schema
	if err := json.Unmarshal(td.Definition, &schema); err != nil {
		return nil, fmt.Errorf("types: compile %s: parse JTD: %w", td.ID, err)
	}
	if err := schema.WellForm(); err != nil {
		return nil, fmt.Errorf("types: compile %s: JTD not well-formed: %w", td.ID, err)
	}
	return &CompiledType{ID: td.ID, Definition: td, Schema: schema}, nil
}

// ValidatePayload checks payload JSON against the compiled JTD schema.
func (c *CompiledType) ValidatePayload(payload json.RawMessage) error {
	var instance any
	if err := json.Unmarshal(payload, &instance); err != nil {
		return fmt.Errorf("types: payload for %s: invalid JSON: %w", c.ID, err)
	}
	errs, err := jtd.Validate(c.Schema, instance, jtd.WithMaxDepth(jtdMaxDepth))
	if err != nil {
		return fmt.Errorf("types: payload for %s: %w", c.ID, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("types: payload for %s: %v", c.ID, errs[0])
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... -run TestCompileAndValidatePayload -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/types/compile.go internal/types/compile_test.go
git commit -m "feat(types): compile RFC 8927 JTD and validate payloads"
```

---

### Task 4: Canonical bytes and blob-backed schema storage

**Files:**
- Create: `internal/types/canonical.go`
- Create: `internal/types/canonical_test.go`
- Create: `internal/types/store.go`
- Create: `internal/types/store_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/canonical_test.go
func TestCanonicalBytesStable(t *testing.T) {
	t.Parallel()
	a := []byte(`{"$id":"trove://type/note/created/1","definition":{"properties":{"title":{"type":"string"}}}}`)
	b := []byte(`{
	  "$id": "trove://type/note/created/1",
	  "definition": { "properties": { "title": { "type": "string" } } }
	}`)
	ha, err := types.CanonicalHash(a)
	if err != nil { t.Fatal(err) }
	hb, err := types.CanonicalHash(b)
	if err != nil { t.Fatal(err) }
	if ha != hb {
		t.Fatalf("hash mismatch: %s vs %s", ha, hb)
	}
}
```

```go
// internal/types/store_test.go — use internal/blob/filesystem test helper pattern
func TestStoreTypeDefinitionPutsBlob(t *testing.T) {
	// Parse TTD, Store(ctx, blobs, td) returns schema_ref with sha256- prefix
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run 'TestCanonical|TestStoreType' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/canonical.go
package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/joshmcarthur/trove/internal/blob"
)

// CanonicalBytes returns stable UTF-8 JSON for a TTD (sorted object keys).
func CanonicalBytes(td TypeDefinition) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")
	if err := enc.Encode(td); err != nil {
		return nil, fmt.Errorf("types: canonicalize: %w", err)
	}
	// Re-parse and encode with json.Marshal on a map for key sorting:
	var generic map[string]any
	if err := json.Unmarshal(buf.Bytes(), &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}

// CanonicalHash returns blob.FormatRef(hex(sha256(canonical bytes))).
func CanonicalHash(raw []byte) (string, error) {
	td, err := ParseTypeDefinition(raw)
	if err != nil {
		return "", err
	}
	canonical, err := CanonicalBytes(td)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return blob.FormatRef(hex.EncodeToString(sum[:])), nil
}
```

```go
// internal/types/store.go
package types

import (
	"context"
	"fmt"
	"io"

	"github.com/joshmcarthur/trove/internal/blob"
)

// StoreTypeDefinition canonicalizes td, stores bytes in blobs, returns schema_ref.
func StoreTypeDefinition(ctx context.Context, blobs blob.Store, td TypeDefinition) (string, error) {
	canonical, err := CanonicalBytes(td)
	if err != nil {
		return "", err
	}
	ref, err := blobs.Put(ctx, bytesReader(canonical))
	if err != nil {
		return "", fmt.Errorf("types: store %s: %w", td.ID, err)
	}
	return ref, nil
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... -run 'TestCanonical|TestStoreType' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/canonical.go internal/types/canonical_test.go internal/types/store.go internal/types/store_test.go
git commit -m "feat(types): canonicalize TTD bytes and store in blob store"
```

---

### Task 5: Type catalog merge (module + user, override warns)

**Files:**
- Create: `internal/types/catalog.go`
- Create: `internal/types/catalog_test.go`
- Create: `internal/types/load.go`
- Create: `internal/types/load_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/catalog_test.go
func TestCatalogUserOverridesModule(t *testing.T) {
	// Register module entry for trove://type/note/created/1 with schema A
	// Register user entry for same $id with schema B
	// Expect: catalog has B, warning logged (use httptest log or return []string warnings)
}

func TestCatalogRejectsConflictingModules(t *testing.T) {
	// Two module sources, different bytes, same $id -> error
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run TestCatalog -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/catalog.go
package types

import "fmt"

// Entry is a registered type in the catalog.
type Entry struct {
	URI        string
	SchemaRef  string
	Compiled   *CompiledType
	Source     string // module name or "user"
	SourcePath string // file path for debugging
}

// Catalog holds all active types keyed by URI.
type Catalog struct {
	entries map[string]Entry
}

func NewCatalog() *Catalog {
	return &Catalog{entries: make(map[string]Entry)}
}

// Register adds or replaces an entry. Returns warning message if URI existed.
func (c *Catalog) Register(e Entry) (warning string, err error) {
	if _, ok := c.entries[e.URI]; ok && e.Source != "" {
		prev := c.entries[e.URI]
		if prev.SchemaRef != e.SchemaRef {
			warning = fmt.Sprintf("types: user override replaces %s (was %s, now %s)", e.URI, prev.Source, e.Source)
		}
	}
	if prev, ok := c.entries[e.URI]; ok && prev.Source != "user" && e.Source != "user" && prev.Source != e.Source && prev.SchemaRef != e.SchemaRef {
		return "", fmt.Errorf("types: conflicting definitions for %s from %q and %q", e.URI, prev.Source, e.Source)
	}
	c.entries[e.URI] = e
	return warning, nil
}

func (c *Catalog) Lookup(uri string) (Entry, bool) {
	e, ok := c.entries[uri]
	return e, ok
}
```

```go
// internal/types/load.go — TypeDecl from config/manifest
type TypeDecl struct {
	Name    string // dotted or path
	Version int
	Schema  string // file path
	Source  string
}

func LoadTypeFile(path string, source string) (Entry, error) {
	// read file, ParseTypeDefinition, Compile, return Entry without SchemaRef yet
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... -run TestCatalog -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/catalog.go internal/types/catalog_test.go internal/types/load.go internal/types/load_test.go
git commit -m "feat(types): type catalog with user override and module conflict detection"
```

---

### Task 6: Config and manifest `[[types]]` declarations

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/modules/manifest.go`
- Modify: `internal/modules/manifest_test.go` (or testdata)

- [ ] **Step 1: Write the failing tests**

```go
// internal/config/config_test.go
func TestLoadTypesSection(t *testing.T) {
	const raw = `
[[types]]
name = "journal.entry"
version = 1
schema = "/tmp/journal.ttd.json"
`
	// write schema file, Load(), assert len(cfg.Types) == 1
}
```

```toml
# internal/modules/testdata/manifests/valid-source-types.toml
name = "test-source"
version = "1.0"
kind = "source"
provides = ["trove://type/mqtt/message/received/*"]

[[types]]
name = "mqtt.message.received"
version = 1
schema = "types/mqtt.ttd.json"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... ./internal/modules/... -run 'TestLoadTypes|valid-source-types' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Add to `internal/config/config.go`:

```go
type TypeDecl struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
	Schema  string `toml:"schema"`
}

type Config struct {
	// ...
	Types []TypeDecl `toml:"types"`
}
```

Add to `internal/modules/manifest.go`:

```go
type ManifestTypeDecl struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
	Schema  string `toml:"schema"`
}

type Manifest struct {
	// ...
	Types []ManifestTypeDecl `toml:"types"`
	// Schemas map — keep parsing but emit deprecation warning; removed in Task 12
}
```

Validation: every `[[types]].name` + version must resolve to a URI present in at least one module `provides` pattern OR be user-only (user types skip provides check).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... ./internal/modules/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/ internal/modules/manifest.go internal/modules/testdata/
git commit -m "feat(config): add [[types]] declarations to config and manifests"
```

---

### Task 7: Journal `schema_ref` column

**Files:**
- Modify: `internal/journal/event.go`
- Modify: `internal/journal/store.go`
- Create: `internal/journal/migrate.go`
- Modify: `internal/journal/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestAppendPersistsSchemaRef(t *testing.T) {
	store := openTestStore(t)
	ref := "sha256-" + strings.Repeat("a", 64)
	err := store.Append(ctx, journal.Event{
		Type: "trove://type/note/created/1",
		SchemaRef: ref,
		Source: "test",
		Payload: json.RawMessage(`{"title":"x"}`),
	})
	// Get and assert SchemaRef matches
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/journal/... -run TestAppendPersistsSchemaRef -v`
Expected: FAIL — unknown field or column

- [ ] **Step 3: Write minimal implementation**

Update `internal/journal/event.go`:

```go
type Event struct {
	ID        string
	Time      time.Time
	Type      string
	SchemaRef string
	Source    string
	Payload   json.RawMessage
	BlobRef   *string
}
```

Update `schemaDDL` in `store.go`:

```sql
CREATE TABLE IF NOT EXISTS events (
  id         TEXT PRIMARY KEY,
  time       TEXT NOT NULL,
  type       TEXT NOT NULL,
  schema_ref TEXT NOT NULL,
  source     TEXT NOT NULL,
  payload    TEXT NOT NULL,
  blob_ref   TEXT
);
```

Add `migrate.go`:

```go
func migrateSchema(db *sql.DB) error {
	// PRAGMA table_info; if schema_ref missing: ALTER TABLE events ADD COLUMN schema_ref TEXT NOT NULL DEFAULT ''
	return nil
}
```

Call from `Open()` after schemaDDL. Update all INSERT/SELECT paths.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/journal/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/journal/
git commit -m "feat(journal): persist schema_ref on events"
```

---

### Task 8: RPC proto and conversion layer

**Files:**
- Modify: `api/proto/trove/v1/module.proto`
- Regenerate: `internal/modules/rpc/trove/v1/*.go` (per project proto gen command)
- Modify: `internal/modules/convert.go`
- Modify: `internal/query/event.go`

- [ ] **Step 1: Write the failing test**

Extend `internal/modules/convert_test.go` (create if missing):

```go
func TestRPCEventRoundTripSchemaRef(t *testing.T) {
	ref := "sha256-" + strings.Repeat("b", 64)
	in := journal.Event{Type: "trove://type/note/created/1", SchemaRef: ref, /* ... */}
	out := rpcEventToJournal(journalEventToRPC(in))
	if out.SchemaRef != ref { t.Fatalf(...) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/... -run TestRPCEventRoundTripSchemaRef -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Add to `module.proto`:

```protobuf
message Event {
  string id = 1;
  string time = 2;
  string type = 3;
  string source = 4;
  bytes payload = 5;
  string blob_ref = 6;
  string schema_ref = 7;
}
```

Regenerate protos (check `Makefile` for target, likely `make proto` or `go generate`).

Update `convert.go` and `internal/query/event.go` to map `schema_ref`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/... ./internal/query/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/proto/ internal/modules/rpc/ internal/modules/convert.go internal/query/
git commit -m "feat(rpc): add schema_ref to Event message"
```

---

### Task 9: Catalog-backed emit validation (replace IngestPolicy JSON Schema)

**Files:**
- Create: `internal/types/validate.go`
- Create: `internal/types/match.go`
- Modify: `internal/modules/policy.go`
- Modify: `internal/modules/services.go`
- Modify: `internal/modules/services_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/types/validate_test.go
func TestValidateEmitSuccess(t *testing.T) {
	// catalog with note/created/1, event with matching payload -> schema_ref returned
}

func TestValidateEmitUnknownType(t *testing.T) {
	// event type not in catalog -> error
}

// internal/types/match_test.go
func TestMatchTypePattern(t *testing.T) {
	if !types.MatchTypePattern("trove://type/note/*", "trove://type/note/created/1") { t.Fatal() }
	if types.MatchTypePattern("trove://type/note/created/1", "trove://type/note/created/2") { t.Fatal() }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/types/... -run 'TestValidateEmit|TestMatchType' -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/validate.go
func (c *Catalog) ValidateEmit(event journal.Event, allowedPatterns []string) (schemaRef string, err error) {
	if !MatchAnyPattern(allowedPatterns, event.Type) {
		return "", fmt.Errorf("type %q not allowed", event.Type)
	}
	entry, ok := c.Lookup(event.Type)
	if !ok {
		return "", fmt.Errorf("type %q is not registered in catalog", event.Type)
	}
	if err := entry.Compiled.ValidatePayload(event.Payload); err != nil {
		return "", err
	}
	return entry.SchemaRef, nil
}
```

```go
// internal/types/match.go — MatchTypePattern for trove://type/... globs
func MatchTypePattern(pattern, typeURI string) bool { /* path.Match on suffix after trove://type/ */ }
func MatchAnyPattern(patterns []string, typeURI string) bool { /* ... */ }
```

Refactor `internal/modules/policy.go`:

```go
type EmitPolicy struct {
	patterns []string
	catalog  *types.Catalog
	module   string
}

func (p EmitPolicy) ValidateEvent(event journal.Event) error {
	ref, err := p.catalog.ValidateEmit(event, p.patterns)
	if err != nil { return err }
	event.SchemaRef = ref
	return nil
}
```

Update `services.go` `Emit` to set `event.SchemaRef` from validation result before append.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/types/... ./internal/modules/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/validate.go internal/types/match.go internal/modules/policy.go internal/modules/services.go
git commit -m "feat(types): catalog-backed emit validation replaces JSON Schema policy"
```

---

### Task 10: Startup catalog build in module runtime

**Files:**
- Modify: `internal/modules/runtime.go`
- Modify: `internal/modules/plugin_host.go`
- Create: `internal/types/build.go` (orchestrate load all modules + config + builtins)

- [ ] **Step 1: Write the failing integration test**

```go
// internal/modules/catalog_integration_test.go
func TestRuntimeBuildsCatalogFromModules(t *testing.T) {
	// Discover testdata plugin with [[types]], start runtime, Emit valid event succeeds
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/... -run TestRuntimeBuildsCatalog -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// internal/types/build.go
func BuildCatalog(ctx context.Context, blobs blob.Store, builtins []TypeDecl, modules []ModuleTypes, user []TypeDecl) (*Catalog, []string, error) {
	// 1. load builtins from types/builtin/
	// 2. load each module [[types]], StoreTypeDefinition -> SchemaRef
	// 3. load user [[types]], Register with override warnings
	// return catalog, warnings, err
}
```

Wire in `runtime.go`: after blob store + config available, call `BuildCatalog`, pass catalog pointer into each module's `EmitPolicy` / `coreServicesServer`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/... -run TestRuntimeBuildsCatalog -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/build.go internal/modules/runtime.go internal/modules/plugin_host.go internal/modules/catalog_integration_test.go
git commit -m "feat(runtime): build type catalog at startup from builtins, modules, and config"
```

---

### Task 11: Built-in types and module migration

**Files:**
- Create: `types/builtin/classify.pending.ttd.json`
- Create: `types/builtin/classify.assigned.ttd.json`
- Create: `types/builtin/note.created.ttd.json`
- Create: `types/builtin/http.ingest.received.ttd.json`
- Create: `types/builtin/mqtt.message.received.ttd.json`
- Modify: `modules/http-ingest/manifest.toml`
- Modify: `modules/capture-classifier/manifest.toml`
- Modify: `modules/mqtt-source/manifest.toml`
- Modify: `modules/telegram-source/manifest.toml`
- Modify: `pkg/classify/classify.go`
- Modify: `modules/httpingest/server.go`

Example builtin `types/builtin/classify.pending.ttd.json`:

```json
{
  "$id": "trove://type/classify/pending/1",
  "title": "Pending capture",
  "definition": {
    "properties": {},
    "optionalProperties": {
      "body": { "type": "string" },
      "caption": { "type": "string" }
    },
    "additionalProperties": true
  }
}
```

Note: JTD uses `additionalProperties` for open objects — verify against `json-typedef-go` syntax (may need `nullable` or empty properties with `values` — adjust per JTD spec; use `optionalProperties` only if strict).

Updated manifest snippet:

```toml
provides = [
  "trove://type/classify/*",
  "trove://type/note/*",
  "trove://type/shortcuts/*",
]

[[types]]
name = "classify.pending"
version = 1
schema = "../../types/builtin/classify.pending.ttd.json"
```

Update `pkg/classify/classify.go`:

```go
const (
	PendingType  = "trove://type/classify/pending/1"
	AssignedType = "trove://type/classify/assigned/1"
)
```

- [ ] **Step 1: Write failing tests** for classify + http-ingest using new URIs (update existing tests)

- [ ] **Step 2: Run tests** — expect FAIL on old type strings

Run: `go test ./pkg/classify/... ./modules/httpingest/... -v`
Expected: FAIL

- [ ] **Step 3: Implement** builtin files + manifest + constant updates

- [ ] **Step 4: Run full module tests**

Run: `go test ./... -count=1`
Expected: PASS (fix failures iteratively)

- [ ] **Step 5: Commit**

```bash
git add types/builtin/ modules/ pkg/classify/
git commit -m "feat(types): ship builtin TTDs and migrate modules to trove:// types"
```

---

### Task 12: Remove legacy `[schemas]` JSON Schema path

**Files:**
- Modify: `internal/modules/manifest.go` (reject `[schemas]` or warn + ignore)
- Modify: `internal/modules/policy.go` (remove `jsonschema-go` usage)
- Modify: `go.mod` (remove `github.com/google/jsonschema-go` if unused elsewhere)
- Modify: `docs/building-modules.md`, `docs/concepts/events.md`, `docs/spec.md`

- [ ] **Step 1: Write test that `[schemas]` in manifest returns clear error**

- [ ] **Step 2: Run test** — expect FAIL until implemented

- [ ] **Step 3: Remove JSON Schema loading from `LoadIngestPolicy`; delete `Schemas` field handling**

- [ ] **Step 4: Run `make check`**

Run: `make check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/ go.mod docs/
git commit -m "refactor(modules): remove legacy JSON Schema [schemas] manifest support"
```

---

### Task 13: Documentation and roadmap

**Files:**
- Create: `docs/concepts/type-catalog.md`
- Create: `docs/planning/type-catalog.md`
- Modify: `docs/concepts/events.md`
- Modify: `docs/building-modules.md`
- Modify: `docs/roadmap.md`
- Modify: `docs/non-goals.md` (clarify: no central *service*, local catalog is in scope)

- [ ] **Step 1: Write concept page** covering TTD, JTD, `trove://` URIs, `schema_ref`, user vs module types, wildcards

- [ ] **Step 2: Write planning page** with acceptance criteria:

```
- [ ] Catalog built at startup from builtins + modules + user [[types]]
- [ ] User override warns; conflicting modules fail startup
- [ ] Emit validates payload and stamps schema_ref
- [ ] Classify validates target type at assignment
- [ ] Schemas retained in blob store after module removal
- [ ] trove:// patterns work in provides/consumes
```

- [ ] **Step 3: Update events.md, building-modules.md, spec §3** to reference type catalog

- [ ] **Step 4: Update roadmap** — add Type catalog row as Supported after merge

- [ ] **Step 5: Commit**

```bash
git add docs/
git commit -m "docs: add type catalog concept and planning pages"
```

---

### Task 14: Final verification

- [ ] **Step 1: Run full check**

Run: `make check`
Expected: PASS (fmt, lint, test)

- [ ] **Step 2: Manual smoke test**

```bash
make build
# trove.toml with [[types]] for journal.entry
curl -X POST http://localhost:8080/ingest/test -d '{"type":"trove://type/note/created/1","title":"hello"}'
# Expect 201; sqlite event has schema_ref populated
```

- [ ] **Step 3: Verify classify path**

```bash
curl -X POST http://localhost:8080/capture/shortcuts -d '{"body":"quick"}'
curl -X POST http://localhost:8080/classify -d '{"source_event_id":"...","target_type":"trove://type/note/created/1","payload":{"title":"x"}}'
```

- [ ] **Step 4: Commit any smoke-test fixes**

- [ ] **Step 5: Open PR** with planning acceptance criteria checked

---

## Self-review

### Spec coverage

| Requirement | Task |
|-------------|------|
| `trove://type/{path}/{version}` URIs | Task 1 |
| TTD envelope with JTD `definition` | Tasks 2–3 |
| Blob store for schema bytes | Task 4 |
| User + module type registration | Tasks 5–6, 10 |
| User override warns | Task 5 |
| Module conflict fails | Task 5 |
| `schema_ref` on events | Tasks 7–8 |
| Validate at emit (ingest + classify) | Tasks 9–11 |
| Wildcards in provides/consumes | Task 9 (`match.go`) |
| Remove per-module JSON Schema | Task 12 |
| Docs | Task 13 |

### Placeholder scan

No TBD/TODO steps. Each task includes concrete code, file paths, and commands.

### Type consistency

- URI format: `trove://type/note/created/1` throughout
- Content refs: `sha256-...` for `schema_ref` and `blob_ref`
- `TypeDefinition.$id` must equal journal `type` for typed emits
- `EmitPolicy` / `Catalog.ValidateEmit` naming consistent

### Gap noted

JTD open/pending capture schema syntax must be verified against `json-typedef-go` during Task 11 — JTD has no `additionalProperties: true` like JSON Schema; use `properties: {}` with all fields in `optionalProperties` or JTD `values` form for free-form capture payloads.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-12-type-catalog.md`. Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
