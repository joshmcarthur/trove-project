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

Expose **record** retrieval via MCP tools backed by an internal RPC API. This is the
historical failure point for prior art — validate conversational retrieval, not
just capture.

## Interfaces

Internal RPC (records):

```
get_record(record_ref, version?) -> Record
search_records(query, type_prefix?, source?, time_range?, include_deleted?) -> []Record
list_incomplete_records(source?, limit?) -> []Record
```

MCP tools map 1:1 onto these methods. Revision audit reads are module `Core` RPC only.

## Implementation notes

- RPC layer and MCP server implemented in `internal/query`
- `modules/mcp-query` is a processor module; record reads go through host
  `RecordProjection` RPCs on the services broker
- `search_records`: FTS5 keyword search on `records_fts`
- MCP streamable HTTP is served by the `mcp-query` module at `POST /mcp` on
  `[http].listen`. See [MCP client setup](../getting-started/mcp-client.md).
- Built-in read tools are registered in-process; module tools from `[[mcp.tools]]`
  are aggregated via `CoreServices.CallMCPTool` — see [mcp-tools](./mcp-tools.md).

## Acceptance criteria

- [x] MCP `search_records` returns matching folded records
- [x] MCP `get_record` resolves by `record_ref`
- [x] MCP `list_incomplete_records` returns incomplete records
- [x] OpenClaw or Cursor can connect as MCP client

## Dependencies

- **Blocks:** end-to-end v0 validation
- **Blocked by:** journal with FTS5, records projection

## Open questions

- Gateway auth for MCP — resolved via `[http.auth].validator`; see [auth.md](./auth.md)
