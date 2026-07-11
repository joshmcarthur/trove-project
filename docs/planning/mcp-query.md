---
title: MCP query server
parent: Planning
nav_order: 6
---

# MCP query server

**Status:** Supported\
**Milestone:** 3\
**Spec:** [Query §9](../spec.md#9-query-interface-mcp-over-rpc), [Build order §11.3](../spec.md#11-build-order--validation-plan)\
**Package:** `internal/query`

## Goal

Expose journal retrieval via MCP tools backed by an internal RPC API. This is the
historical failure point for prior art — validate conversational retrieval, not
just capture.

## Interfaces

Internal RPC:

```
search_events(query, type_prefix?, source?, time_range?) -> []Event
get_events_by_type(type, time_range) -> []Event
get_event(id) -> Event
summarize_range(time_range) -> Summary
```

MCP tools map 1:1 onto these methods.

## Implementation notes

- RPC layer and MCP server implemented in `internal/query`
- `modules/mcp-query` is a processor module; journal reads go through
  `CoreServices` query RPCs on the services broker
- `search_events`: FTS5 keyword search
- `summarize_range`: counts by type, notable events — avoids dumping raw rows
- MCP streamable HTTP is served by the `mcp-query` module at `POST /mcp` on
  `[http].listen`. See [MCP client setup](../getting-started/mcp-client.md).
- Built-in read tools are registered in-process; module tools from `[[mcp.tools]]`
  are aggregated via `CoreServices.CallMCPTool` — see [mcp-tools](./mcp-tools.md).

## Acceptance criteria

- [x] MCP `search_events` returns matching journal events
- [x] MCP `get_event` resolves by ULID
- [x] `summarize_range` returns aggregated summary for a time window
- [x] MCP `get_events_by_type` returns events matching exact type
- [x] OpenClaw or Cursor can connect as MCP client

## Dependencies

- **Blocks:** end-to-end v0 validation
- **Blocked by:** journal with FTS5

## Open questions

- Auth for MCP endpoint — [auth.md](./auth.md), [open-items.md](../open-items.md)
- `summarize_range` write-time vs query-time aggregation — [open-items.md](../open-items.md)
- HTTP gateway migration — [http-gateway.md](./http-gateway.md) (Supported)
