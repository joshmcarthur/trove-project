---
title: Records layer
parent: Planning
nav_order: 14
---

# Records layer

**Status:** Planned\
**Milestone:** 5a — Records projection\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Journal §4](../spec.md#4-journal), [Query §9](../spec.md#9-query-interface-mcp-over-rpc)\
**Package:** `internal/records`, `internal/journal`, `internal/query`, `internal/modules`

## Goal

Introduce a **rebuildable record index** projected from an append-only event log
(`apply` and `delete` operations). Records become the primary MCP query surface;
events remain the audit/rebuild source of truth.

See [records concept](../concepts/records.md) for vocabulary.

## Interfaces

### Write — `RecordWrite`

Single code path for all mutations (RPC, HTTP, module `Core`).

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
  "transforms": [{ "op": "add", "path": "/members/-", "value": "01JREC-photo" }],
  "blob_ref": "sha256-..."
}
```

| Operation | `record_ref` | Purpose |
|-----------|--------------|---------|
| `apply` | Optional (server allocates on create) | Create or amend record |
| `delete` | Required | Tombstone; body retained |

Response: `{ event_id, record_ref, version, completeness, operation }`

gRPC: `RecordWrite(WriteRequest) returns (WriteResponse)` on `CoreServices`.

### Read — internal RPC / MCP

```
get_record(record_ref, version?) -> Record
list_records(filters...) -> []Record
search_records(query, filters...) -> []Record
list_incomplete_records(source?, limit?) -> []Record
```

MCP tools: `get_record`, `search_records`, `list_incomplete_records`.

`get_event` remains for audit.

## Data model

### Journal (`events`)

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
- `record_events` — `(record_ref, version)` → `event_id`
- `records_fts` — FTS5 on type, source, body

**Completeness:** `incomplete` \| `complete` \| `deleted`

### Fold order (`apply`)

1. Merge `payload` into previous body (RFC 7396)
2. Apply `transforms` (RFC 6902, body-rooted sandbox)
3. Set `type` / `content_ref` from event fields
4. Validate folded body against TTD when type set
5. Write `record_heads` + FTS

**Delete:** set `completeness = deleted`; retain body, type, `content_ref`.

## Implementation notes

- Materializer in `internal/records`; same SQLite txn as event append
- TTDs describe record **body**, not journal envelope
- No migration from `classify.pending` — wipe dev journals during rollout
- `retention_days` cascades to record projection tables
- Processor routing: `consumes_operations` (default `["apply"]`) + `consumes` on type
- Albums: `trove://type/album/created/1` with `body.members[]` of `record_ref`
- Rebuild: `trove records rebuild` replays all events

### PR delivery

See epic branch `cursor/records-layer-71b9` — 13 stacked PRs; major version bump
on merge (`feat!:`).

## Acceptance criteria

### Core

- [ ] `RecordWrite` `apply` without `record_ref` creates record at version 1
- [ ] `RecordWrite` `apply` with `record_ref` increments version
- [ ] `RecordWrite` `delete` sets completeness `deleted` and retains body
- [ ] Fold order: merge payload → transforms → type/blob_ref → validate
- [ ] Materialization in same txn as event append
- [ ] `trove records rebuild` reproduces identical `record_heads`

### Query

- [ ] MCP `get_record`, `search_records`, `list_incomplete_records`
- [ ] FTS on `records_fts`; deleted excluded from default search
- [ ] `get_event` available for audit

### Sources

- [ ] `POST /records` handles `apply` and `delete`
- [ ] MQTT one-message-one-record
- [ ] Telegram classify uses `record_ref`
- [ ] capture-classifier module removed

### Albums

- [ ] Album create + member add/remove via transforms

## Dependencies

- **Blocked by:** none (greenfield during active development)
- **Blocks:** record-centric embeddings, album workflows
- **Supersedes:** [deferred-capture](./deferred-capture.md)

## Open questions

| Item | Decision |
|------|----------|
| Delete body | Retain previous body |
| `search_records` body shape | TBD — full body vs summary |

## See also

- [Records concept](../concepts/records.md)
- [Events concept](../concepts/events.md)
- [Type catalog](../concepts/type-catalog.md)
