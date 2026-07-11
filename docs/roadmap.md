---
title: Roadmap
nav_order: 1
---

# Roadmap

Living plan for Trove development. Update this page when a feature lands.

## Current focus

**Milestone 4 — two-week live test.** Milestones 1, 2, 2b, and 3 are complete.
Run the two-week live test with [iOS Shortcuts](./getting-started/ios-shortcuts.md)
capture (spec §11.4).

## Milestone sequence

Build order from spec §11:

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | SQLite journal + module-loading core + HTTP ingest | Supported |
| 2 | MQTT source module | Supported |
| 2b | Blob store (filesystem + HTTP upload) | Supported |
| 3 | Minimal MCP query server | Supported |
| 4 | Two-week live test | Planned |
| 5 | HA tap, embeddings, remote modules, processors/sinks | Later |

## Feature matrix

| Feature | Status | Spec | Planning | Go package |
|---------|--------|------|----------|------------|
| Go CLI | Supported | — | — | `cmd/trove` |
| SQLite journal | Supported | §4 | [journal](./planning/journal.md) | `internal/journal` |
| Generic HTTP ingest | Supported | §6, §11.1 | [http-ingest](./planning/http-ingest.md) | module + `internal/modules` |
| Module discovery (go-plugin) | Supported | §8 | [module-runtime](./planning/module-runtime.md) | `internal/modules` |
| MQTT source | Supported | §6, §11.2 | [mqtt-source](./planning/mqtt-source.md) | `modules/mqtt-source` |
| MCP query server | Supported | §9, §11.3 | [mcp-query](./planning/mcp-query.md) | `internal/query` + `modules/mcp-query` |
| HTTP gateway | Supported | §8, §9 | [http-gateway](./planning/http-gateway.md) | `internal/gateway` |
| MCP query module | Supported | §9 | [mcp-query](./planning/mcp-query.md) | `modules/mcp-query` |
| TOML config | Supported | §10 | [config](./planning/config.md) | `internal/config` |
| iOS Shortcuts capture recipes | Supported | §6 | [iOS Shortcuts](./getting-started/ios-shortcuts.md) | `examples/ios-shortcuts/` |
| Blob store (filesystem) | Supported | §5 | [blobs](./planning/blobs.md) | `internal/blob` |
| MCP tool registration | Supported | §8, §9 | [mcp-tools](./planning/mcp-tools.md) | `internal/modules` + `modules/mcp-query` |
| Deferred capture / classify | Supported | §3, §6 | [deferred-capture](./planning/deferred-capture.md) | `modules/capture-classifier` |
| Network auth (HTTP ingest + MCP) | Open | §13 | [auth](./planning/auth.md) | `internal/config` + modules |
| Journal retention / pruning | Open | §13 | [journal](./planning/journal.md) | `internal/journal` |
| HA WebSocket tap | Later | §6 | [ha-source](./planning/ha-source.md) | external module |
| Remote modules (Tailscale) | Later | §8 | [remote-modules](./planning/remote-modules.md) | `internal/modules` |
| Semantic search (sqlite-vec) | Later | §4 | [embeddings](./planning/embeddings.md) | `internal/journal` |
| Processors / sinks | Later | §7 | [processors-sinks](./planning/processors-sinks.md) | external modules |
| Alternative journal backends | Non-goal | §2, §12 | [non-goals](./non-goals.md) | — |
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
