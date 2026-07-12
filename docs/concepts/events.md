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
  "type": "trove://type/mqtt/message/received/1",
  "schema_ref": "sha256-abc123...",
  "source": "home/sensors/temp",
  "payload": { "...": "..." },
  "blob_ref": null
}
```

| Field | Type | Notes |
|-------|------|-------|
| `id` | ULID | Sortable, unique, generated at ingest |
| `time` | RFC3339 | Event time, not ingest time |
| `type` | string | `trove://type/{path}/{version}` URI — see [type catalog](./type-catalog.md) |
| `schema_ref` | string | Content hash of the TTD that validated `payload` at emit time |
| `source` | string | Free-text origin (topic, device, app) |
| `payload` | JSON | Structured data; shape enforced by the type catalog (JTD) |
| `blob_ref` | string \| null | Optional attachment reference |

## Type naming

Event `type` values are **`trove://` URIs** registered in the [type catalog](./type-catalog.md),
for example `trove://type/note/created/1`. Modules declare which types they may emit
in `provides` and subscriptions in `consumes` (exact URIs or `trove://type/.../*`
wildcard patterns for routing). Each concrete type must have a Trove Type Definition
(TTD) with an RFC 8927 JTD `definition`; validated emits stamp `schema_ref` on the
journal row. There is no central schema registry **service** — the catalog is built
locally at startup. If a payload shape changes, bump the URI version segment
(`/2`, `/3`, …) rather than mutating events in place.

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
`trove://type/classify/pending/1` for quick capture, then
`trove://type/classify/assigned/1` plus a typed target event when classified later.

## Implementation

Events are persisted by the [journal](./journal.md). The [type catalog](./type-catalog.md)
validates payloads and stamps `schema_ref` at every emit boundary (module `Emit`,
HTTP ingest, classify) — see [building modules](../building-modules.md). See
[planning/journal.md](../planning/journal.md) for the SQLite schema and
`Journal` interface.
