---
title: Config loader
parent: Planning
nav_order: 4
---

# Config loader

**Status:** Planned\
**Milestone:** 1 — Journal + module core\
**Spec:** [Configuration §10](../spec.md#10-configuration)\
**Package:** `internal/config`

## Goal

Load and validate Trove TOML configuration: journal path, blob settings, module
paths, remote listener, MCP listen address.

## Interfaces

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

Go struct with sensible defaults for development.

## Implementation notes

- Use a TOML library (e.g. `BurntSushi/toml` or `pelletier/go-toml`)
- Expand `~` in paths
- Validate required fields for enabled subsystems
- Flag: `-config /path/to/trove.toml` in `cmd/trove`

## Acceptance criteria

- [ ] Parses example config from spec
- [ ] Returns clear errors on missing/invalid fields
- [ ] `main` loads config before starting journal/modules

## Dependencies

- **Blocks:** journal (db path), module runtime (paths), MCP (listen)
- **Blocked by:** none

## Open questions

- Default config file location (XDG? `/etc/trove/config.toml`?)
