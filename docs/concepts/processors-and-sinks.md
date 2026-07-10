---
title: Processors and sinks
parent: Concepts
nav_order: 7
---

# Processors and sinks

Kept deliberately minimal in v0.

See [spec §7](../spec.md#7-processors-and-sinks).

## Processors

Consume events and may emit derived events or write blobs. Build only when a
concrete need appears during validation:

- **Embedding generator** — feeds `sqlite-vec` (see [embeddings planning](../planning/embeddings.md))
- Treat AI-derived events as one-shot facts unless you snapshot model + prompt +
  version for replay

## Sinks

Consume events and take an action (thermal printer, notification). Not needed for
v0 — add when a workflow demands it.

## Implementation

**Status:** Later — [planning/processors-sinks.md](../planning/processors-sinks.md)
