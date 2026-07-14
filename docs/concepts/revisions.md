---
title: Revisions
parent: Concepts
nav_order: 1
---

# Revisions

A **revision** is an append-only journal row: a record-scoped change with an
**operation**, optional **payload** and **transforms**, and stable **record_ref**
identity. Revisions are the audit log and replay source for [records](./records.md).

Trove is a **personal content graph** backed by an append-only log: modules append
revisions to create and enrich [records](./records.md), link them with
[references](./records.md#references) (planned), and attach [blobs](./blobs.md).

See [spec §3](../spec.md#3-core-concepts) and [planning/revision-rename.md](../planning/revision-rename.md).

## Shape (supported today)

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
| `time` | Event time from source (not host commit time) |
| `operation` | `apply` or `delete` (see below) |
| `record_ref` | Stable record identity |
| `type` | `trove://type/...` when set |
| `source` | External origin (topic, device, app) |
| `completeness` | **Not on revisions** — see [records](./records.md) |

### Planned fields

| Field | Notes |
|-------|-------|
| `producer` | Host-stamped module identity (e.g. `module.http-ingest`); modules must not set |
| `references` | List of `{ ref, rel? }` edges — see [references planning](../planning/references.md) |
| `recorded_at` | Host commit time for deterministic replay |
| `caused_by` | Optional `trove://revision/...` derivation pointer |

## Operations

### Supported

| Operation | Purpose |
|-----------|---------|
| `apply` | Create or amend record body (merge payload, apply transforms) |
| `delete` | Tombstone; body retained on record head |

### Planned

| Operation | Purpose |
|-----------|---------|
| `link` | Add reference edges without merging body |
| `unlink` | Remove reference edges without merging body |
| `apply` + `references` | Replace or clear the record’s reference list while merging body |

Full semantics: [planning/references.md](../planning/references.md).

## Immutability

Revisions are never updated. Record changes are new revisions materialized into a new
`version` on `record_heads`. Wipe projections and replay revisions to rebuild.

## Router

Processor modules declare `consumes` type URI patterns; the router dispatches matching
**revisions** (not records). Modules guard `operation` in `Process` / `Handle` when
they should ignore `delete` or (later) `link` / `unlink`.

## Implementation

| Area | Status |
|------|--------|
| `apply`, `delete` | Supported — [journal](./journal.md), [planning/journal.md](../planning/journal.md) |
| `link`, `unlink`, `references` | Planned — [planning/references.md](../planning/references.md) |
| `producer`, `recorded_at`, `sequence` | Planned — [open-items](../open-items.md) |
