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
source modules, and supervises them until interrupted:

```bash
make build
trove -config /path/to/trove.toml
```

Point `[modules].paths` at the parent directory containing module installs (for
example, the repo `modules/` directory after `make build`). Then POST JSON to
`POST /ingest/:source` on the HTTP ingest listen address (default `:8080`).

## What's coming

To finish milestone 1 validation:

1. MCP query server (milestone 3).

## Capture events

1. Configure paths in TOML (see [configuration](./configuration.md)).
2. Start `trove` — core loads modules from configured paths.
3. POST JSON to `POST /ingest/:source` to append events.

## iOS Shortcuts

```
POST https://your-host/ingest/shortcuts
Content-Type: application/json

{ "title": "...", "body": "..." }
```

The `:source` path segment becomes the event `source` field.

## Next steps

- [Roadmap](../roadmap.md) — what to build and in what order
- [Configuration](./configuration.md) — TOML shape (§10)
- [Planning: HTTP ingest](../planning/http-ingest.md) — generic webhook capture
