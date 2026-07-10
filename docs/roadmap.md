---
title: Roadmap
nav_order: 1
---

# Roadmap

Living plan for Trove development. Update this page when a feature lands.

## Current focus

**Milestone 1 — Journal + module core.** Start with
[Journal (SQLite)](./planning/journal.md): the append-only event store everything
else depends on.

## Milestone sequence

Build order from spec §11:

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | SQLite journal + module-loading core + HTTP ingest | Planned |
| 2 | MQTT source module | Planned |
| 3 | Minimal MCP query server | Planned |
| 4 | Two-week live test | Planned |
| 5 | Blob store, HA tap, embeddings, remote modules, processors/sinks | Later |

## Feature matrix

| Feature | Status | Spec | Planning | Go package |
|---------|--------|------|----------|------------|
| Go CLI scaffold | Scaffold | — | — | `cmd/trove` |
| SQLite journal | Supported | §4 | [journal](./planning/journal.md) | `internal/journal` |
| Generic HTTP ingest | Supported | §6, §11.1 | [http-ingest](./planning/http-ingest.md) | module + `internal/modules` |
| Module discovery (go-plugin) | Supported | §8 | [module-runtime](./planning/module-runtime.md) | `internal/modules` |
| MQTT source | Planned | §6, §11.2 | [mqtt-source](./planning/mqtt-source.md) | external module |
| MCP query server | Planned | §9, §11.3 | [mcp-query](./planning/mcp-query.md) | `internal/query` |
| TOML config | Supported | §10 | [config](./planning/config.md) | `internal/config` |
| Blob store (filesystem) | Later | §5 | [blobs](./planning/blobs.md) | `internal/blob` |
| HA WebSocket tap | Later | §6 | [ha-source](./planning/ha-source.md) | external module |
| Remote modules (Tailscale) | Later | §8 | [remote-modules](./planning/remote-modules.md) | `internal/modules` |
| Semantic search (sqlite-vec) | Later | §4 | [embeddings](./planning/embeddings.md) | `internal/journal` |
| Processors / sinks | Later | §7 | [processors-sinks](./planning/processors-sinks.md) | external modules |
| Multi-journal sync | Non-goal | §12 | [non-goals](./non-goals.md) | — |
| WASM runtime | Non-goal | §12 | [non-goals](./non-goals.md) | — |

## How to use this

Pick the first **Planned** row whose dependencies are **Supported** (or
**Scaffold** where noted). Open its planning page, implement in the listed Go
package, then update status here and check off acceptance criteria on the
planning page in the same PR.

## Status vocabulary

| Status | Meaning |
|--------|---------|
| **Supported** | Implemented and usable |
| **Scaffold** | Package/CLI exists; no real behaviour yet |
| **Planned** | In scope for v0 validation (§11) |
| **Later** | Deferred until after the two-week live test |
| **Non-goal** | Deliberately out of scope (§12) |
| **Open** | Needs a decision before implementation (§13) |
