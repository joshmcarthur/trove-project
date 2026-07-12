---
title: Query
parent: Concepts
nav_order: 7
---

# Query interface

The primary way to interact with Trove is conversational, via an MCP server. All
real logic lives in an internal RPC API; MCP tools are a thin wrapper.

See [spec §9](../spec.md#9-query-interface-mcp-over-rpc).

## Internal RPC

```
search_events(query, type_prefix?, source?, time_range?) -> []Event
get_events_by_type(type, time_range) -> []Event
get_event(id) -> Event
summarize_range(time_range) -> Summary
```

## MCP tools

Map 1:1 onto the RPC methods — narrow and typed, not raw SQL access.
`summarize_range` exists so "how was my week" does not dump thousands of rows
into context.

`search_events` will use FTS5 initially; semantic search via `sqlite-vec` when
embeddings land.

## Implementation

**Status:** Supported — [planning/mcp-query.md](../planning/mcp-query.md) (milestone 3)
