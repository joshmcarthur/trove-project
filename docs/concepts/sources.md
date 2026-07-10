---
title: Sources
parent: Concepts
nav_order: 4
---

# Sources

Sources are modules that run continuously (or on a schedule) and emit events
into the core via `Emit(event)`.

See [spec §6](../spec.md#6-sources).

## v0 priority order

1. **Generic HTTP ingest** (`POST /ingest/:source`) — catch-all for Shortcuts,
   webhooks, IFTTT. Highest leverage, build first. See
   [iOS Shortcuts guide](../getting-started/ios-shortcuts.md) for importable
   capture Shortcuts.
2. **MQTT listener** — subscribes to configured topics on Mosquitto; covers
   Meshtastic (MQTT-bridged) and ESPHome traffic.
3. **Home Assistant WebSocket tap** — `state_changed` events from HA's
   `/api/websocket`.

## Later

Batch importers (Google Takeout, photo folders) as standalone scripts that POST
to HTTP ingest — no need for full Trove source modules for occasional jobs.

## Contract (conceptual)

```go
type Source interface {
    Name() string
    Run(ctx context.Context, emit func(Event)) error
}
```

Realized as RPC over the module socket (§8), not an in-process Go interface.

## Planning pages

- [HTTP ingest](../planning/http-ingest.md) — milestone 1, **Supported**
- [MQTT source](../planning/mqtt-source.md) — milestone 2, **Supported**
- [HA source](../planning/ha-source.md) — later
