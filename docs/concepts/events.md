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

Use namespaced strings: `<source-family>.<subject>.<verb>`. There is **no central
schema registry** — modules declare allowed types in `provides` and subscriptions
in `consumes` (exact strings or glob patterns such as `note.*`). Optional JSON
Schema files in the module manifest validate payloads when declared. If a shape
changes, use naming discipline (e.g. `.v2` suffixes) rather than in-place mutation.

## Immutability

Events are never updated. Corrections and follow-ups are new events. The journal
is append-only.

Routing metadata (`seen` module lists, processing chain `root_id`) is passed to
processors and sinks at dispatch time only — it is **not** stored on events in
the journal.

## Derived events

When a new event supersedes or re-types an earlier capture, link them in payload
metadata rather than mutating the original:

```json
{
  "_trove": {
    "derived_from": "01JXYZ..."
  }
}
```

The [capture-classifier](../planning/deferred-capture.md) module emits
`classify.pending` for quick capture, then `classify.assigned` plus a typed target
event when classified later.

## Implementation

Events are persisted by the [journal](./journal.md). Type allowlists and optional
schema validation are enforced at the module `Emit` boundary — see
[building modules](../building-modules.md). See
[planning/journal.md](../planning/journal.md) for the SQLite schema and
`Journal` interface.
