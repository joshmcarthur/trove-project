---
title: Processors and sinks
parent: Planning
nav_order: 11
---

# Processors and sinks

**Status:** Supported\
**Milestone:** After two-week live test\
**Spec:** [Processors and sinks §7](../spec.md#7-processors-and-sinks)\
**Package:** `internal/modules`, external modules

## Goal

Derived-event processors and side-effect sinks for journal event workflows
(e.g. embeddings, notifications, printers).

## Interfaces

```
Processor : core calls Process(event, DispatchContext) -> []event synchronously
Sink      : core calls Handle(event, DispatchContext) -> ack
```

## Manifest

Event-routing modules declare subscriptions and emissions:

```toml
kind     = "processor"
consumes = ["note.*"]
provides = ["note.embedding.generated"]
```

See [modules concept](../concepts/modules.md) for per-kind rules. HTTP-only
processors (for example `mcp-query`) use `[[http.routes]]` instead of `consumes`.

## Event routing

1. Source or processor emits an event → journal append
2. Router pulls events after `last_dispatched_id` in ULID order (pub/sub is wakeup only)
3. Router matches `consumes` patterns
4. Core calls `Process` / `Handle` with `DispatchContext{root_id, seen}`
5. Processor-derived events are validated against `provides` and appended
6. If a module name is already in `seen`, the event is skipped (loop prevention)
7. Watermark advances only after successful dispatch (at-least-once delivery)

Startup logs a warning when manifest declarations suggest a cyclic graph;
runtime `seen` tracking is the safety net.

Derived-event routing context (`root_id`, `seen`) is persisted in
`event_dispatch` until dispatch completes so restart catch-up preserves loop
prevention.

## Implementation notes

- Processors may emit derived events — treat AI output as one-shot facts unless
  model/version snapshotted
- Dispatch is at-least-once: processors and sinks should tolerate duplicate
  invocations for the same event id (e.g. crash after dispatch, before watermark save)
- Sinks: notifications, printers — add when a workflow demands them
- Event-routing modules use the same go-plugin supervision as sources
- HTTP-only processors remain on the gateway path and do not use the router

## Acceptance criteria

- [x] Processor module receives events and can emit derived events
- [x] Sink module receives events and acknowledges
- [x] Processor crash isolated like sources
- [x] Manifest `consumes` / `provides` validation
- [x] Loop prevention via `DispatchContext.seen`
- [x] Every persisted event is dispatched to matching processors and sinks

## Dependencies

- **Blocked by:** module runtime, journal

## Open questions

- Which processor to build first depends on live-test gaps
