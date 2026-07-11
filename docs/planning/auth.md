---
title: Network auth
parent: Planning
nav_order: 12
---

# Network auth

**Status:** Supported\
**Milestone:** Decision before wider network exposure\
**Spec:** [Open items §13](../spec.md#13-open-items-not-yet-decided)\
**Package:** `internal/config` + `internal/gateway`

## Goal

Decide and implement an authentication model for Trove's network-facing endpoints
before exposing them beyond localhost or a trusted tailnet.

## Scope

| Endpoint | Config | Current state |
|----------|--------|---------------|
| HTTP ingest | `[http].listen` + gateway | Optional Bearer token |
| MCP query | `POST /mcp` on gateway | Optional Bearer token (same token) |
| Blob upload | `PUT /blobs` on gateway | Optional Bearer token (same token) |
| Remote modules | `[modules.remote].listen` | Not implemented |

## Chosen model

**Shared secret Bearer token** — set `[http].auth_token` in core config. When set,
every gateway route requires `Authorization: Bearer <token>`. When unset, endpoints
remain open (localhost development only).

## Implementation notes

- Enforced in `internal/gateway` before module dispatch
- Constant-time token comparison
- iOS Shortcuts: add `Authorization` header `Bearer <token>` on ingest/blob requests
- MCP clients: configure Bearer token on the HTTP transport (see
  [MCP client setup](../getting-started/mcp-client.md))

## Acceptance criteria

- [x] Auth model chosen and documented in config
- [x] HTTP ingest rejects unauthenticated requests when auth enabled
- [x] MCP query rejects unauthenticated requests when auth enabled
- [x] iOS Shortcuts / MCP client setup documented for chosen model

## Dependencies

- **Blocks:** safe exposure on untrusted networks (partial — single shared secret)
- **Blocked by:** —

## Open questions

- Tailscale identity headers vs reverse-proxy auth — future hardening if needed
