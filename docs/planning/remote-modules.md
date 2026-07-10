---
title: Remote modules
parent: Planning
nav_order: 9
---

# Remote modules

**Status:** Later\
**Milestone:** After two-week live test\
**Spec:** [Module architecture §8](../spec.md#8-module-architecture-dynamic-socket-based)\
**Package:** `internal/modules`

## Goal

Accept module connections from edge devices over Tailscale (e.g. Tararua Meshtastic
node) via a plain gRPC listener — same event/RPC shapes as local modules,
different transport.

## Interfaces

Same RPC as local modules (`Emit`, `Healthcheck`, etc.) over gRPC server on
`[modules.remote].listen`.

## Implementation notes

- go-plugin explicitly does not support real networks — separate code path
- Likely reuse protobuf definitions from local protocol
- Edge module dials in; core does not spawn remote process
- Auth via Tailscale identity

## Acceptance criteria

- [ ] Remote source can Emit events over tailnet
- [ ] Connection loss handled without core crash
- [ ] Distinct from go-plugin local path in codebase

## Dependencies

- **Blocked by:** local module runtime, journal

## Open questions

- RPC protocol shape — [open-items.md](../open-items.md)
