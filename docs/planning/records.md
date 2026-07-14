---
title: Records layer
parent: Planning
nav_order: 14
---

# Records layer

**Status:** Supported\
**Milestone:** 5a — Records projection\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Journal §4](../spec.md#4-journal), [Query §9](../spec.md#9-query-interface-mcp-over-rpc)\
**Package:** `internal/records`, `internal/journal`, `internal/query`, `internal/modules`

## Goal

Introduce a **rebuildable record index** projected from an append-only revision log
(`apply` and `delete` operations). Records are the primary MCP query surface;
revisions remain the audit/rebuild source of truth.

See [records concept](../concepts/records.md) for vocabulary.

## Interfaces

### Write — `AppendRevision`

Appends a record-scoped journal revision and materializes the record projection in
one transaction (RPC, HTTP, module `Core`).

```
POST /records
```

```json
{
  "operation": "apply",
  "record_ref": "01JREC...",
  "type": "trove://type/note/quick/1",
  "source": "shortcuts",
  "payload": { "text": "hello" },
  "transforms": [{ "op": "add", "path": "/tags/-", "value": "photo" }],
  "blob_ref": "sha256-..."
}
```

| Operation | `record_ref` | Purpose |
|-----------|--------------|---------|
| `apply` | Optional (server allocates on create) | Create or amend record |
| `delete` | Required | Tombstone; body retained |

Response: `{ revision_id, record_ref, version, completeness, operation }`

gRPC: `AppendRevision(AppendRevisionRequest) returns (AppendRevisionResponse)` on `CoreServices`.

### Read — internal RPC / MCP

```
get_record(record_ref, version?) -> Record
search_records(query, filters...) -> []Record
list_incomplete_records(source?, limit?) -> []Record
```

MCP tools: `get_record`, `search_records`, `list_incomplete_records`.

## Data model

### Journal (`revisions`)

| Column | Notes |
|--------|-------|
| `operation` | `apply` \| `delete` |
| `record_ref` | Always persisted; server assigns on first apply |
| `type` | Record type when set (nullable) |
| `payload` | Merge-patch fragment (`apply`); `{}` for delete |
| `transforms` | RFC 6902 JSON Patch array (`apply` only) |
| `blob_ref` | Primary content (`apply` only) |

### Projection

- `record_heads` — current folded state per `record_ref`
- `record_revisions` — `(record_ref, version)` → `revision_id`
- `records_fts` — FTS5 on type, source, body

**Completeness:** `incomplete` \| `complete` \| `deleted`

### Fold order (`apply`)

1. Merge `payload` into previous body (RFC 7396)
2. Apply `transforms` (RFC 6902) against the body object
3. Set `type` / `content_ref` from revision fields
4. Validate folded body against TTD when type set
5. Write `record_heads` + FTS

**Delete:** set `completeness = deleted`; retain body, type, `content_ref`.

## Implementation notes

- Materializer in `internal/records`; same SQLite txn as revision append
- TTDs describe record **body**, not journal envelope
- Legacy `events` databases migrate to `revisions` on `journal.Open`
- `retention_days` cascades to record projection tables
- Processor routing: `consumes` on type; modules guard `operation` in `Process`/`Handle`
- Rebuild: `trove records rebuild` replays all revisions

## Acceptance criteria

### Core

- [x] `AppendRevision` `apply` without `record_ref` creates record at version 1
- [x] `AppendRevision` `apply` with `record_ref` increments version
- [x] `AppendRevision` `delete` sets completeness `deleted` and retains body
- [x] Fold order: merge payload → transforms → type/blob_ref → validate
- [x] Materialization in same txn as revision append
- [x] `trove records rebuild` reproduces identical `record_heads`

### Query

- [x] MCP `get_record`, `search_records`, `list_incomplete_records`
- [x] FTS on `records_fts`; deleted excluded from default search

### Sources

- [x] `POST /records` handles `apply` and `delete`
- [x] MQTT one-message-one-record
- [x] Telegram classify uses `record_ref`
- [x] capture-classifier module removed

### Retention

- [x] `PruneBefore` cascades to `record_heads`, `record_revisions`, and `records_fts`

## Dependencies

- **Blocks:** record-centric embeddings

## Open questions

| Item | Decision |
|------|----------|
| Delete body | Retain previous body |
| `search_records` body shape | TBD — full body vs summary |

## See also

- [Records concept](../concepts/records.md)
- [Revisions concept](../concepts/revisions.md)
- [Type catalog](../concepts/type-catalog.md)
- [Revision rename](./revision-rename.md)
