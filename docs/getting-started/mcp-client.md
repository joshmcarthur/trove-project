---
title: MCP client setup
parent: Getting started
nav_order: 6
---

# MCP client setup

Query your Trove journal from Cursor (or any MCP client that supports Streamable
HTTP). Trove exposes four core tools backed by the internal query API, plus
additional tools registered by loaded modules — see
[MCP query planning](../planning/mcp-query.md).

## Prerequisites

1. Build Trove and configure `[http].listen` — see [Quick Start](./quick-start.md).
2. `trove` running with a populated journal (capture events via
   [HTTP ingest](./quick-start.md#capture-events) or Shortcuts first).
3. Default HTTP gateway address: `:8080` (MCP at `POST /mcp` on the same port).

## Connect Cursor

Create `.cursor/mcp.json` in your project root, or edit `~/.cursor/mcp.json` for
a global setup. A committed example lives at
[`examples/mcp/cursor-mcp.json`](https://github.com/joshmcarthur/trove/blob/main/examples/mcp/cursor-mcp.json).

```json
{
  "mcpServers": {
    "trove": {
      "url": "http://127.0.0.1:8080/mcp"
    }
  }
}
```

Adjust the host and port to match `[http].listen` in your config. Reload Cursor
(Settings → MCP, or restart the editor) after saving.

## Verify the connection

1. Open **Cursor Settings → MCP**. The `trove` server should show as connected.
2. Confirm at least **4 core tools** are listed:
   - `search_events` — FTS5 keyword search
   - `get_event` — fetch one event by ULID
   - `get_events_by_type` — events with an exact type
   - `summarize_range` — counts by type and notable events for a time window
3. When `capture-classifier` is loaded, also expect `classify_event` and
   `list_unclassified_captures`.
4. In chat, ask the agent to call `summarize_range` for today, or `search_events`
   with a keyword from a captured event.

If the server fails to connect, check that `trove` is running and that nothing
else is bound to the MCP port.

## Available tools

| Tool | Purpose |
|------|---------|
| `search_events` | Keyword search with optional `type_prefix`, `source`, `time_from`, `time_to` |
| `get_event` | Single event by `id` (ULID) |
| `get_events_by_type` | All events of an exact `type`, optional time range |
| `summarize_range` | Aggregated `total`, `by_type`, and `notable` events for `time_from` / `time_to` |

**Module tools** (when loaded): `classify_event`, `list_unclassified_captures` from
`capture-classifier`.

Tool arguments use RFC3339 timestamps where a time range is accepted.

## Network and auth

By default MCP has **no authentication**. When `[http.auth].validator` is set
(e.g. `module.http-gateway.bearer`), configure your MCP client to send
`Authorization: Bearer <token>` on requests to `/mcp`. See
[network auth planning](../planning/auth.md).

Common setups:

- **Local development** — `http://127.0.0.1:8080/mcp` while `trove` runs on the same
  machine as Cursor.
- **Tailscale** — point `url` at your tailnet hostname with `/mcp` path if Trove runs on a home
  server (recommended for remote access).
- **Reverse proxy** — terminate TLS in front of `[http].listen` and use an `https://`
  URL in `mcp.json`.

Do not expose an unauthenticated MCP endpoint on the public internet.

## See also

- [Query concept](../concepts/query.md) — RPC and tool design
- [Configuration](./configuration.md) — `[http].listen` in TOML
- [iOS Shortcuts](./ios-shortcuts.md) — capture events to query later
