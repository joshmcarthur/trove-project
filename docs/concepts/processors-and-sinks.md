---
title: Processors and sinks
parent: Concepts
nav_order: 8
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

Event routing for processors and sinks is **Supported** — see
[planning/processors-sinks.md](../planning/processors-and-sinks.md).

First-party examples today:

- **`capture-classifier`** — source module with HTTP routes and MCP tools for
  deferred capture classification (not an event-routing processor)
- **`mcp-query`** — HTTP-only processor that exposes the MCP endpoint on the
  gateway

No first-party sink module ships yet; add one when a workflow demands it.
