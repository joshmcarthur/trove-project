---
title: Network auth
parent: Planning
nav_order: 12
---

# Network auth

**Status:** Open\
**Milestone:** Decision before wider network exposure\
**Spec:** [Open items §13](../spec.md#13-open-items-not-yet-decided)\
**Package:** `internal/config` + modules

## Goal

Decide and implement an authentication model for Trove's network-facing endpoints
before exposing them beyond localhost or a trusted tailnet.

## Scope

| Endpoint | Config | Current state |
|----------|--------|---------------|
| HTTP ingest | module listen address | No authentication |
| MCP query | `[mcp].listen` | No authentication |
| Remote modules | `[modules.remote].listen` | Not implemented |

v0 is suitable only for localhost or trusted network boundaries. This becomes
especially important once [blob store](./blobs.md) exposes binary upload via
`PUT /blobs`.

## Options to decide

- **Tailscale identity** — bind via tailnet hostname; verify
  `X-Tailscale-User` or equivalent (matches existing OpenClaw pattern)
- **Reverse-proxy auth** — Caddy/Traefik in front; Trove stays bind-localhost
- **Shared secret / API key** — header or Bearer token in core config; simplest
  for LAN + Shortcuts
- **mTLS** — heavier; likely overkill for single-user Pi

## Implementation notes

- Not blocking MQTT source or live test on a trusted tailnet
- Decide before exposing HTTP ingest or MCP beyond localhost/Tailscale
- Auth may live in core (MCP) and/or http-ingest module depending on approach

## Acceptance criteria

- [ ] Auth model chosen and documented in config
- [ ] HTTP ingest rejects unauthenticated requests when auth enabled
- [ ] MCP query rejects unauthenticated requests when auth enabled
- [ ] iOS Shortcuts / MCP client setup documented for chosen model

## Dependencies

- **Blocks:** safe exposure on untrusted networks
- **Blocked by:** decision on auth model

## Open questions

- Tailscale-only vs shared secret vs reverse-proxy — [open-items.md](../open-items.md)
