---
title: Building modules
nav_order: 11
---

# Building modules

External modules extend Trove without recompiling the core. See [spec §8](../spec.md#8-module-architecture-dynamic-socket-based)
and [concepts/modules.md](./concepts/modules.md).

**Status:** Module runtime is **Planned** — [planning/module-runtime.md](./planning/module-runtime.md).

## Layout

Install under a configured search path:

```
~/.local/lib/trove/modules/my-source/
    manifest.toml
    my-source          # executable
```

## Manifest

```toml
name     = "my-source"
version  = "1.0"
kind     = "source"
provides = ["my-source.event.received"]
```

`kind` is `source`, `processor`, or `sink`.

## Source module contract

The module process speaks RPC to the core:

- **Emit(event)** — stream events to the journal (sources)
- **Healthcheck()** — periodic liveness

Local modules use hashicorp/go-plugin; the core discovers the binary from
`manifest.toml` and manages the subprocess.

## Module-specific config

Broker addresses, topics, API tokens, and similar settings belong in the module's
own config (alongside or inside `manifest.toml`), not in the core TOML.

## Examples (planned)

| Module | Planning page |
|--------|---------------|
| HTTP ingest | [http-ingest](./planning/http-ingest.md) |
| MQTT | [mqtt-source](./planning/mqtt-source.md) |
| Home Assistant | [ha-source](./planning/ha-source.md) |

## Publishing

No central registry in v0 — copy the module directory into a search path on the
host running Trove.
