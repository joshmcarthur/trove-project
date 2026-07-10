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

The config loader (`internal/config`) and SQLite journal (`internal/journal`) are
implemented. With a valid config file, `trove` validates settings and opens the
journal database:

```bash
trove -config /path/to/trove.toml
```

Module runtime and HTTP ingest are not wired up yet — the process exits after
opening the journal.

## What's coming (milestone 1)

To finish milestone 1:

1. Module discovery and go-plugin runtime (see
   [planning/module-runtime.md](../planning/module-runtime.md)).
2. HTTP ingest module — `POST /ingest/:source` (see
   [planning/http-ingest.md](../planning/http-ingest.md)).
3. MCP query server (milestone 3).

When milestone 1 is complete:

1. Configure paths in TOML (see [configuration](./configuration.md)).
2. Start `trove` — core loads modules from configured paths.
3. POST JSON to `POST /ingest/:source` to append events.

## iOS Shortcuts (after HTTP ingest)

When generic HTTP ingest is live, point an iOS Shortcut at your Trove instance:

```
POST https://your-host/ingest/shortcuts
Content-Type: application/json

{ "title": "...", "body": "..." }
```

The `:source` path segment becomes the event `source` field.

## Next steps

- [Roadmap](../roadmap.md) — what to build and in what order
- [Configuration](./configuration.md) — TOML shape (§10)
- [Planning: module runtime](../planning/module-runtime.md) — next milestone 1 task
