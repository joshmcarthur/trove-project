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
- `revisions` table with ULID primary key, `schema_ref` (TTD content hash), indexed
  by time, type, and source.
- Optional FTS5 on revisions (`revisions_fts`) for module revision queries; record
  search uses `records_fts` on the projection.
- Optional `sqlite-vec` later for semantic search.

## Interface

```go
type Journal interface {
    Append(ctx context.Context, r Revision) error
    Query(ctx context.Context, f Filter) ([]Revision, error)
    Get(ctx context.Context, id string) (Revision, error)
    Watch(ctx context.Context) (<-chan struct{}, error)
}
```

`Filter` supports type prefix, source, time range, and free-text match at
minimum.

New appends signal `Watch` watchers; consumers pull data via `Query`.
The revision router combines `Watch` with a durable cursor for guaranteed
dispatch to processors and sinks.

## Why SQLite

Single file, durable, queryable on a Pi without extra services. Fits the
single-user, single-journal model. Alternative journal backends (Postgres, etc.)
are a [non-goal](../non-goals.md).

## Implementation

**Status:** Supported — [planning/journal.md](../planning/journal.md)
