---
title: Configuration
parent: Getting started
nav_order: 3
---

# Configuration

Trove uses TOML (not YAML). Full detail is in [spec §10](../spec.md#10-configuration).

**Status:** Planned — see [planning/config.md](../planning/config.md).

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

## Local development (planned)

A config file path will be passed via flag or discovered from a conventional
location. Until the config loader is implemented, no config file is required
for `trove -version`.
