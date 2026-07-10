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
runtime (`internal/modules`), HTTP ingest module (`modules/http-ingest`), MQTT
source module (`modules/mqtt-source`), and MCP query server (`internal/query`)
are implemented. With a valid config file, `trove` opens the journal, discovers
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

The MCP server listens on `[mcp].listen` (default `:8081`) and exposes four tools:
`search_events`, `get_event`, `get_events_by_type`, and `summarize_range`. See
[MCP query planning](../planning/mcp-query.md).

MQTT source subscribes to configured topics in `modules/mqtt-source/manifest.toml`
(default broker `tcp://localhost:1883`, topics `["home/#"]`). See
[MQTT source planning](../planning/mqtt-source.md) and
[building modules](../building-modules.md).

## What's coming

- [Blob store](../planning/blobs.md) — photo/attachment upload for iOS Shortcuts
  share sheet (`PUT /blobs` then ingest with `blob_ref`)
- Two-week live test — capture recipes and conversational retrieval validation

JSON-only capture works today. Photo attachments require the blob store (Planned).

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
