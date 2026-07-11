---
title: Configuration
parent: Getting started
nav_order: 3
---

# Configuration

Trove uses TOML (not YAML). Full detail is in [spec §10](../spec.md#10-configuration).

**Status:** Supported — see [planning/config.md](../planning/config.md).

> **Not yet active:** `[modules.remote]` is accepted by the config loader but has
> no runtime effect until the [remote modules](../planning/remote-modules.md)
> milestone lands.

## Example

```toml
[journal]
path = "/data/trove.db"
# retention_days = 90   # optional: delete events older than N days on startup

[blobs]
backend = "filesystem"
path = "/data/blobs"

[modules]
paths = ["/usr/local/lib/trove/modules", "~/.local/lib/trove/modules"]

# Optional per-module settings overlays (see below).
[modules.config]
mqtt-source = "/etc/trove/mqtt.toml"

[modules.settings.mqtt-source]
broker = "tcp://mosquitto:1883"

[modules.remote]
listen = "tailscale:trove"

[http]
listen = ":8080"

# Optional gateway auth (requires http-gateway module in [modules].paths)
# [http.auth]
# validator = "module.http-gateway.bearer"
#
# [modules.settings.http-gateway]
# token_env = "TROVE_HTTP_TOKEN"
```

## Principles

- Core config covers journal path, blob backend, module search paths, and the HTTP
  gateway listen address (`[http].listen`).
- MCP is provided by the `mcp-query` module at `POST /mcp` on the same listener.
- Per-module settings (broker URLs, topics, tokens) live in each module's own
  `manifest.toml` by default — the core does not validate module-specific shapes.
- Optional **`[modules.settings]`** and **`[modules.config]`** in `trove.toml`
  overlay or replace those values at runtime without editing files under
  `modules/`.

## Module settings overlays

Modules read `manifest.toml` beside their binary. Trove can pass additional
settings from `trove.toml` via the `TROVE_MODULE_SETTINGS` environment variable
(set automatically when overlays are configured).

**Inline overlay** — keys under `[modules.settings.<module-name>]`:

```toml
[modules.settings.mqtt-source]
broker = "tcp://mosquitto:1883"
topics = ["sensors/#"]
```

**External file** — map module name to a TOML file path in `[modules.config]`:

```toml
[modules.config]
telegram-source = "/etc/trove/telegram.toml"
```

When both are set for the same module, the external file is loaded first and
inline settings are merged on top. Overlay keys override the module manifest;
arrays and tables in the overlay replace the manifest values wholesale.

Module `manifest.toml` is still required for discovery (`name`, `provides`, HTTP
routes, MCP tools). Use overlays for deployment-specific values (brokers, chat
IDs, secrets paths) rather than duplicating the full manifest.

## Local development

Pass the config file path with `-config`:

```bash
trove -config /path/to/trove.toml
```

With a valid config, `trove` opens the journal database, discovers source modules
from `[modules].paths`, starts the HTTP gateway on `[http].listen` (ingest routes
and MCP), and supervises modules until interrupted. Invalid or missing config fails fast with a
clear error. Journal open failures (e.g. unwritable path) are reported before
exit. Use `trove -version` without a config file.
