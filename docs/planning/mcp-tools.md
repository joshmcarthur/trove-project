---
title: MCP tools
parent: Planning
nav_order: 13
---

# MCP tool registration

**Status:** Supported\
**Milestone:** 4\
**Spec:** [Query §9](../spec.md#9-query-interface-mcp-over-rpc), [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/modules`, `internal/query`, `modules/mcp-query`

## Goal

Let modules contribute MCP tools the same way they contribute HTTP routes.
`mcp-query` remains the single `POST /mcp` entrypoint and aggregates built-in
read tools with module-provided tools.

## Manifest

```toml
[[mcp.tools]]
name = "classify_event"
description = "Classify a pending capture into a typed event"
```

- Tool names must be unique across all discovered modules (startup fails on duplicates)
- Module subprocess implements `trovemodule.MCPToolHandler`

## RPC surface

- `MCPModule.CallTool` — implemented by tool-providing modules
- `CoreServices.ListMCPTools` — returns collected manifest tools
- `CoreServices.CallMCPTool` — host routes to the owning module via `MCPRegistry`

## Acceptance criteria

- [x] `[[mcp.tools]]` parsed and validated from manifest
- [x] Duplicate tool names rejected at startup
- [x] `mcp-query` registers built-in read tools plus module tools
- [x] `CallMCPTool` routes to the correct module subprocess

## See also

- [MCP query server](./mcp-query.md)
- [Deferred capture](./deferred-capture.md)
