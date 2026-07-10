---
title: HTTP ingest
parent: Planning
nav_order: 3
---

# HTTP ingest

**Status:** Supported\
**Milestone:** 1 — Journal + module core\
**Spec:** [Sources §6](../spec.md#6-sources), [Build order §11.1](../spec.md#11-build-order--validation-plan)\
**Package:** external source module + `internal/modules`

## Goal

Generic `POST /ingest/:source` endpoint that accepts arbitrary JSON, wraps it as
an event with `source` from the URL path. Highest-leverage v0 ingest path.

## Interfaces

Source module emitting via RPC:

```go
// Conceptual — realized over module socket
type Source interface {
    Name() string
    Run(ctx context.Context, emit func(Event)) error
}
```

Event shape: `type` from payload or default `http.ingest.received`; `source` from
`:source` path segment.

## Implementation notes

- Implement as a **module**, not wired directly into core — proves module boundary
- Listen on configurable port (module config, not core)
- Parse JSON body as `payload`; optional `time`, `type`, and `blob_ref` fields in body
- `max_body_bytes` in module manifest (default 10 MiB); raise for larger JSON payloads
- Large binary content (photos, audio) should **not** be inlined — use `blob_ref` on the
  event and store bytes via `PUT /blobs` ([blobs](./blobs.md))
- `provides` in manifest controls allowed client `type` values (wildcards such as
  `note.*` supported); early HTTP 400 for disallowed types
- Optional `[schemas]` validated at core `Emit`; failures return HTTP 400

### Request / response

| Request | Response |
|---------|----------|
| `POST /ingest/{source}` with valid JSON body | `204 No Content` |
| Empty body, invalid JSON, bad metadata fields | `400 Bad Request` |
| Type not in manifest `provides` | `400 Bad Request` |
| Schema validation failure (when declared) | `400 Bad Request` |
| Non-POST to `/ingest/{source}` | `405 Method Not Allowed` |
| Other Emit failure | `500 Internal Server Error` |
| `PUT /blobs` with body bytes | `201 Created` + `{ "blob_ref": "sha256-..." }` |
| Empty or oversize `PUT /blobs` body | `400 Bad Request` |
| Non-PUT to `/blobs` | `405 Method Not Allowed` |
| Blob store failure on `PUT /blobs` | `500 Internal Server Error` |

Optional JSON object fields peeled into event metadata: `type`, `time` (RFC3339),
`blob_ref`. Remaining keys become `payload`. Default event type:
`http.ingest.received`.

## Acceptance criteria

- [x] `POST /ingest/shortcuts` with JSON creates journal event
- [x] `source` field matches path segment
- [x] Invalid JSON returns 4xx without journal write
- [x] Disallowed type returns 400 without journal write
- [x] Module runs under go-plugin supervision

## Dependencies

- **Blocks:** iOS Shortcuts capture, webhook integrations
- **Blocked by:** journal, module runtime, config

## See also

- [iOS Shortcuts guide](../getting-started/ios-shortcuts.md) — importable
  Shortcuts and payload conventions for `POST /ingest/shortcuts`

## Open questions

- Auth model — [auth.md](./auth.md), [open-items.md](../open-items.md)
- Blob upload: `PUT /blobs` — see [blobs](./blobs.md) (Supported)
