---
title: Quick Start
parent: Getting started
nav_order: 2
---

# Quick Start

Trove is not yet feature-complete. This page describes what works today and what
comes next.

## What works today

Build and run the CLI:

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
./bin/trove -version
```

The config loader (`internal/config`), SQLite journal (`internal/journal`), module
runtime (`internal/modules`), and HTTP ingest module (`modules/http-ingest`) are
implemented. With a valid config file, `trove` opens the journal, discovers
source modules, supervises them, and starts the MCP query server until
interrupted:

```bash
make build
trove -config /path/to/trove.toml
```

Point `[modules].paths` at the parent directory containing module installs (for
example, the repo `modules/` directory after `make build`). Then POST JSON to
`POST /ingest/:source` on the HTTP ingest listen address (default `:8080`).

### Example config

```toml
[journal]
path = "./trove.db"

[modules]
paths = ["./modules"]

[mcp]
listen = ":8081"
```

## What's coming

MCP query is partially wired: `search_events` and `get_event` work today.
`summarize_range` and full client validation (OpenClaw / Cursor) are still
outstanding — see [MCP query planning](../planning/mcp-query.md).

## Capture events

1. Configure paths in TOML (see [configuration](./configuration.md)).
2. Start `trove` — core loads modules from configured paths.
3. POST JSON to `POST /ingest/:source` to append events.

### HTTP ingest responses

| Status | Meaning |
|--------|---------|
| `204` | Event accepted (empty body) |
| `400` | Invalid JSON, missing body, or bad `type` / `time` / `blob_ref` |
| `405` | Non-POST request to `/ingest/:source` |
| `500` | Internal emit failure |

## iOS Shortcuts

Import a ready-made Shortcut or build your own — see
[iOS Shortcuts](./ios-shortcuts.md). All Shortcuts POST JSON to
`https://<your-host>/ingest/shortcuts`; the `:source` path segment becomes the
event `source` field.

## Next steps

- [iOS Shortcuts](./ios-shortcuts.md) — importable Shortcuts and capture recipes
- [Roadmap](../roadmap.md) — what to build and in what order
- [Configuration](./configuration.md) — TOML shape (§10)
- [Planning: HTTP ingest](../planning/http-ingest.md) — generic webhook capture
