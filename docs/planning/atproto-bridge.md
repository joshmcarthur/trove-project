---
title: AT Protocol bridge
parent: Planning
nav_order: 17
---

# AT Protocol bridge

**Status:** Planned\
**Milestone:** Later (post live-test)\
**Spec:** [Modules §8](../spec.md#8-module-architecture-dynamic-socket-based), [References](./references.md)\
**Package:** `modules/atproto-bridge`

## Goal

Bidirectional **bridge** between AT Protocol and the Trove life repo:

- **Pull** — ingest records from AT hosts (relay firehose, upstream repo sync, on-demand
  fetch) into the journal, filtered by wildcard allowlists
- **Push** — emit selected Trove revisions to AT Protocol (local [PDS](./atproto-pds.md) or
  upstream host), filtered by wildcard allowlists

The bridge does **not** replace `atproto-pds`. The PDS module owns your federated repo
(signing, firehose out, XRPC serve). The bridge owns **cross-boundary traffic** with
explicit, configurable filters on both sides.

## Architecture

```mermaid
flowchart TB
  subgraph atNet [AT Protocol network]
    relay[Relay firehose]
    upstream[Upstream PDS]
    others[Other repos]
  end

  subgraph bridge [atproto-bridge module]
    pull[Pull - Run loop]
    push[Push - Handle sink]
    pullFilter[pull allowlists]
    pushFilter[push allowlists]
  end

  subgraph trove [Trove core]
    journal[(Journal)]
    router[Revision router]
    blobs[(Blobs)]
    mcp[MCP query]
  end

  subgraph pds [atproto-pds - optional peer]
    repo[Signed repo]
  end

  relay --> pull
  upstream --> pull
  others --> pull
  pull --> pullFilter --> journal
  router --> pushFilter --> push
  push -->|allowlisted| pds
  push -->|or| upstream
  pds --> repo
  journal --> mcp
```

| Direction | Module hook | Allowlist keys |
|-----------|-------------|----------------|
| Pull | `Run()` — firehose / sync / fetch loops | `repos`, `collections` |
| Push | `Handle()` — revision router sink | `types`, `collections` (target) |

## Canonical journal envelope

Pulled records and push confirmations both use one type:

```
trove://type/atproto/record/stored/1
```

| Field | Pull | Push |
|-------|------|------|
| `source` | Remote or own DID | Own DID |
| `payload.at_uri` | `at://{did}/{collection}/{rkey}` | Same after write |
| `payload.collection` | NSID | NSID |
| `payload.rkey` | From repo | Assigned on create |
| `payload.cid` | From repo | From write response |
| `payload.value` | Lexicon JSON | Lexicon JSON (translated from Trove body) |
| `payload.direction` | `pulled` | `pushed` |
| `payload.origin` | `firehose` \| `sync` \| `fetch` | `sink` |
| `references` | `at://` edges + optional `trove://record/...` link back | `{ ref: trove://record/..., rel: trove_origin }` |

Dedupe on `(at_uri, cid)` for pulls; on `(trove_origin_record_ref, at_uri)` for push
confirmations.

## Allowlists

Both sides use **glob patterns** (Go `path.Match` semantics, same spirit as manifest
`consumes` / `provides`). Empty list = deny all (fail closed). No bare `*`.

### Pull allowlists

Filter **before** `AppendRevision`.

```toml
[bridge.pull]
# Whose repos to accept (DID globs)
repos = [
  "did:plc:myself",              # exact
  "did:plc:followed-*",          # glob
]

# Which lexicon collections (NSID globs)
collections = [
  "app.bsky.feed.post",
  "app.bsky.actor.profile",
  "app.bsky.graph.*",
]

# Optional: deny overrides (evaluated after allow; first match wins deny)
# deny_repos = []
# deny_collections = ["app.bsky.graph.block"]
```

| Pattern example | Matches |
|-----------------|---------|
| `did:plc:abc123` | Exact DID |
| `did:plc:followed-*` | DID prefix glob |
| `app.bsky.feed.post` | Exact NSID |
| `app.bsky.graph.*` | `app.bsky.graph.follow`, `app.bsky.graph.block`, … |
| `app.bsky.*` | Any `app.bsky` collection |

**Pull sources** (independent toggles):

```toml
[bridge.pull.firehose]
enabled = true
relay = "wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos"
cursor = "/var/lib/trove/atproto-bridge-firehose.cursor"

[bridge.pull.sync]
enabled = false
repos = ["did:plc:myself"]        # periodic getRepo from each
pds_hosts = ["https://bsky.social"]
interval = "1h"

[bridge.pull.fetch]
enabled = true                    # on-demand when Trove references at://...
```

Firehose delivers all network commits — **allowlists are mandatory** to avoid filling
the journal with the entire relay stream.

### Push allowlists

Filter in `Handle()` **before** AT write.

```toml
[bridge.push]
# Which Trove revision types may be pushed (trove:// globs)
types = [
  "trove://type/note/*",
  "trove://type/shortcuts/share/saved/1",
]

# Only these journal operations (default: apply only)
operations = ["apply"]

# Target AT collection per type (first matching glob wins)
[[bridge.push.map]]
types = ["trove://type/note/*"]
collection = "app.bsky.feed.post"

[[bridge.push.map]]
types = ["trove://type/shortcuts/share/saved/1"]
collection = "app.bsky.feed.post"

# Fallback when no map entry matches
default_collection = "app.bsky.feed.post"

# Optional deny
# deny_types = ["trove://type/mqtt/*"]
```

Manifest `consumes` must be a **superset** of `bridge.push.types` so the revision router
delivers matching revisions:

```toml
name     = "atproto-bridge"
version  = "0.1.0"
kind     = "processor"
consumes = ["trove://type/note/*", "trove://type/shortcuts/share/saved/1"]
provides = ["trove://type/atproto/record/stored/1"]
```

### Push target

```toml
[bridge.push.target]
# "local" — write via atproto-pds repo (recommended)
# "upstream" — XRPC to external PDS with configured session
mode = "local"

[bridge.push.target.local]
# in-process repo handle or RPC to sibling module

[bridge.push.target.upstream]
pds_url = "https://bsky.social"
# session via app password or OAuth token file — see open questions
```

## Module shape

```toml
name     = "atproto-bridge"
version  = "0.1.0"
kind     = "processor"

consumes = ["trove://type/note/*", "trove://type/shortcuts/*"]
provides = ["trove://type/atproto/record/stored/1"]

[[types]]
name    = "atproto.record.stored"
version = 1
schema  = "types/atproto.record.stored.ttd.json"
```

No `[[http.routes]]` — the bridge is a router participant, not an XRPC server. HTTP
surface stays on [atproto-pds](./atproto-pds.md) when federation is needed.

## Pull paths (detail)

### Firehose

1. WebSocket `com.atproto.sync.subscribeRepos` on configured relay
2. For each commit op: extract `(did, collection, rkey)`
3. Match `repos` + `collections` allowlists → skip if no match
4. Fetch full record if needed (`getRecord`) → translate to envelope → `AppendRevision`
5. Persist cursor; resume on restart (at-least-once; dedupe on `(at_uri, cid)`)

### Repo sync

1. On interval (or startup): `com.atproto.sync.getRepo` per `pull.sync.repos`
2. Walk CAR / commit ops; same allowlist filter
3. Useful for **migration** and backfill without live firehose

### On-demand fetch

1. When a Trove revision references an `at://` URI (link, embed, manual capture)
2. If not already in journal index → `getRecord` from resolved PDS
3. Same allowlist filter — external refs to non-allowlisted repos are not pulled

## Push paths (detail)

### Translation

Each allowlisted Trove revision maps to a lexicon record:

| Trove type | AT collection | Notes |
|------------|---------------|-------|
| `trove://type/note/quick/1` | `app.bsky.feed.post` | `text` ← body.text |
| `trove://type/shortcuts/share/saved/1` | `app.bsky.feed.post` | embed URL from payload |

Translation tables live in module config or small built-in defaults; custom
`[[bridge.push.map]]` entries override.

### Write + confirm

1. `Handle(revision)` — check `types`, `operations` allowlists
2. Translate body → lexicon `value`
3. `createRecord` / `putRecord` on push target
4. Append confirmation envelope with `direction: pushed`, `trove_origin` reference
5. Idempotent: if `(record_ref, collection)` already pushed with same body hash, skip

### Loop prevention

Push confirmation envelopes use `provides` type `atproto/record/stored` — ensure
`consumes` does **not** match that type, or push filter excludes
`payload.direction == pushed`.

## Relationship to `atproto-pds`

| Concern | Owner |
|---------|-------|
| Signing, DID, MST, XRPC serve, firehose **out** | `atproto-pds` |
| Pull from network, push from Trove, allowlists | `atproto-bridge` |
| Journal mirror of AT records | Both may append envelopes; dedupe on `at_uri` |

Typical deployment: run **both** modules. Bridge `push.target.mode = local` writes into
the PDS repo; PDS mirrors to journal; bridge pull also appends external records the PDS
does not own.

## Implementation phases

### Phase 1 — Push with allowlists

- Processor sink: `Handle()` + `bridge.push.types` filtering
- Translate `note/*` → `app.bsky.feed.post`
- Write to local PDS or upstream XRPC
- Confirmation envelope in journal

### Phase 2 — Pull with allowlists

- `Run()` firehose consumer + cursor
- `repos` + `collections` filters
- Envelope append + blob fetch for embeds

### Phase 3 — Sync + fetch

- Periodic `getRepo` backfill
- On-demand fetch for referenced `at://` URIs
- Deny lists, per-type push maps, metrics/healthcheck

## Acceptance criteria

### Phase 1 (push)

- [ ] `bridge.push.types` globs filter revisions; non-matching skipped
- [ ] Allowlisted note creates `app.bsky.feed.post` on target
- [ ] Confirmation envelope in journal with `trove_origin` reference
- [ ] Re-delivery of same revision does not duplicate AT record
- [ ] `operations = ["apply"]` excludes deletes

### Phase 2 (pull)

- [ ] Firehose cursor resumes without duplicate envelopes
- [ ] `repos` + `collections` allowlists enforced; out-of-scope commits ignored
- [ ] Pulled post searchable via MCP alongside Trove-native records
- [ ] Empty allowlist denies all (fail closed)

### Phase 3

- [ ] `getRepo` sync backfills allowlisted records
- [ ] On-demand fetch for referenced `at://` when allowlisted
- [ ] `deny_*` overrides work

## Dependencies

- **Blocked by:** records layer, revision router, type catalog, blob store
- **Soft:** [atproto-pds](./atproto-pds.md) for `push.target.mode = local` and federation
- **External:** indigo client libraries for XRPC + CAR/firehose parsing
- **Blocks:** nothing in v0 milestone sequence

## Open questions

| Question | Recommendation |
|----------|----------------|
| One module vs split ingest/emit | Single `atproto-bridge` processor |
| Allowlist in manifest vs config only | Config for pull/push lists; manifest `consumes` mirrors push types |
| NSID glob implementation | Reuse `path.Match`; document examples in getting-started |
| Upstream push auth | App password file for v1; OAuth later |
| Pull blob media | Fetch via `getBlob` → `core.Put` when record references blobs |

When resolved, move decisions to [open-items.md](../open-items.md).

## See also

- [AT Protocol PDS module](./atproto-pds.md) — federated repo host (peer, not replacement)
- [Processors and sinks](./processors-sinks.md) — revision routing
- [References](./references.md) — `at://` and `trove://` edges
