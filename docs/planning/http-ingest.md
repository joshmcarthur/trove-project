---
title: HTTP ingest
parent: Planning
nav_order: 3
---

# HTTP ingest

**Status:** Planned\
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
- Parse JSON body as `payload`; optional `time` and `type` fields in body
- Even in v0, keep behind go-plugin per spec §11

## Acceptance criteria

- [ ] `POST /ingest/shortcuts` with JSON creates journal event
- [ ] `source` field matches path segment
- [ ] Invalid JSON returns 4xx without journal write
- [ ] Module runs under go-plugin supervision

## Dependencies

- **Blocks:** iOS Shortcuts capture, webhook integrations
- **Blocked by:** journal, module runtime, config

## Open questions

- Auth model (Tailscale-only?) — [open-items.md](../open-items.md)
