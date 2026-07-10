---
title: Events
parent: Concepts
nav_order: 1
---

# Events

An **event** is an immutable fact: something that happened, captured once, never
edited. If something changes, a new event is appended — nothing is mutated in
place.

See [spec §3](../spec.md#3-core-concepts) for the canonical definition.

## Event shape

```json
{
  "id": "01JXYZ...",
  "time": "2026-07-10T10:00:00+12:00",
  "type": "meshtastic.message.received",
  "source": "radio-node-1",
  "payload": { "...": "..." },
  "blob_ref": null
}
```

| Field | Type | Notes |
|-------|------|-------|
| `id` | ULID | Sortable, unique, generated at ingest |
| `time` | RFC3339 | Event time, not ingest time |
| `type` | string | Dotted namespace, e.g. `mqtt.tararuawx.temp` |
| `source` | string | Free-text origin (topic, device, app) |
| `payload` | JSON | Arbitrary structured data |
| `blob_ref` | string \| null | Optional attachment reference |

## Type naming

Use namespaced strings: `<source-family>.<subject>.<verb>`. There is **no schema
registry** — `type` is a convention, and `payload` is whatever JSON the source
module produces. If a shape changes, use naming discipline (e.g. `.v2` suffixes)
rather than in-place mutation.

## Immutability

Events are never updated. Corrections and follow-ups are new events. The journal
is append-only.

## Implementation

Events are persisted by the [journal](./journal.md). See
[planning/journal.md](../planning/journal.md) for the SQLite schema and
`Journal` interface.
