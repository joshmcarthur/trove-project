---
title: Journal
parent: Concepts
nav_order: 2
---

# Journal

The journal is the single source of truth: an append-only table in SQLite.

See [spec §4](../spec.md#4-journal).

## Model

- One SQLite database file (path from config).
- `events` table with ULID primary key, indexed by time, type, and source.
- Optional FTS5 for keyword search; optional `sqlite-vec` later for semantic
  search.

## Interface

```go
type Journal interface {
    Append(ctx context.Context, e Event) error
    Query(ctx context.Context, f Filter) ([]Event, error)
    Get(ctx context.Context, id string) (Event, error)
    Subscribe(ctx context.Context, f Filter) (<-chan Event, error)
}
```

`Filter` supports type prefix, source, time range, and free-text match at
minimum.

Persistence and routing are separate: every accepted append is durable in
SQLite. The event router pulls undispatched events via a stored cursor, so
processor/sink delivery does not depend on pub/sub channel capacity.

## Why SQLite

Single file, durable, queryable on a Pi without extra services. Fits the
single-user, single-journal model. Alternative journal backends (Postgres, etc.)
are a [non-goal](../non-goals.md).

## Implementation

**Status:** Supported — [planning/journal.md](../planning/journal.md)
