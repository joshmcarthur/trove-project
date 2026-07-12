---
title: "Journal (SQLite)"
parent: Planning
nav_order: 1
---

# Journal

**Status:** Supported\
**Milestone:** 1 — Journal + module core\
**Spec:** [Journal §4](../spec.md#4-journal)\
**Package:** `internal/journal`

## Goal

Implement the append-only SQLite event store — the single source of truth every
other component depends on.

## Interfaces

```go
type Journal interface {
    Append(ctx context.Context, e Event) error
    Query(ctx context.Context, f Filter) ([]Event, error)
    Get(ctx context.Context, id string) (Event, error)
    Subscribe(ctx context.Context, f Filter) (<-chan Event, error)
}
```

## Implementation notes

- Schema DDL from spec §4 (`events` table + indexes on time, type, source)
- FTS5 virtual table (`events_fts`) for keyword search via `Filter.Text`
- SQLite driver: `modernc.org/sqlite` (pure Go, no CGO) vs `mattn/go-sqlite3` —
  prefer pure Go for Pi cross-compile unless FTS5/vec needs CGO
- ULID generation at append time
- `WatchAppends` signals coalesced wakeups after each append (no payload); the
  event router uses this plus a cursor pull — not `Subscribe`
- `Subscribe` applies the same filters as `Query`, including `Filter.Text` (FTS)
- `Subscribe` delivers matching event payloads on a bounded, best-effort channel
  for live streaming consumers; slow subscribers may miss events
- `router_state` and `event_dispatch` tables support cursor-based routing replay

## Acceptance criteria

- [x] Append persists event with ULID
- [x] Query by type prefix, source, time range
- [x] Query with `Filter.Text` performs FTS5 keyword search
- [x] Get by id
- [x] Subscribe streams new events
- [x] Optional `retention_days` prunes events older than the configured window on startup
- [x] Router cursor (`router_state`) enables pull-based dispatch via `WatchAppends`

## Dependencies

- **Blocks:** HTTP ingest, MCP query
- **Blocked by:** config loader (for db path)

## Open questions

- ~~Retention / pruning~~ — resolved: `[journal].retention_days` deletes events older than N days on startup (FTS rows included). Blob orphan cleanup is a follow-up.
