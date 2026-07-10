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
| [HTTP ingest](./http-ingest.md) | external module |
| [Config loader](./config.md) | `internal/config` |

## Milestone 2 — MQTT

| Page | Package |
|------|---------|
| [MQTT source](./mqtt-source.md) | external module |

## Milestone 3 — MCP query

| Page | Package |
|------|---------|
| [MCP query server](./mcp-query.md) | `internal/query` |

## Later

| Page | Package |
|------|---------|
| [Blob store](./blobs.md) | `internal/blob` |
| [HA WebSocket source](./ha-source.md) | external module |
| [Remote modules](./remote-modules.md) | `internal/modules` |
| [Embeddings / semantic search](./embeddings.md) | `internal/journal` |
| [Processors and sinks](./processors-sinks.md) | external modules |

## Template

Each planning page follows the same structure: goal, interfaces from spec,
implementation notes, acceptance criteria, dependencies, open questions.
