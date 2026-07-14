---
title: Query
parent: Concepts
nav_order: 7
---

# Query interface

The primary way to interact with Trove is conversational, via an MCP server. All
real logic lives in an internal RPC API; MCP tools are a thin wrapper over **record**
projections.

See [spec §9](../spec.md#9-query-interface-mcp-over-rpc).

## Internal RPC (records)

```
get_record(record_ref, version?) -> Record
search_records(query, type_prefix?, source?, time_range?, include_deleted?) -> []Record
list_incomplete_records(source?, limit?) -> []Record
```

Revision audit reads (`GetRevision`, `SearchRevisions`, `SummarizeRange`) are
available on module `Core` and host RPC — not exposed as MCP tools.

## MCP tools

Three built-in tools map onto the record RPC methods — narrow and typed, not raw
SQL access:

| Tool | Purpose |
|------|---------|
| `get_record` | Folded record by `record_ref` (optional `version`) |
| `search_records` | FTS5 keyword search over record bodies |
| `list_incomplete_records` | Records with `completeness = incomplete` |

`search_records` uses FTS5 initially; semantic search via `sqlite-vec` when
embeddings land.

Additional tools from loaded modules (e.g. classify) are aggregated at runtime.

## Implementation

**Status:** Supported — [planning/mcp-query.md](../planning/mcp-query.md) (milestone 3)
