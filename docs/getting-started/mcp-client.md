---
title: MCP client setup
parent: Getting started
nav_order: 6
---

# MCP client setup

Query your Trove records from Cursor (or any MCP client that supports Streamable
HTTP). Trove exposes three core record tools backed by the internal query API, plus
additional tools registered by loaded modules — see
[MCP query planning](../planning/mcp-query.md).

## Prerequisites

1. Build Trove and configure `[http].listen` — see [Quick Start](./quick-start.md).
2. `trove` running with a populated journal (capture records via
   [HTTP ingest](./quick-start.md#capture-records) or Shortcuts first).
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
2. Confirm at least **3 core tools** are listed:
   - `search_records` — FTS5 keyword search over folded records
   - `get_record` — fetch one record by `record_ref`
   - `list_incomplete_records` — records awaiting classification
3. When `capture-classifier` is loaded, also expect module-specific tools.
4. In chat, ask the agent to call `search_records` with a keyword from a captured
   record, or `list_incomplete_records` to find unclassified captures.

If the server fails to connect, check that `trove` is running and that nothing
else is bound to the MCP port.

## Available tools

| Tool | Purpose |
|------|---------|
| `search_records` | Keyword search with optional `type_prefix`, `source`, `time_from`, `time_to`, `include_deleted` |
| `get_record` | Single record by `record_ref` (optional `version`) |
| `list_incomplete_records` | Records with `completeness = incomplete`, optional `source` and `limit` |

**Module tools** (when loaded): additional tools from modules that register
`[[mcp.tools]]` in their manifest.

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
- [iOS Shortcuts](./ios-shortcuts.md) — capture records to query later
