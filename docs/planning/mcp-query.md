---
title: MCP query server
parent: Planning
nav_order: 6
---

# MCP query server

**Status:** Planned\
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

- Implement RPC layer first; MCP server is thin wrapper
- `search_events`: FTS5 keyword search initially
- `summarize_range`: counts by type, notable events — avoid dumping raw rows
- Listen on `[mcp].listen` from config

## Acceptance criteria

- [ ] MCP `search_events` returns matching journal events
- [x] MCP `get_event` resolves by ULID
- [ ] `summarize_range` returns aggregated summary for a time window
- [ ] OpenClaw or Cursor can connect as MCP client

## Dependencies

- **Blocks:** end-to-end v0 validation
- **Blocked by:** journal with FTS5

## Open questions

- Auth for MCP endpoint — [open-items.md](../open-items.md)
- `summarize_range` write-time vs query-time aggregation — [open-items.md](../open-items.md)
