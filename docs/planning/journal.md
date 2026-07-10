---
title: "Journal (SQLite)"
parent: Planning
nav_order: 1
---

# Journal

**Status:** Planned\
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
- FTS5 virtual table for `search_events` placeholder
- SQLite driver: `modernc.org/sqlite` (pure Go, no CGO) vs `mattn/go-sqlite3` —
  prefer pure Go for Pi cross-compile unless FTS5/vec needs CGO
- ULID generation at append time

## Acceptance criteria

- [x] Append persists event with ULID
- [ ] Query by type prefix, source, time range
- [x] Get by id
- [ ] Subscribe streams new events

## Dependencies

- **Blocks:** HTTP ingest, MCP query
- **Blocked by:** config loader (for db path)

## Open questions

- Retention / pruning — see [open-items.md](../open-items.md)
