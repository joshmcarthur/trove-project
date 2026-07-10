---
title: HA WebSocket source
parent: Planning
nav_order: 8
---

# HA WebSocket source

**Status:** Later\
**Milestone:** After two-week live test\
**Spec:** [Sources §6](../spec.md#6-sources)\
**Package:** external source module

## Goal

Subscribe to Home Assistant `state_changed` events via `/api/websocket`; emit one
journal event per state change.

## Interfaces

Source module — `Emit(event)` per HA state change. Suggested type:
`homeassistant.state_changed`.

## Implementation notes

- Module config: HA URL, long-lived access token
- Payload: entity_id, old/new state, attributes
- `source`: entity_id or domain
- Handle HA reconnect

## Acceptance criteria

- [ ] Connects to HA WebSocket API
- [ ] State changes append journal events
- [ ] Reconnects after HA restart

## Dependencies

- **Blocked by:** journal, module runtime

## Open questions

- Filter which entities/domains to subscribe to
