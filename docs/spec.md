# Trove: A Personal Event & Attachment Store

## 1. Overview

Trove is a small, self-contained personal data store: a single binary that
captures typed events and attachments from the sources in your life —
Meshtastic, Home Assistant, MQTT, iOS Shortcuts, webhooks, and periodic
batch exports — into one durable, queryable journal, with a conversational
MCP interface as the primary way to get information back out.

The guiding idea:

> Capture broadly, store simply, converse to retrieve.

This is a narrower goal than a general "personal storage system for life"
(see §12, Non-Goals). Trove is not trying to be Perkeep, Huginn, or
LifeStream's replacement feature-for-feature — it's trying to be the
smallest thing that makes "remember things in one place, ask questions
about them" actually work, for one user, on one Pi.

---

## 2. Design Goals

**Primary goals**

- A small core binary, with modules loaded dynamically — from predefined
  filesystem locations and/or connecting in over a socket — rather than
  compiled in
- SQLite as the default and only required journal backend
- Simple, generic ingestion (MQTT, webhook, websocket) that covers most
  real sources with no source-specific core code
- A blob store with a Perkeep-shaped interface (`put`/`get`/`enumerate`),
  content-addressed, swappable backend
- MCP server over an internal RPC API as the primary query surface
- Fast to validate: a working, useful version buildable in a weekend

**Non-goals** (see §12 for detail)

- Multi-node / multi-journal sync or reconciliation
- A WASM module system or sandboxed guest runtime
- A central schema registry or formal schema evolution machinery
- Being a general-purpose content-addressable storage platform
- Replacing Postgres, Kafka, or MQTT itself

---

## 3. Core Concepts

An **event** is an immutable fact: something that happened, captured once,
never edited. If something changes, a new event is appended — nothing is
mutated in place.

```json
{
  "id": "01JXYZ...",
  "time": "2026-07-10T10:00:00+12:00",
  "type": "meshtastic.message.received",
  "source": "radio-node-1",
  "payload": { "...": "..." },
  "blob_ref": null
}
```

Fields:

| Field      | Type              | Notes                                             |
|------------|-------------------|----------------------------------------------------|
| `id`       | ULID              | sortable, unique, generated at ingest              |
| `time`     | RFC3339 timestamp | event time, not ingest time (source may supply it)|
| `type`     | string            | dotted namespace, e.g. `mqtt.tararuawx.temp`       |
| `source`   | string            | free-text origin identifier (topic, device, app)   |
| `payload`  | JSON              | arbitrary structured data, module-defined shape    |
| `blob_ref` | string \| null    | optional reference to an attachment (see §6)       |

There is **no central schema registry**. `type` is a namespaced string convention
(`<source-family>.<subject>.<verb>`), and `payload` is JSON defined by the source
module. Source modules declare which types they may emit in `manifest.toml`
(`provides`, including wildcard patterns such as `note.*`). The core enforces
that allowlist at the `Emit` RPC boundary. Modules may optionally attach JSON
Schema files per type pattern; when declared, payloads are validated before
append. The journal itself only validates the event envelope (type, source, valid
JSON). If a payload shape changes, use naming discipline (`.v2` suffixes) rather
than in-place mutation.

---

## 4. Journal

The journal is the single source of truth: an append-only table in SQLite.

```sql
CREATE TABLE events (
  id        TEXT PRIMARY KEY,
  time      TEXT NOT NULL,
  type      TEXT NOT NULL,
  source    TEXT NOT NULL,
  payload   TEXT NOT NULL,   -- JSON
  blob_ref  TEXT
);
CREATE INDEX idx_events_time ON events(time);
CREATE INDEX idx_events_type ON events(type);
CREATE INDEX idx_events_source ON events(source);
```

Interface (internal Go API, not exposed directly to modules):

```go
type Journal interface {
    Append(ctx context.Context, e Event) error
    Query(ctx context.Context, f Filter) ([]Event, error)
    Get(ctx context.Context, id string) (Event, error)
    Watch(ctx context.Context) (<-chan struct{}, error)
}
```

`Filter` supports type prefix, source, time range, and free-text match
(via SQLite FTS5) at minimum. `Watch` signals coalesced wakeups after
each append; consumers pull events via `Query` (the router uses a durable
cursor for guaranteed dispatch).

**Semantic search** (optional, add once basic search proves insufficient):
`sqlite-vec` as a virtual table alongside `events`, populated by an
embedding processor on write. Keeps everything in one SQLite file — no
separate vector database, no Postgres/pgvector dependency.

---

## 5. Blob Storage

Large content (photos, audio, printed documents, raw sensor dumps) is
never inlined in `payload` — it's stored separately and referenced by
`blob_ref`, following Perkeep's interface shape (without its permanode/
claims/signing machinery, which solves problems Trove doesn't have):

```go
type BlobStore interface {
    Put(ctx context.Context, data io.Reader) (ref string, err error)
    Get(ctx context.Context, ref string) (io.ReadCloser, error)
    Range(ctx context.Context, ref string, start, end int64) (io.ReadCloser, error)
    Enumerate(ctx context.Context) (<-chan string, error)
}
```

- `ref` is a content hash (`sha256-<hex>`), giving free deduplication and
  integrity checking.
- v0 backend: local filesystem, blobs stored by hash prefix
  (`/data/blobs/ab/cd/abcd1234...`).
- Backends worth adding later, matching Perkeep's proven list: S3-compatible
  (MinIO or real S3), Backblaze B2. Same interface, swap the implementation —
  no changes needed elsewhere.
- No sync/replication in v0. If you want off-Pi backup, that's a periodic
  `rclone`/`restic` job against the blob directory, not a Trove feature.

---

## 6. Sources

Sources are modules loaded dynamically at runtime — see §8 for the
loading and transport mechanism. Conceptually each implements:

```go
// Conceptual contract — realized as RPC calls over the module socket (§8),
// not an in-process Go interface.
type Source interface {
    Name() string
    Run(ctx context.Context, emit func(Event)) error
}
```

**v0 sources, in priority order:**

1. **Generic HTTP ingest** (`POST /ingest/:source`) — accepts arbitrary
   JSON, wraps it as an event with `source` from the URL path. This is
   the catch-all for iOS Shortcuts, webhooks, IFTTT, and anything else
   that can make an HTTP call. Build this first — it's the highest
   leverage for the least code, and it's what the two-hour validation
   version should be built around.
2. **MQTT listener** — subscribes to configured topics on your existing
   Mosquitto broker, wraps every message as an event. Covers Meshtastic
   (already bridged to MQTT) and any ESPHome/sensor traffic for free.
3. **Home Assistant WebSocket tap** — subscribes to `state_changed` on
   HA's `/api/websocket`, emits one event per state change.

**Later / as needed:**

- Batch importers (Google Takeout, photo folder watcher) as small
  standalone scripts that diff against a checkpoint and POST to the
  generic HTTP ingest — no need for these to be "real" Trove source
  modules, since they run occasionally, not continuously.

---

## 7. Processors and Sinks

**Processors** consume events and may emit derived events or write blobs.
Manifest fields:

- `consumes` — event types the processor subscribes to (exact or glob)
- `provides` — derived event types the processor may emit

**Sinks** consume events and take an action (thermal printer, notification).
They declare `consumes` only.

The core dispatches journal events to matching modules and passes a
`DispatchContext` with `root_id` and a `seen` list of module names already
handled in the chain. If a module sees itself in `seen`, it skips the event.
Derived events inherit `seen` so routing loops terminate.

Treat AI-derived events as one-shot facts unless model + prompt + version are
snapshotted alongside the output.

See [concepts/modules.md](./concepts/modules.md) for routing details.

---

## 8. Module Architecture: Dynamic, Socket-Based

Modules are **separate processes**, discovered from predefined filesystem
locations and connected to the Trove core over a local (or networked)
socket, speaking a small RPC protocol. This is a deliberate correction
from an earlier draft of this spec, which proposed compiling modules in
at build time — that traded away exactly the flexibility that matters
for a system where you're actively iterating on individual modules
(Meshtastic tuning, sensor calibration) far more often than on the core.

This also avoids two other options considered and rejected:

- **Go's native `plugin` package** — notoriously fragile across compiler
  versions and Linux-only; not worth the pain for what it buys.
- **A WASM host/guest ABI** — real complexity (sandboxing, a defined
  guest API) aimed at portability and safety guarantees a single trusted
  Pi doesn't need.

**Precedent**: this is the same shape as HashiCorp's `go-plugin` (used by
Terraform and Vault providers) and Docker's plugin system — a supervisor
process that spawns or accepts connections from plugin binaries over a
socket, speaking a fixed RPC protocol.

### Discovery

Module binaries live in predefined locations, checked in order at
startup (mirrors standard Linux module-path conventions):

```
/usr/lib/trove/modules/
/usr/local/lib/trove/modules/
~/.local/lib/trove/modules/
```

Each module is a directory containing a manifest and an executable:

```
module/
    manifest.toml
    module          # executable
```

```toml
name     = "mqtt-source"
version  = "1.0"
kind     = "source"        # source | processor | sink
provides = ["mqtt.*.received"]   # exact types or glob patterns; required for sources

[schemas]
"mqtt.*.received" = "schemas/message.json"   # optional JSON Schema per pattern
```

Source modules may only `Emit` types matching a `provides` pattern. Wildcards use
Go `path.Match` rules (`note.*` matches `note.created`). Bare `*` is rejected.
Optional `[schemas]` entries validate payloads when present; undeclared types are
allowlisted but not schema-checked. The journal validates the envelope only.

### Transport

Two distinct mechanisms, because they have genuinely different
requirements:

**Local modules** — use [`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin)
directly rather than hand-rolling process supervision and an RPC
protocol. It already provides: subprocess launch and lifecycle
management, a gRPC (or net/rpc) transport, and crash isolation (a
panicking plugin doesn't take down the core). This is exactly the
battle-tested version of what §8 originally proposed hand-rolling —
it's been in production use by Terraform, Vault, Nomad, and others for
years. Trove's own discovery layer (scanning the predefined module
paths, reading each `manifest.toml`) still needs to be written — go-plugin
starts once you hand it a binary path, it doesn't discover one for you.

**Remote modules** (e.g. a solar Meshtastic node in the Tararuas,
connecting in over Tailscale) — **cannot** use go-plugin for this: its
own documentation is explicit that it's designed only for a local,
reliable connection between a host and a subprocess it launched itself,
and real network use is unsupported and will misbehave. Remote modules
instead need a small, separate gRPC service that the core listens on
(over the tailnet), which the remote module dials into — same event/RPC
shapes as the local protocol, different transport and connection
direction. Worth keeping this as a genuinely distinct code path rather
than trying to force one mechanism to cover both.

### RPC surface (module ↔ core)

```
Source    : core receives a stream of Emit(event) calls from the module
Processor : core calls Process(event, DispatchContext) -> []event synchronously
Sink      : core calls Handle(event, DispatchContext) -> ack
All kinds : core calls Healthcheck() periodically
```

### Supervision

- The core watches the module directories (or reacts to `SIGHUP`) and can
  start, stop, or restart individual modules without a full core restart.
- A crashing module must not take down the core — supervise with restart
  and backoff, log failures, keep serving from the journal regardless.

### Tradeoffs, stated honestly

Compared to compiling modules in, this costs: process supervision, socket
lifecycle management, and an actual RPC protocol to design, run, and
version. It buys: independent restart/redeploy of any single module
(useful given how often the Meshtastic/sensor side gets tinkered with),
and a clean, already-solved path for edge devices to stream events home
over Tailscale without full sync machinery. Given your stated iteration
pattern, that trade is worth it here — it wouldn't necessarily be for a
system with a more static set of sources.

---

## 9. Query Interface: MCP over RPC

The primary way you interact with Trove is conversational, via an MCP
server. The MCP server is a thin wrapper — all real logic lives in an
internal RPC API that the MCP tools call, so a future web dashboard or
OpenClaw integration can hit the same RPC directly without depending on
MCP.

**RPC API (internal):**

```
search_events(query string, type_prefix?, source?, time_range?) -> []Event
get_events_by_type(type string, time_range) -> []Event
get_event(id string) -> Event  // resolves blob_ref if present
summarize_range(time_range) -> Summary  // pre-aggregated: counts by type, notable events
```

**MCP tools** map 1:1 onto these, deliberately narrow and typed rather
than one generic "run a query" tool — this gives the model structured,
predictable results instead of raw SQL access, and keeps token usage
sane (`summarize_range` exists specifically so "how was my week" doesn't
dump thousands of raw rows into context).

`search_events` should support fuzzy/semantic matching via the
`sqlite-vec` index (§4) once that exists — until then, FTS5 keyword
search is a fine placeholder.

**OpenClaw's role**: becomes a client of this MCP server (plus optionally
its own Source registered against the generic HTTP ingest for
Telegram-captured notes), rather than needing direct database access.

---

## 10. Configuration

TOML, not YAML — matches the stance in earlier drafts.

```toml
[journal]
path = "/data/trove.db"

[blobs]
backend = "filesystem"
path = "/data/blobs"

[modules]
paths = ["/usr/local/lib/trove/modules", "~/.local/lib/trove/modules"]

[modules.remote]
listen = "tailscale:trove"   # planned: accept remote module connections over the tailnet

[http]
listen = ":8080"
max_body_bytes = 10485760
```

MCP query is served by the `mcp-query` module at `POST /mcp` on the HTTP gateway
(same listen address as ingest). There is no separate `[mcp]` section in core
config — see [configuration](./getting-started/configuration.md).

Individual module configuration (broker addresses, topics, tokens) lives
in each module's own manifest/config, not the core config — the core
shouldn't need to know the shape of a module's settings.

---

## 11. Build Order / Validation Plan

Don't build the full module ecosystem before knowing this is worth having.
In order:

1. **SQLite journal + a minimal module-loading core.** The `events` table,
   go-plugin gRPC IPC, and one built-in-for-now module: generic HTTP ingest
   (`POST /ingest/:source`). Even in v0, keep ingest behind the module socket
   rather than wiring it directly into the core — it's the cheapest way to prove
   the module boundary actually works before more modules depend on it.
2. **One real hardware-adjacent source module** — MQTT is highest-leverage
   since Mosquitto and Meshtastic-to-MQTT already exist.
3. **A minimal MCP server** wrapping `search_events` and `get_event`
   against that SQLite file. This is the part that's historically been
   the actual failure point for prior art (Perkeep, MyLifeBits) — capture
   was never the hard part, conversational retrieval was. Validate this
   specifically, not just the capture side.
4. **Live with it for two weeks.** Use the iOS Shortcuts share-sheet
   capture (POST to generic ingest) as your manual catch-all during this
   period. See if you actually query it.
5. Only after that: decide whether blob storage, the HA tap, embeddings,
   the remote/Tailscale module transport, processors, or sinks are worth
   adding — informed by what you actually asked it and what was missing,
   not by speculation.

---

## 12. Non-Goals (Explicit)

These were considered and deliberately excluded, per earlier discussion —
noted here so they don't get silently re-added:

- **Multi-journal / multi-node sync.** Not solving edge-node
  (Tararua Meshtastic node) ↔ home-journal reconciliation as a general
  problem. Remote modules can stream events to the one central journal
  over Tailscale (§8), but there's still only one journal, not several
  that need reconciling.
- **Central schema registry / formal schema evolution.** Per-module `provides`
  allowlists and optional colocated JSON Schema files are in scope; a shared
  registry service is not.
- **A WASM guest runtime or module manifest discovery beyond the simple
  filesystem-path convention in §8.** Dynamic loading is in scope;
  sandboxed guest execution is not.
- **Perkeep-style content model (permanodes, claims, GPG signing).**
  Evaluated and rejected — the ingest friction (signing requirement) and
  thin structured-query support weren't worth it against a from-scratch
  typed event index. Perkeep's blob-store *interface shape* was kept
  (§5); its object model was not.
- **Being a general "store anything for anyone" platform.** This is a
  single-user tool built around your specific sources.

---

## 13. Open Items (Not Yet Decided)

Flagging honestly rather than guessing, since you noted the spec was
incomplete — these need a decision but aren't blocking §11's build order:

- **RPC protocol for the remote/edge module path** — local modules now
  use go-plugin's gRPC transport (decided, §8); the separate remote
  listener for Tailscale-connected edge modules still needs its protocol
  shape settled (likely the same protobuf definitions, served over a
  plain gRPC listener rather than go-plugin).
- **Blob store backend priority**: local filesystem is specified for v0;
  whether S3-compatible or B2 comes next depends on whether you actually
  want off-Pi backup, which wasn't settled in discussion.
- **Embedding model choice** for `sqlite-vec` — local (e.g. a small
  sentence-transformer via ONNX) vs. calling out to OpenRouter/Qwen like
  OpenClaw does. Affects whether Trove has any external network
  dependency at all.
- **Auth model for the HTTP ingest and MCP endpoints** — Tailscale-only
  (matching your existing pattern for OpenClaw) is the obvious default
  but wasn't explicitly confirmed.
- **Retention / pruning policy**, if any — journal is append-only by
  design, but disk isn't infinite; not discussed whether old raw sensor
  events ever get rolled up or archived.
- **Whether `summarize_range` pre-aggregates at write time or query
  time** — the AI-non-determinism concern in §7 applies here too if
  summaries get cached/stored rather than generated fresh per query.

---

## Appendix: What Changed From the Original "Eventd" Draft

- Renamed Eventd → **Trove**.
- Dropped WASM extension model and replaced it with a socket-based
  dynamic module system: local modules load via `hashicorp/go-plugin`
  (subprocess launch, gRPC transport, crash isolation — all handled by
  the library rather than hand-rolled), discovered from predefined
  filesystem locations with a manifest; remote/edge modules (e.g. a
  Tararua Meshtastic node over Tailscale) use a separate plain gRPC
  listener, since go-plugin explicitly doesn't support real-network
  connections. An interim draft of this spec proposed precompiled,
  build-time-only modules instead — that was a mistake given how often
  individual modules (Meshtastic tuning, sensor calibration) get
  iterated on independently of the core; this version corrects back to
  dynamic loading while keeping the manifest/discovery-path shape from
  the very first draft.
- Dropped formal schema registry (protobuf/CBOR descriptors) in favor of
  namespaced `type` strings and plain JSON payloads.
- Made the MCP-over-RPC query interface a first-class, load-bearing part
  of the spec (§9) rather than an afterthought — this is the piece most
  directly informed by understanding *why* prior art (Perkeep, MyLifeBits)
  stalled.
- Added an explicit blob storage interface modeled on Perkeep's
  `put/get/enumerate`, without adopting Perkeep itself.
- Added §11 (build order) and §13 (open items) to keep the spec honest
  about what's actually decided vs. still open.