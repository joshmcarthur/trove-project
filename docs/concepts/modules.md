---
title: Modules
parent: Concepts
nav_order: 5
---

# Modules

Modules are **separate processes**, discovered from predefined filesystem paths
and connected to the Trove core over a local (or networked) socket.

See [spec §8](../spec.md#8-module-architecture-dynamic-socket-based).

## Discovery paths

Checked at startup, in order:

```
/usr/lib/trove/modules/
/usr/local/lib/trove/modules/
~/.local/lib/trove/modules/
```

Each module is a directory:

```
module/
    manifest.toml
    module          # executable
```

```toml
name     = "mqtt-source"
version  = "1.0"
kind     = "source"        # source | processor | sink
provides = ["mqtt.message.received"]
```

## Local vs remote

| Path | Mechanism |
|------|-----------|
| Local | [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) — subprocess, gRPC, crash isolation |
| Remote (Tailscale) | Plain gRPC listener; edge device dials in — go-plugin does not support real networks |

Trove writes discovery (scan paths, read manifests); go-plugin handles launch
and transport once given a binary path.

## RPC surface

```
Source    : module streams Emit(event) to core
Processor : core calls Process(event) -> []event
Sink      : core calls Handle(event) -> ack
All kinds : core calls Healthcheck() periodically
```

At runtime today, only **source** modules are started. Processor and sink kinds
are accepted in manifests but not wired yet.

## Implementation

- [Module runtime](../planning/module-runtime.md) — milestone 1
- [Remote modules](../planning/remote-modules.md) — later
- [Building modules](../building-modules.md) — author guide
