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

For local development, `make build` also builds the first-party HTTP ingest,
MQTT source, and MCP query modules into `modules/http-ingest/`,
`modules/mqtt-source/`, and `modules/mcp-query/`.
Point `[modules].paths` at the repo `modules/` directory (the parent of each
module folder).

## Manifest

```toml
name     = "my-source"
version  = "1.0"
kind     = "source"
provides = ["my-source.event.received", "my-source.*"]

[schemas]
"my-source.event.received" = "schemas/received.json"
```

Event-routing processor example:

```toml
name     = "embedder"
version  = "0.1.0"
kind     = "processor"
consumes = ["note.*"]
provides = ["note.embedding.generated"]
```

Sink example:

```toml
name     = "webhook-sink"
version  = "2.0"
kind     = "sink"
consumes = ["note.created"]
```

`kind` is `source`, `processor`, or `sink`.

| Kind | `provides` | `consumes` |
|------|------------|------------|
| `source` | **required** | forbidden |
| `processor` (event-routing) | required when emitting derived events | **required** |
| `processor` (HTTP-only) | forbidden | forbidden; use `[[http.routes]]` |
| `sink` | forbidden | **required** |

Each pattern is an exact event type or a glob (`note.*`, `mqtt.*.received`).
Bare `*` is not allowed. Per-module `listen` addresses are rejected — register
`[[http.routes]]` and let the core [HTTP gateway](./planning/http-gateway.md)
listen instead.

Optional `[schemas]` maps a type or pattern to a JSON Schema file (relative to
the module directory). When present, the core validates the payload before append.
Types without a schema entry are allowlisted but not payload-validated.

Module-specific keys such as `listen` are read by the module binary; the core
parser ignores unknown fields.

## Module contract

Modules run as subprocesses of the single `trove` process. When the parent
starts a module, it passes a **Core** handle — your connection back to the
parent for journal writes, blob storage, and journal reads:

```go
func (m *myModule) Run(ctx context.Context, core trovemodule.Core) error {
    return core.Emit(ctx, &troverpc.Event{ ... })
}
```

Use `trovemodule.Serve` to register the module. Optional interfaces:

- **HTTPHandler** — serve HTTP routes declared in the manifest
- **MCPToolHandler** — handle MCP tools declared in `[[mcp.tools]]`
- **EventProcessor** — `Process(event, dispatch)` for event-routing processors
- **EventSink** — `Handle(event, dispatch)` for sinks
- **HealthChecker** — report liveness to the parent

Event-routing processors and sinks implement `Run` with `trovemodule.WaitCore`
when they do not stream from `Run` themselves. The parent passes a
`DispatchContext` with `root_id` and `seen` module names for loop prevention.

The parent enforces ingest policy on `core.Emit` and on derived events returned
from `Process`. Modules do not open `trove.db` or the blob directory directly.

## Module-specific config

Broker addresses, topics, API tokens, and similar settings belong in the module's
own config (alongside or inside `manifest.toml`), not in the core TOML.

## Examples

| Module | Location | Planning page |
|--------|----------|---------------|
| HTTP ingest | `modules/http-ingest/` | [http-ingest](./planning/http-ingest.md) |
| Capture classifier | `modules/capture-classifier/` | [deferred-capture](./planning/deferred-capture.md) |
| MCP query | `modules/mcp-query/` | [mcp-query](./planning/mcp-query.md) |
| MQTT source | `modules/mqtt-source/` | [mqtt-source](./planning/mqtt-source.md) |
| Home Assistant | external | [ha-source](./planning/ha-source.md) |

### HTTP ingest

After `make build`, add the repo `modules/` directory to `[modules].paths` and
start `trove`. POST JSON to `http://localhost:8080/ingest/shortcuts` (default
listen address). The `:source` path segment becomes the event `source` field;
optional `type`, `time`, and `blob_ref` keys in the JSON body override event
metadata. Default request body limit is 10 MiB (`max_body_bytes` in manifest).
Allowed client `type` values are controlled by `provides` in the module manifest
(for example `note.*` for Shortcuts). Disallowed types and schema validation
failures return **400 Bad Request** with an error message.

For large attachments, do not inline bytes in JSON. Once the blob store is
implemented, upload content separately and reference it with `blob_ref` on the
ingest payload.

See [iOS Shortcuts](../getting-started/ios-shortcuts.md) for importable capture
Shortcuts that POST to this endpoint.

### MQTT source

After `make build`, configure broker and topics in
`modules/mqtt-source/manifest.toml` (default broker `tcp://localhost:1883`,
topics `["home/#"]`). Add `modules/` to `[modules].paths` and start `trove`.
Each MQTT message becomes a journal event with `type`
`mqtt.<topic>.received` (slashes become dots), `source` set to the topic, and
`payload.metadata.topic` preserving the original MQTT topic, with the MQTT
body in `payload.message` (JSON) or `payload.raw` (non-JSON).

## Publishing

No central registry in v0 — copy the module directory into a search path on the
host running Trove.
