---
title: Revisions
parent: Concepts
nav_order: 1
---

# Revisions

A **revision** is an append-only journal row: a record-scoped change with an **operation** (`apply` or `delete`), optional **payload** and **transforms**, and stable **record_ref** identity. Revisions are the audit log and replay source for [records](./records.md).

See [spec §3](../spec.md#3-core-concepts) and [planning/revision-rename.md](../planning/revision-rename.md).

## Shape

```json
{
  "id": "01JREV...",
  "time": "2026-07-10T10:00:00+12:00",
  "operation": "apply",
  "record_ref": "01JREC...",
  "type": "trove://type/note/quick/1",
  "schema_ref": "sha256-...",
  "source": "shortcuts",
  "payload": { "text": "hello" },
  "transforms": [],
  "blob_ref": null
}
```

| Field | Notes |
|-------|-------|
| `id` | ULID; sortable, unique |
| `operation` | `apply` or `delete` |
| `record_ref` | Stable record identity |
| `type` | `trove://type/...` when set |
| `completeness` | **Not on revisions** — see [records](./records.md) |

## Immutability

Revisions are never updated. Record changes are new revisions materialized into a new `version` on `record_heads`. Wipe projections and replay revisions to rebuild.

## Router

Processor modules declare `consumes` type URI patterns; the router dispatches matching **revisions** (not records). Modules guard `operation` in `Process` / `Handle` when they should ignore `delete`.

## Implementation

**Status:** Supported — [journal](./journal.md), [planning/journal.md](../planning/journal.md)
