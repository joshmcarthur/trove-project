---
title: Type catalog
parent: Concepts
nav_order: 5
---

# Type catalog

Trove's **type catalog** is a local, in-process registry of event payload contracts.
Each registered type has a canonical `trove://` URI, a Trove Type Definition (TTD)
envelope, and an RFC 8927 JSON Type Definition (JTD) `definition` that validates
payloads at emit time.

See [spec §3](../spec.md#3-core-concepts) for how types relate to events.

## Why a catalog

Modules declare which types they may emit (`provides`) and subscribe to (`consumes`),
but routing alone does not define payload shape. The catalog closes that gap:

- Every validated emit path checks the event `type` against the catalog.
- Payloads are validated with a compiled JTD validator before append.
- The canonical TTD bytes are stored in the [blob store](./blobs.md); the journal
  records a `schema_ref` content hash on each validated event.

There is **no central schema registry service** — see [non-goals](../non-goals.md).
The catalog is built locally at startup from builtins, module manifests, and user
config.

## `trove://` type URIs

Event `type` values use the URI form:

```
trove://type/{path}/{version}
```

| Segment | Meaning |
|---------|---------|
| `path` | Slash-separated namespace, e.g. `note/created`, `mqtt/message/received` |
| `version` | Positive integer; bump when the payload contract changes |

Examples:

| URI | Typical use |
|-----|-------------|
| `trove://type/note/created/1` | Note capture with a `title` field |
| `trove://type/classify/pending/1` | Quick capture awaiting classification |
| `trove://type/mqtt/message/received/1` | MQTT message wrapped as an event |

Dotted shorthand (`note.created`) appears in manifest `[[types]]` declarations and
is converted to slash paths internally. Legacy dotted event types are no longer
used for new emits.

## TTD envelope

A **Trove Type Definition** is a JSON file (convention: `*.ttd.json`) with this
shape:

```json
{
  "$id": "trove://type/note/created/1",
  "title": "Note created",
  "description": "A journal entry created from a note capture.",
  "definition": {
    "properties": {
      "title": { "type": "string" }
    }
  },
  "status": "active"
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `$id` | yes | Must equal the canonical `trove://type/...` URI for this definition |
| `definition` | yes | RFC 8927 JTD schema for the event `payload` |
| `title`, `description` | no | Documentation for humans and MCP tools |
| `supersedes` | no | Previous `trove://` URI this version replaces |
| `status` | no | `active` (default) or `deprecated` |

The `definition` object follows [RFC 8927](https://www.rfc-editor.org/rfc/rfc8927.html)
(JTD). Trove compiles it with `github.com/jsontypedef/json-typedef-go` at catalog
build time. The envelope itself is validated in Go; see
`schemas/meta/type-definition-1.ttd.json` for a reference example.

Builtin types ship under `types/builtin/`. Modules and users contribute additional
TTD files referenced from `[[types]]` entries.

## `schema_ref`

When a payload passes catalog validation, the core stamps `schema_ref` on the event
before journal append. The value is a content-addressed blob reference (`sha256-<hex>`)
of the canonical TTD bytes.

```json
{
  "id": "01JXYZ...",
  "time": "2026-07-10T10:00:00+12:00",
  "type": "trove://type/note/created/1",
  "schema_ref": "sha256-abc123...",
  "source": "shortcuts",
  "payload": { "title": "hello" }
}
```

`schema_ref` lets readers retrieve the exact contract that was in force when the
event was written, even if a module is later removed or a user overrides the type
definition. Schema bytes already stored remain in the blob store; journal rows keep
their historical `schema_ref`.

## User vs module types

The catalog merges contributions in order:

1. **Builtins** — `types/builtin/*.ttd.json` shipped with Trove
2. **Modules** — `[[types]]` entries in each module `manifest.toml`
3. **User** — `[[types]]` entries in root `trove.toml`

Merge rules:

| Situation | Behaviour |
|-----------|-----------|
| User redefines a type already registered by a module | User wins; core logs a **warning** at startup |
| Two modules register the same URI with different TTD bytes | **Startup fails** — conflicting definitions |
| Same URI, identical canonical bytes | Silent dedup |

User overrides are intentional customisation (tighter validation, local fields).
Module conflicts are errors because Trove cannot pick a winner without guessing.

## Wildcards in `provides` and `consumes`

Wildcard patterns in module manifests control **routing and emit allowlists only**.
They do not register types in the catalog.

```toml
provides = [
  "trove://type/note/created/1",
  "trove://type/note/*",
]
```

For `trove://type/...` patterns:

- Exact URI match always works.
- A trailing `/*` matches the prefix and any sub-path
  (`trove://type/note/*` matches `trove://type/note/created/1`).
- Other segments use Go `path.Match` rules on the path after `trove://type/`.

An emit must satisfy **both**:

1. `event.type` matches a `provides` pattern for the emitting module, and
2. `event.type` is an **exact** catalog URI with a registered TTD.

So `provides = ["trove://type/note/*"]` allows any note sub-type the module is
permitted to emit, but each concrete type (e.g. `trove://type/note/created/1`)
must still have a `[[types]]` entry (from builtins, the module, or user config).

## Validation paths

Catalog validation runs on every emit boundary:

- Module `core.Emit`
- HTTP ingest (`POST /ingest/...`)
- Classify assignment (`POST /classify` with `target_type`)

Failures return an error before append (HTTP **400** for ingest). Successful emits
set both `type` and `schema_ref`.

## Declaring types

**Module manifest:**

```toml
[[types]]
name = "note.created"
version = 1
schema = "types/note.created.ttd.json"
```

**Root config (`trove.toml`):**

```toml
[[types]]
name = "note.created"
version = 1
schema = "/etc/trove/types/note.created.ttd.json"
```

The `name` + `version` must match the TTD `$id`. Paths are relative to the module
directory or absolute for user entries.

Legacy `[schemas]` JSON Schema entries in manifests are **not supported** — use
`[[types]]` and JTD instead. See [building modules](../building-modules.md).

## Implementation

**Status:** Supported — [planning/type-catalog.md](../planning/type-catalog.md)\
**Package:** `internal/types`
