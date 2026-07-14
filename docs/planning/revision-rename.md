---
title: Revision rename
parent: Planning
nav_order: 15
---

# Revision rename

**Status:** Supported\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Journal §4](../spec.md#4-journal), [Query §9](../spec.md#9-query-interface-mcp-over-rpc)

## Goal

Adopt **Revision** as the canonical name for append-only journal rows. **Record** remains the folded projection; **Blob** remains attachments.

| Concept | Name | Owns |
|---------|------|------|
| Journal row | **Revision** | `id`, `time`, `source`, `payload`, `type`, `schema_ref`, `blob_ref`, `operation`, `record_ref`, `transforms` |
| Projection | **Record** | `record_ref`, `version`, `body`, `completeness`, `content_ref`, … |
| Attachment | **Blob** | content-addressed bytes |

**Completeness** is on the **record** head only. A revision *causes* completeness to change. `delete` is a tombstone revision; body is retained on the record head.

## Write path

`AppendRevision` appends a revision and materializes the record in one transaction (RPC, HTTP `POST /records`, module `Core`).

## Query surfaces

| Surface | API |
|---------|-----|
| **MCP** (conversational) | `get_record`, `search_records`, `list_incomplete_records` only |
| **Module `Core`** | `AppendRevision`, `GetRevision`, `SearchRevisions`, `GetRevisionsByType`, `SummarizeRange` |
| **Host-only RPC** | `GetRecord`, `SearchRecords`, `ListIncompleteRecords` (mcp-query, classify via `RecordProjection`) |

## SQLite schema

Fresh and migrated databases use:

- `revisions` (was `events`)
- `revisions_fts` (was `events_fts`)
- `record_revisions` (was `record_events`, column `revision_id`)
- `revision_dispatch` (was `event_dispatch`)

Legacy databases with `events` tables are migrated on `journal.Open`.

## Module API (`pkg/trovemodule`)

- `RevisionAppender.AppendRevision`
- `RevisionQuerier` (revision read RPCs)
- `RevisionProcessor` / `RevisionSink` (router dispatch)
- No `EmitRecord`, `Emitter`, or `RecordEmitter` shims

## Breaking changes

- MCP: removed `get_event`, `summarize_range` (use `search_records`)
- RPC: `EmitRecord` → `AppendRevision`; `GetEvent` → `GetRevision`; response `revision_id`
- HTTP `POST /records`: response field `revision_id` (was `event_id`)
- Proto: `message Revision` (was `Event`)

## Acceptance criteria

- [x] Canonical docs: Revision / Record / Blob triangle
- [x] Module SDK revision-only on `Core`
- [x] MCP records-only
- [x] SQLite `revisions` schema on fresh + migrated DBs
- [x] `make check` green
