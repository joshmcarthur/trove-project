---
title: Quick Start
parent: Getting started
nav_order: 3
---

# Quick Start

Developer reference for what Trove implements today. **First time here?** Follow
[Try in a day](./try-in-a-day.md) for a guided afternoon experiment instead.

## What works today

Build and run the CLI:

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
./bin/trove -version
```

The config loader (`internal/config`), SQLite journal (`internal/journal`), blob
store (`internal/blob`), module runtime (`internal/modules`), HTTP gateway
(`internal/gateway`), HTTP ingest module (`modules/http-ingest`), MQTT source
(`modules/mqtt-source`), Telegram source (`modules/telegram-source`), deferred
capture (`modules/capture-classifier`), and MCP query (`internal/query` +
`modules/mcp-query`) are implemented. With a valid config file, `trove` opens the
journal, discovers modules, supervises them, and starts the HTTP gateway (ingest,
blobs, capture, classify, MCP) until interrupted:

```bash
make build
trove -config /path/to/trove.toml
```

Point `[modules].paths` at the parent directory containing module installs (for
example, the repo `modules/` directory after `make build`). Then POST JSON to
`POST /ingest/:source` on the HTTP gateway listen address (default `:8080`).

### Example config

```toml
[journal]
path = "./trove.db"

[modules]
paths = ["./modules"]

[http]
listen = ":8080"
```

### Query the journal

Connect Cursor (or another MCP client) to `http://<host>:8080/mcp` on the HTTP
gateway — see [MCP client setup](./mcp-client.md). Four core tools are always
available: `search_events`, `get_event`, `get_events_by_type`, and
`summarize_range`. Additional tools (e.g. `classify_event`) appear when modules
that register MCP tools are loaded.

MQTT source subscribes to configured topics in `modules/mqtt-source/manifest.toml`
(default broker `tcp://localhost:1883`, topics `["home/#"]`). See
[MQTT source planning](../planning/mqtt-source.md) and
[building modules](../building-modules.md).

## What's coming

- **Two-week live test** — capture recipes and conversational retrieval validation
  (see [roadmap](../roadmap.md))

Photo attachments work today via `PUT /blobs` then ingest with `blob_ref` — see
[iOS Shortcuts](./ios-shortcuts.md).

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

- [Two-week live test](./live-test.md) — milestone 4 runbook and validation checklist
- [MCP client setup](./mcp-client.md) — connect Cursor to query your journal
- [iOS Shortcuts](./ios-shortcuts.md) — importable Shortcuts and capture recipes
- [Roadmap](../roadmap.md) — what to build and in what order
- [Configuration](./configuration.md) — TOML shape (§10)
- [Planning: HTTP ingest](../planning/http-ingest.md) — generic webhook capture
