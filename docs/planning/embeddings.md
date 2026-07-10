---
title: Embeddings
parent: Planning
nav_order: 10
---

# Embeddings / semantic search

**Status:** Later\
**Milestone:** After two-week live test\
**Spec:** [Journal §4](../spec.md#4-journal), [Processors §7](../spec.md#7-processors-and-sinks)\
**Package:** `internal/journal`

## Goal

Optional semantic search via `sqlite-vec` virtual table, populated by an embedding
processor on write. Keeps vectors in the same SQLite file as events.

## Interfaces

Extends `search_events` to support fuzzy/semantic matching when index exists.

## Implementation notes

- Embedding processor module or internal hook on append
- `sqlite-vec` extension — CGO/SQLite build implications for Pi
- Model choice: local ONNX vs remote API

## Acceptance criteria

- [ ] Embeddings stored alongside events
- [ ] `search_events` returns semantically similar results
- [ ] Degrades gracefully when embeddings disabled

## Dependencies

- **Blocked by:** journal, FTS5 keyword search working first

## Open questions

- Embedding model — [open-items.md](../open-items.md)
- AI-derived events non-determinism — spec §7
