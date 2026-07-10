---
title: Open items
nav_order: 9
---

# Open items

Decisions not yet made. Not blocking milestone 1, but affect later work.

From [spec §13](../spec.md#13-open-items-not-yet-decided):

| Item | Affects |
|------|---------|
| RPC protocol for remote/edge modules | [remote-modules](./planning/remote-modules.md) |
| Blob backend priority after filesystem | [blobs](./planning/blobs.md) |
| Embedding model (local ONNX vs API) | [embeddings](./planning/embeddings.md) |
| Auth for HTTP ingest and MCP (Tailscale-only?) | [http-ingest](./planning/http-ingest.md), [mcp-query](./planning/mcp-query.md) |
| Retention / pruning policy | [journal](./planning/journal.md) |
| `summarize_range`: pre-aggregate at write vs query time | [mcp-query](./planning/mcp-query.md), [processors-sinks](./planning/processors-sinks.md) |

When you resolve an item, move the decision here with a date and link to the PR
that implemented it.
