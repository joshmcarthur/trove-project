---
title: Building modules
nav_order: 11
---

# Building modules

External modules extend Trove without recompiling the core. See [spec §8](../spec.md#8-module-architecture-dynamic-socket-based)
and [concepts/modules.md](./concepts/modules.md).

**Status:** Module runtime is **Supported** for local source modules — see
[planning/module-runtime.md](./planning/module-runtime.md).

## Layout

Install under a configured search path:

```
~/.local/lib/trove/modules/my-source/
    manifest.toml
    module               # executable
```

For local development, `make build` also builds the first-party HTTP ingest
module into `modules/http-ingest/`. Point `[modules].paths` at the repo `modules/`
directory (the parent of each module folder).

## Manifest

```toml
name     = "my-source"
version  = "1.0"
kind     = "source"
provides = ["my-source.event.received"]
listen   = ":8080"   # optional module-specific setting (ignored by core)
```

`kind` is `source`, `processor`, or `sink`. Module-specific keys such as `listen`
are read by the module binary; the core parser ignores unknown fields.

## Source module contract

The module process speaks RPC to the core:

- **Emit(event)** — stream events to the journal (sources)
- **Healthcheck()** — periodic liveness

Local modules use hashicorp/go-plugin; the core discovers the binary named
`module` from `manifest.toml` and manages the subprocess.

## Module-specific config

Broker addresses, topics, API tokens, and similar settings belong in the module's
own config (alongside or inside `manifest.toml`), not in the core TOML.

## Examples

| Module | Location | Planning page |
|--------|----------|---------------|
| HTTP ingest | `modules/http-ingest/` | [http-ingest](./planning/http-ingest.md) |
| MQTT | external | [mqtt-source](./planning/mqtt-source.md) |
| Home Assistant | external | [ha-source](./planning/ha-source.md) |

### HTTP ingest

After `make build`, add the repo `modules/` directory to `[modules].paths` and
start `trove`. POST JSON to `http://localhost:8080/ingest/shortcuts` (default
listen address). The `:source` path segment becomes the event `source` field;
optional `type` and `time` keys in the JSON body override event metadata.

## Publishing

No central registry in v0 — copy the module directory into a search path on the
host running Trove.
