---
title: Sources
parent: Concepts
nav_order: 8
---

# Sources

Sources are modules that run continuously (or on a schedule) and emit events
into the core via `Emit(event)`.

See [spec §6](../spec.md#6-sources).

## Supported sources

| Source | Planning page |
|--------|---------------|
| HTTP ingest | [http-ingest](../planning/http-ingest.md) |
| MQTT listener | [mqtt-source](../planning/mqtt-source.md) |
| Telegram bot | [telegram-source](../planning/telegram-source.md) |
| Deferred capture (capture-classifier) | [deferred-capture](../planning/deferred-capture.md) |

## Later

- **Home Assistant WebSocket tap** — `state_changed` events from HA's
  `/api/websocket`. See [ha-source](../planning/ha-source.md).

## Contract (conceptual)

```go
type Source interface {
    Name() string
    Run(ctx context.Context, emit func(Event)) error
}
```

Realized as RPC over the module socket (§8), not an in-process Go interface.

Batch importers (Google Takeout, photo folders) can POST to HTTP ingest as
standalone scripts — no full Trove source module required for occasional jobs.
