---
title: Planning
nav_order: 4
---

# Planning

Implementation briefs — one page per feature. Pick a page, implement in the
listed package, update the [roadmap](./roadmap.md).

## Milestone 1 — Journal + module core

| Page | Package |
|------|---------|
| [Journal (SQLite)](./journal.md) | `internal/journal` |
| [Module runtime](./module-runtime.md) | `internal/modules` |
| [HTTP ingest](./http-ingest.md) | `modules/http-ingest` (built-in) |
| [Config loader](./config.md) | `internal/config` |
| [Type catalog](./type-catalog.md) | `internal/types` |

## Milestone 2 — MQTT + blob storage

| Page | Package |
|------|---------|
| [MQTT source](./mqtt-source.md) | `modules/mqtt-source` |
| [Blob store](./blobs.md) | `internal/blob` |

## Milestone 3 — MCP query + gateway (complete)

| Page | Package |
|------|---------|
| [HTTP gateway](./http-gateway.md) | `internal/gateway` + modules |
| [MCP query server](./mcp-query.md) | `internal/query` + `modules/mcp-query` |
| [MCP tool registration](./mcp-tools.md) | `internal/modules` |
| [Network auth](./auth.md) | `internal/gateway` + `modules/http-gateway` |
| [Deferred capture](./deferred-capture.md) | `modules/capture-classifier` |
| [Telegram source](./telegram-source.md) | `modules/telegram-source` |
| [Processors and sinks](./processors-sinks.md) | `internal/modules` + modules |
| [CLI command registration](./cli-commands.md) | `internal/modules` + `cmd/trove` |
| [Type introspection](./type-introspection.md) | `modules/type-catalog` |

## Milestone 4 — Two-week live test (current)

Operational runbook — not a planning page:

- [Two-week live test](../getting-started/live-test.md)
- [iOS Shortcuts](../getting-started/ios-shortcuts.md)

## Open decisions

| Page | Notes |
|------|-------|
| [Config loader](./config.md) | Default config file location (XDG vs `/etc/trove`) |
| [MCP query server](./mcp-query.md) | `summarize_range` write-time vs query-time aggregation |
| [Blob store](./blobs.md) | Backend priority after filesystem |
| [Embeddings](./embeddings.md) | Embedding model (local ONNX vs API) |
| [Remote modules](./remote-modules.md) | Remote/edge RPC protocol shape |

## Milestone 5a — Records layer

| Page | Package |
|------|---------|
| [Records layer](./records.md) | `internal/records`, `internal/journal`, `internal/query` |

## Later

| Page | Package |
|------|---------|
| [HA WebSocket source](./ha-source.md) | external module |
| [Remote modules](./remote-modules.md) | `internal/modules` |
| [Embeddings / semantic search](./embeddings.md) | `internal/journal` |

## Template

Each planning page follows the same structure: goal, interfaces from spec,
implementation notes, acceptance criteria, dependencies, open questions.
