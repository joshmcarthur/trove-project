---
title: Processors and sinks
parent: Planning
nav_order: 11
---

# Processors and sinks

**Status:** Later\
**Milestone:** After two-week live test\
**Spec:** [Processors and sinks §7](../spec.md#7-processors-and-sinks)\
**Package:** external modules

## Goal

Optional derived-event processors and side-effect sinks — only when a concrete
workflow needs them (e.g. thermal printer for trip summaries).

## Interfaces

```
Processor : core calls Process(event) -> []event synchronously
Sink      : core calls Handle(event) -> ack
```

## Implementation notes

- Processors may emit new events (e.g. embeddings) — treat AI output as one-shot
  facts unless model/version snapshotted
- Sinks: notifications, printers — out of scope until needed
- Both use same module manifest `kind` field

## Acceptance criteria

- [ ] Processor module receives events and can emit derived events
- [ ] Sink module receives events and acknowledges
- [ ] Processor crash isolated like sources

## Dependencies

- **Blocked by:** module runtime, journal

## Open questions

- Which processor to build first depends on live-test gaps
