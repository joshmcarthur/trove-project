---
title: "Welcome"
nav_order: 0
description: Capture broadly, store simply, converse to retrieve — your personal event journal.
---

<div class="trove-hero">

<p class="trove-tagline">Capture broadly, store simply, converse to retrieve.</p>

<p class="trove-lead">
Trove is a small, self-contained personal data store: one binary that captures
typed events from the sources in your life into a durable journal, with a
conversational MCP interface as the primary way to get information back out.
</p>

<div class="trove-cta">
  <a href="./getting-started/try-in-a-day/">Try Trove in a day</a>
  <span class="trove-cta-hint">Install, capture six typed events, query them via MCP — about two hours.</span>
</div>

</div>

## What it feels like when it works

You jot a quick note from your phone, bookmark a link, or POST JSON from a script.
Later you ask Cursor "what did I save about example.com this week?" and Trove
answers from your journal — no bespoke app per source, no SQL.

Trove v0 runs as a single Go binary with HTTP ingest, iOS Shortcuts, MQTT,
Telegram, and MCP query. See the [roadmap](./roadmap.md) for current status.

## Documentation

| Section | Start here |
|---------|------------|
| Getting started | [Try in a day](./getting-started/try-in-a-day.md) |
| Architecture | [Concepts](./concepts.md) |
| Status & build order | [Roadmap](./roadmap.md) |
| Full specification | [Specification](./spec.md) |

## Development

See [contributing](./contributing.md) and [AGENTS.md](https://github.com/joshmcarthur/trove/blob/main/AGENTS.md)
for local commands and agent workflow.

## License

GNU GPLv3 — see the [LICENSE](https://github.com/joshmcarthur/trove/blob/main/LICENSE).
