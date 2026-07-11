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
- `Subscribe` applies the same filters as `Query`, including `Filter.Text` (FTS)
- `Subscribe` uses a bounded, non-blocking channel — slow subscribers may miss
  live notifications; the event router does not rely on pub/sub delivery and
  instead pulls undispatched events via a durable `last_dispatched_id` cursor
- `router_state` and `event_dispatch` tables support cursor-based routing replay

## Acceptance criteria

- [x] Append persists event with ULID
- [x] Query by type prefix, source, time range
- [x] Query with `Filter.Text` performs FTS5 keyword search
- [x] Get by id
- [x] Subscribe streams new events
- [x] Optional `retention_days` prunes events older than the configured window on startup
- [x] Router cursor (`router_state`) enables pull-based dispatch independent of pub/sub drops

## Dependencies

- **Blocks:** HTTP ingest, MCP query
- **Blocked by:** config loader (for db path)

## Open questions

- ~~Retention / pruning~~ — resolved: `[journal].retention_days` deletes events older than N days on startup (FTS rows included). Blob orphan cleanup is a follow-up.
