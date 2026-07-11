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
| Blob upload path | **Resolved:** `PUT /blobs` on http-ingest — see [blobs](./planning/blobs.md), [http-ingest](./planning/http-ingest.md) |
| Embedding model (local ONNX vs API) | [embeddings](./planning/embeddings.md) |
| Auth for HTTP ingest and MCP | [auth](./planning/auth.md), [http-ingest](./planning/http-ingest.md), [mcp-query](./planning/mcp-query.md) |
| Retention / pruning policy | [journal](./planning/journal.md) |
| Default config file location (XDG vs `/etc/trove`) | [config](./planning/config.md) |
| `summarize_range`: pre-aggregate at write vs query time | [mcp-query](./planning/mcp-query.md), [processors-sinks](./planning/processors-sinks.md) |
| HTTP gateway: route registration, MCP migration, streaming RPC | [http-gateway](./planning/http-gateway.md) |

## Resolved

| Item | Decision | Date |
|------|----------|------|
| Manifest subscription model (`consumes`) | Modules declare `consumes` / `provides` with glob patterns; see [modules](../concepts/modules.md) | 2026-07-11 |
| Circular event-routing prevention | `DispatchContext.seen` skips modules already in the chain; startup graph warning | 2026-07-11 |

When you resolve an item, move the decision here with a date and link to the PR
that implemented it.
