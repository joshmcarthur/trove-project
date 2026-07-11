---
title: Deferred capture
parent: Planning
nav_order: 12
---

# Deferred capture

**Status:** Supported\
**Milestone:** 4 — Two-week live test\
**Spec:** [Events §3](../spec.md#3-core-concepts), [Sources §6](../spec.md#6-sources)\
**Package:** `modules/capture-classifier`, `pkg/classify`

## Goal

Capture events quickly before the semantic type is known, then classify later into
properly typed journal events without mutating the original capture.

## Interfaces

HTTP (gateway routes on `capture-classifier`):

```
POST /capture/{source}   -> emit classify.pending
POST /classify           -> emit typed event + classify.assigned
GET  /pending            -> list unclassified classify.pending events
```

MCP tools (registered on `capture-classifier`, aggregated by `mcp-query`):

```
classify_event(source_event_id, target_type, payload?)
list_unclassified_captures()
```

## Event conventions

| Type | Purpose |
|------|---------|
| `classify.pending` | Quick capture awaiting classification |
| `classify.assigned` | Link record: `{source_event_id, target_event_id, target_type}` |
| Target types (e.g. `shortcuts.note.created`) | Properly typed classified event with `_trove.derived_from` |

## Implementation notes

- Shared logic in [`pkg/classify`](../../pkg/classify/)
- Module uses `trovemodule.Core` for emit and query
- MCP tools declared via `[[mcp.tools]]` in manifest — see [mcp-tools](./mcp-tools.md)

## Acceptance criteria

- [x] `POST /capture/shortcuts` emits `classify.pending`
- [x] `POST /classify` emits typed event and `classify.assigned`
- [x] Original pending event is never mutated
- [x] Double-classify rejected
- [x] `GET /pending` omits already-classified pending events
- [x] MCP `classify_event` and `list_unclassified_captures` work via module tool registration
- [x] iOS Shortcuts doc includes quick-capture recipe

## Dependencies

- **Blocked by:** HTTP gateway, module runtime, MCP tool registration
- **Blocks:** two-week live test quick-capture workflows

## See also

- [MCP tools](./mcp-tools.md)
- [iOS Shortcuts](../getting-started/ios-shortcuts.md)
