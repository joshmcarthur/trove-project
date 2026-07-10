---
title: Module runtime
parent: Planning
nav_order: 2
---

# Module runtime

**Status:** Supported\
**Milestone:** 1 — Journal + module core\
**Spec:** [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/modules`

## Goal

Discover modules from filesystem paths, read manifests, and launch local modules
via hashicorp/go-plugin with crash isolation and gRPC transport.

## Interfaces

RPC surface (module ↔ core):

```
Source    : core receives Emit(event) from module
Processor : core calls Process(event) -> []event
Sink      : core calls Handle(event) -> ack
All kinds : core calls Healthcheck() periodically
```

## Implementation notes

- Scan `[modules].paths` from config (standard Linux paths + home dir)
- Parse `manifest.toml` (`name`, `version`, `kind`, `provides`) — landed in
  `internal/modules/manifest.go`
- Filesystem discovery scanner — landed in `internal/modules/discover.go`
- Integrate go-plugin for subprocess lifecycle
- Supervise with restart + backoff on crash
- Only `kind = "source"` modules are started at runtime; processor and sink
  kinds are accepted in manifests but not wired yet
- React to `SIGHUP` for reload (stretch)

## Acceptance criteria

- [x] Discovers module directories with valid manifests
- [x] Starts source module and receives Emit calls into journal
- [x] Module crash does not take down core
- [x] Healthcheck RPC wired

## Dependencies

- **Blocks:** HTTP ingest (as first module), MQTT source
- **Blocked by:** config loader (module paths)

## Open questions

- Remote path is separate — see [remote-modules.md](./remote-modules.md)
