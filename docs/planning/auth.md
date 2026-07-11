---
title: Network auth
parent: Planning
nav_order: 12
---

# Network auth

**Status:** Supported (gateway default auth via `http-gateway` module)\
**Milestone:** Decision before wider network exposure\
**Spec:** [Open items §13](../spec.md#13-open-items-not-yet-decided)\
**Package:** `internal/gateway` + `modules/http-gateway`

## Goal

Authenticate Trove's network-facing HTTP endpoints before exposing them beyond
localhost or a trusted tailnet.

## Model

Gateway auth uses **pluggable validator modules** referenced as
`module.<module-name>.<validator-id>`:

```toml
[http.auth]
validator = "module.http-gateway.bearer"
```

The gateway calls `ValidateAuth` on the auth module before dispatching
`HandleHTTP`. Both HTTP routes and auth validators resolve live clients from the
same module registry keyed by module name; config refs use `module.<name>.<capability>`
(e.g. `module.http-gateway.bearer`; HTTP routes implicitly use capability `http`).

Per-route overrides on `[[http.routes]]`:

| `auth` value | Behavior |
|--------------|----------|
| `inherit` (default) | Use `[http.auth].validator` |
| `none` | Skip gateway auth; route module may verify |
| `module.<name>.<id>` | Route-specific validator |

## Scope

| Endpoint | Protection |
|----------|------------|
| HTTP ingest (`POST /ingest/*`) | Gateway validator when configured |
| Blob upload (`PUT /blobs`) | Same |
| MCP (`POST /mcp`) | Same |
| Remote modules | Not implemented — separate Tailscale auth (Later) |

When `[http.auth].validator` is unset, endpoints remain open (localhost dev).

## First-party validator: `module.http-gateway.bearer`

Bearer token settings live in the `http-gateway` module:

```toml
[modules.settings.http-gateway]
token_env = "TROVE_HTTP_TOKEN"
# token = "..."   # optional inline
```

Clients send `Authorization: Bearer <token>`.

## Layered posture (home network)

1. **Network boundary** — Tailscale/VPN/localhost (primary control)
2. **Gateway validator** — optional `module.http-gateway.bearer`
3. **Per-route** — `auth = "none"` or integration-specific validators
4. **Module-local** — webhook HMAC, Telegram allowlist, etc.

## Acceptance criteria

- [x] Auth model chosen and documented in config
- [x] HTTP ingest rejects unauthenticated requests when auth enabled
- [x] MCP rejects unauthenticated requests when auth enabled
- [x] iOS Shortcuts / MCP client setup documented for chosen model

## Open questions

- Tailscale identity validator (`module.http-gateway.tailscale`) — Planned
- Reverse-proxy-only auth (no Trove validator) — document as alternative
