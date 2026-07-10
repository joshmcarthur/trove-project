---
title: Configuration
parent: Getting started
nav_order: 3
---

# Configuration

Trove uses TOML (not YAML). Full detail is in [spec §10](../spec.md#10-configuration).

**Status:** Supported — see [planning/config.md](../planning/config.md).

> **Not yet active:** `[blobs]` and `[modules.remote]` are accepted by the config
> loader but have no runtime effect until the [blob store](../planning/blobs.md)
> and [remote modules](../planning/remote-modules.md) milestones land.

## Example

```toml
[journal]
path = "/data/trove.db"

[blobs]
backend = "filesystem"
path = "/data/blobs"

[modules]
paths = ["/usr/local/lib/trove/modules", "~/.local/lib/trove/modules"]

[modules.remote]
listen = "tailscale:trove"

[mcp]
listen = ":8081"
```

## Principles

- Core config covers journal path, blob backend, module search paths, and MCP
  listen address.
- Per-module settings (broker URLs, topics, tokens) live in each module's own
  `manifest.toml` — the core does not need to know module-specific shapes.

## Local development

Pass the config file path with `-config`:

```bash
trove -config /path/to/trove.toml
```

With a valid config, `trove` opens the journal database, discovers source modules
from `[modules].paths`, and supervises them until interrupted. Invalid or missing
config fails fast with a clear error. Journal open failures (e.g. unwritable path)
are reported before exit. Use `trove -version` without a config file.
