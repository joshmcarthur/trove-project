---
title: Open items
nav_order: 9
---

# Open items

Decisions not yet made. Not blocking milestones 1–3, but affect later work.

From [spec §13](../spec.md#13-open-items-not-yet-decided):

| Item | Affects |
|------|---------|
| RPC protocol for remote/edge modules | [remote-modules](./planning/remote-modules.md) |
| Blob backend priority after filesystem | [blobs](./planning/blobs.md) |
| Embedding model (local ONNX vs API) | [embeddings](./planning/embeddings.md) |
| Default config file location (XDG vs `/etc/trove`) | [config](./planning/config.md) |
| `summarize_range`: pre-aggregate at write vs query time | [mcp-query](./planning/mcp-query.md), [processors-sinks](./planning/processors-sinks.md) |
| References on delete tombstone | [references](./planning/references.md) — retain or clear `references` on `delete` |
| Enricher idempotency keys | [references](./planning/references.md) — parallel processors on one capture |

## Resolved

| Item | Decision | Date |
|------|----------|------|
| Manifest subscription model (`consumes`) | Modules declare `consumes` / `provides` with glob patterns; see [modules](../concepts/modules.md) | 2026-07-11 |
| Auth for HTTP ingest and MCP | Gateway auth validators (`module.<name>.<id>`) | 2026-07-11 |
| Circular event-routing prevention | `DispatchContext.seen` skips modules already in the chain; startup graph warning | 2026-07-11 |
| Blob upload path | `PUT /blobs` on http-ingest via gateway | 2026-07-11 |
| Retention / pruning policy | `[journal].retention_days` | 2026-07-11 |
| HTTP gateway route registration | Single `[http].listen`, manifest `[[http.routes]]`, MCP on same port | 2026-07-11 |
| Content graph model | Records as nodes, revisions as audit log, references as `{ ref, rel? }` edges; see [references](./planning/references.md) | 2026-07-14 |
| URI grammar | `trove://record/`, `trove://revision/`, `trove://blob/`; external absolute URIs allowed | 2026-07-14 |
| Reference tuple | Bare `ref` URI; optional `rel`; attachments = blob refs; metadata in body | 2026-07-14 |
| `link` / `unlink` ops | Union add / subtract edges; no body merge; unlink omits `rel` → remove all edges for `ref` | 2026-07-14 |
| `apply` + `references` | Omit = unchanged; `[]` = clear; list = full replace | 2026-07-14 |
| Provenance split | `producer` host-stamped module id; `source` external origin; optional future `caused_by` | 2026-07-14 |
| Projections vs truth | Journal + blobs = source of truth; SQLite tables rebuildable | 2026-07-14 |
| Blob GC | Referenced blobs retained; unlink ≠ delete bytes; explicit pin deferred | 2026-07-14 |
| Module persistence rules | Same record → `apply`/`link`; new identity → new record + link; indexes → projections only | 2026-07-14 |
| Replay ordering prerequisite | Need `recorded_at` + per-record `sequence`; replay by sequence not event `time` | 2026-07-14 |
| Optimistic concurrency | Optional `expected_version` on full-copy `apply` (incl. references replace) | 2026-07-14 |

When you resolve an item, move the decision here with a date and link to the PR
that implemented it.
