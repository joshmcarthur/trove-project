---
title: "Welcome"
nav_order: 0
---

# Welcome to Trove

Trove is a small, self-contained personal data store: a single binary that
captures typed events from the sources in your life — Meshtastic, Home Assistant,
MQTT, iOS Shortcuts, webhooks, and more — into one durable, queryable journal,
with a conversational MCP interface as the primary way to get information back
out.

> Capture broadly, store simply, converse to retrieve.

The project is migrating from an earlier Deno prototype to Go. Today you can run
`trove`, capture events via the HTTP ingest module, and query the journal through
the MCP server. See the [roadmap](./roadmap.md) for status.

## What's next

Milestones 1 (journal, module runtime, HTTP ingest, config) and 3 (MCP query) are
**Supported**. Current focus: [iOS Shortcuts](./getting-started/ios-shortcuts.md)
capture and the two-week live test. Next feature build: [MQTT source](./planning/mqtt-source.md).

## Documentation

- [Roadmap](./roadmap.md) — status matrix and build order
- [Getting started](./getting-started/installation.md) — install and build
- [Concepts](./concepts.md) — architecture reference
- [Planning](./planning/index.md) — per-feature implementation briefs
- [Specification](./spec.md) — full canonical spec

## Development

See [contributing](./contributing.md) and [AGENTS.md](https://github.com/joshmcarthur/trove/blob/main/AGENTS.md)
for local commands and agent workflow.

## License

GNU GPLv3 — see the [LICENSE](https://github.com/joshmcarthur/trove/blob/main/LICENSE).
