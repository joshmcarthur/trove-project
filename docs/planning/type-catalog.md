---
title: Type catalog
parent: Planning
nav_order: 13
---

# Type catalog

**Status:** Supported\
**Milestone:** 4 — Two-week live test\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/types`, `internal/modules`, `internal/journal`

## Goal

Replace per-module optional JSON Schema with a global type catalog: `trove://` type
URIs, TTD envelopes with RFC 8927 JTD payload contracts, blob-stored schema bytes,
and `schema_ref` on every validated journal event.

## Interfaces

**Type URI** (`internal/types/uri.go`):

```
trove://type/{path}/{version}
```

**TTD file** (`*.ttd.json`):

```json
{
  "$id": "trove://type/note/created/1",
  "definition": { "properties": { "title": { "type": "string" } } }
}
```

**Manifest / config registration:**

```toml
[[types]]
name = "note.created"
version = 1
schema = "types/note.created.ttd.json"
```

**Journal field** (`internal/journal/event.go`):

```go
type Event struct {
    // ...
    Type      string
    SchemaRef string // sha256-<hex> of canonical TTD bytes
}
```

**Catalog API** (`internal/types/catalog.go`):

```go
func BuildCatalog(ctx context.Context, blobs blob.Store, builtinDir string,
    moduleTypes []ModuleTypesInput, userTypes []TypeDecl) (*Catalog, []string, error)

func (c *Catalog) ValidateEmit(event journal.Event, allowedPatterns []string) (schemaRef string, err error)
```

## Implementation notes

- Builtin TTDs in `types/builtin/`; reference envelope in `schemas/meta/type-definition-1.ttd.json`
- `BuildCatalog` at startup: load builtins → module `[[types]]` → user `[[types]]`
- Canonical TTD JSON stored via `internal/blob`; `schema_ref` is the returned content hash
- JTD compiled with `github.com/jsontypedef/json-typedef-go`
- `provides` / `consumes` wildcards (`trove://type/note/*`) for routing; catalog lookup requires exact URI
- User override of a module type logs a warning; conflicting module definitions fail startup
- `[schemas]` manifest section removed — manifests with `[schemas]` fail load with a clear error
- All first-party modules migrated to `trove://` types and `[[types]]`
- Concept page: [type catalog](../concepts/type-catalog.md)

## Acceptance criteria

- [x] Catalog built at startup from builtins + modules + user `[[types]]`
- [x] User override warns; conflicting modules fail startup
- [x] Emit validates payload and stamps `schema_ref`
- [x] Classify validates target type at assignment
- [x] Schemas retained in blob store after module removal
- [x] `trove://` patterns work in `provides`/`consumes`

## Dependencies

- **Blocks:** reliable payload validation for HTTP ingest, classify, and module emit
- **Blocked by:** blob store (`internal/blob`), journal `schema_ref` column

## Open questions

- None for v0 — versioning discipline is by URI version segment (`/1`, `/2`, …) and
  optional TTD `supersedes` metadata; no automatic migration machinery.
