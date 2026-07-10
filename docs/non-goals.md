---
title: Non-goals
nav_order: 8
---

# Non-goals

Explicitly out of scope — do not re-add without revisiting the spec.

From [spec §12](../spec.md#12-non-goals-explicit):

## Multi-journal / multi-node sync

Not solving edge-node ↔ home-journal reconciliation as a general problem.
Remote modules can stream events to one central journal over Tailscale, but there
is still only one journal.

## Schema registry / formal schema evolution

`type` strings plus JSON payload is enough at this scale.

## WASM guest runtime

Dynamic loading via go-plugin and gRPC is in scope; sandboxed guest execution is
not.

## Perkeep-style content model

No permanodes, claims, or GPG signing. The blob-store _interface shape_ from
Perkeep was kept; its object model was not.

## General-purpose platform

Single-user tool built around specific sources — not "store anything for anyone."
