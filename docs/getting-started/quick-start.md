---
title: Quick Start
parent: Getting started
nav_order: 2
---

# Quick Start

Trove is not yet feature-complete. This page describes what works today and what
comes next.

## What works today

Build and run the CLI scaffold:

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
./bin/trove -version
```

Running `trove` without `-version` prints `not yet implemented` and exits — that
is expected until milestone 1 lands.

## What's coming (milestone 1)

Once journal, config, and HTTP ingest are implemented:

1. Configure paths in TOML (see [configuration](./configuration.md)).
2. Start `trove` — core loads modules from configured paths.
3. POST JSON to `POST /ingest/:source` to append events.
4. Query via MCP tools (milestone 3).

Follow [planning/journal.md](../planning/journal.md) for the first implementation
task.

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
- [Planning: HTTP ingest](../planning/http-ingest.md) — milestone 1 ingest module
