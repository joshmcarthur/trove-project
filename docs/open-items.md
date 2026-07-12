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

## Resolved

| Item | Decision | Date |
|------|----------|------|
| Manifest subscription model (`consumes`) | Modules declare `consumes` / `provides` with glob patterns; see [modules](../concepts/modules.md) | 2026-07-11 |
| Auth for HTTP ingest and MCP | Gateway auth validators (`module.<name>.<id>`) | 2026-07-11 |
| Circular event-routing prevention | `DispatchContext.seen` skips modules already in the chain; startup graph warning | 2026-07-11 |
| Blob upload path | `PUT /blobs` on http-ingest via gateway | 2026-07-11 |
| Retention / pruning policy | `[journal].retention_days` | 2026-07-11 |
| HTTP gateway route registration | Single `[http].listen`, manifest `[[http.routes]]`, MCP on same port | 2026-07-11 |

When you resolve an item, move the decision here with a date and link to the PR
that implemented it.
