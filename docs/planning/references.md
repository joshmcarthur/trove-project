---
title: References and URIs
parent: Planning
nav_order: 16
---

# References and URIs

**Status:** Planned\
**Spec:** [Core concepts §3](../spec.md#3-core-concepts), [Records concept](../concepts/records.md)\
**Packages:** `internal/journal`, `internal/records`, `internal/modules`

## Goal

Extend Trove from a typed revision log into a **personal content graph**: stable
**records** as nodes, **references** as directed edges, **blobs** as immutable
bytes, and **revisions** as the append-only audit trail modules use to cooperate.

Capture stays minimal (“share anything”); modules enrich, classify, and connect
later without rewriting history.

## Model

| Concept | Role |
|---------|------|
| **Record** | Stable node for something shared or kept (`record_ref`) |
| **Revision** | Immutable claim that changes a record (inter-module protocol) |
| **Reference** | Directed edge `{ ref, rel? }` to any absolute URI |
| **Blob** | Immutable bytes at `trove://blob/sha256-...` |

The journal plus blob store is the source of truth. SQLite tables (`record_heads`,
`records_fts`, future link indexes) are **projections** — wipe and replay revisions
to rebuild.

## URI grammar

| URI | Meaning |
|-----|---------|
| `trove://type/{path}/{version}` | Schema contract (type catalog) |
| `trove://record/{ulid}` | Record identity |
| `trove://revision/{ulid}` | Audit / provenance pointer |
| `trove://blob/sha256-...` | Content-addressed attachment |
| `https://...`, `mailto:...`, etc. | External refs allowed |

Edges use **bare URIs** in `ref`. Optional `rel` names the relationship when the
default (plain link) is not enough. There is no `trove://link/rel/...` path encoding.

### Reference tuple

```json
{ "ref": "trove://record/01JREC...", "rel": "mentions" }
{ "ref": "trove://blob/sha256-abc...", "rel": "cover" }
{ "ref": "https://example.com/article" }
```

| Field | Required | Notes |
|-------|----------|-------|
| `ref` | yes | Absolute URI |
| `rel` | no | Relationship name when typed edges matter |

**Attachments** are blob references on the record head’s `references` list (or via
`rel` such as `cover`, `attachment`). Caption and other metadata live in the typed
**body**, keyed by ref URI — not on the reference tuple in v1.

**Primary content** may continue to use `content_ref` / `blob_ref` on revisions for
backward compatibility; new work should prefer `references` for multiple attachments.

## Revision operations

### Supported today

| Op | Body | References |
|----|------|------------|
| `apply` | Merge payload → body (RFC 7396); transforms (RFC 6902) | Unchanged (no `references` field) |
| `delete` | `{}` | Tombstone; body retained; references policy TBD at delete |

### Planned

| Op | Body | References |
|----|------|------------|
| `apply` (no `references` field) | Merge payload | Unchanged |
| `apply` (`references: []`) | Merge payload | **Clear all** |
| `apply` (`references: [...]`) | Merge payload | **Full copy replace** |
| `link` | `{}` or `{ "references": [...] }` only | **Union add** (dedupe by `ref` + `rel`) |
| `unlink` | same | **Subtract** matching pairs |
| `delete` | `{}` | Tombstone (references on head TBD) |

`link` and `unlink` do **not** merge domain payload — only the `references` list
on the revision matters (body field is empty or omitted).

### Unlink matching

Subtract edges where `ref` matches. If `rel` is present on the unlink tuple, match
exact `(ref, rel)`. If `rel` is omitted, remove **all** edges with that `ref`
regardless of `rel`.

## Provenance fields

| Field | Set by | Meaning |
|-------|--------|---------|
| `producer` | **Host** (authenticated module identity, e.g. `module.http-ingest`) | Which Trove module wrote this revision |
| `source` | Caller / module | External origin (topic, chat, device, URL channel) |
| `caused_by` | Optional future | `trove://revision/...` derivation chain |

Modules must **not** set `producer`; the host stamps it at the `AppendRevision`
boundary. `source` remains required and free-text as today.

## Module persistence rules

When a processor enriches data:

| Situation | Action |
|-----------|--------|
| Same shared thing, more fields or links | `apply` or `link` on the **same** `record_ref` |
| Distinct identity (person, author, derived document) | New `apply` (new record) + `link` from parent |
| Rebuildable index (FTS, embeddings, link index) | Projection only — do not duplicate as records |
| Many parallel enrichers on one capture | Requires idempotency keys (planned) |

Trove is an **active** module ecosystem: revisions are how modules cooperate.
Passive blob-and-claim stores (e.g. Perkeep) optimize for user-signed claims;
Trove optimizes for capture-first workflows and later connection.

## Blob retention

Referenced blobs are retained while any record head references them. `unlink` does
not delete bytes. Explicit keep/pin and garbage collection are deferred.

## Replay ordering (prerequisite)

Today’s materializer replays by revision `time`, which is **event time** from the
source. That is insufficient when multiple revisions for one record arrive out of
wall-clock order.

Before references land in production:

- Add host **`recorded_at`** (commit time) and per-record **`sequence`**
- Replay by `sequence`, not source event time
- Optional **`expected_version`** on full-copy `apply` (including `references`
  replace) to detect lost updates

See [open-items](../open-items.md).

## Acceptance criteria

- [ ] `references` JSON column on revisions; folded `references` on `record_heads`
- [ ] `link` / `unlink` operations validated at append boundary
- [ ] `apply` with `references` field: omit = unchanged, `[]` = clear, list = replace
- [x] Host stamps `producer`; modules cannot override
- [x] `recorded_at` + per-record `sequence`; materializer replays by sequence
- [ ] Projections rebuild identically after `trove records rebuild`
- [ ] Blob refs in `references` participate in retention (no GC of pinned blobs)

## Dependencies

- **Blocks:** graph traversal MCP tools, multi-attachment records
- **Blocked by:** replay ordering (`sequence` / `recorded_at`)

## See also

- [Records concept](../concepts/records.md)
- [Revisions concept](../concepts/revisions.md)
- [Revision rename](./revision-rename.md)
- [Records layer](./records.md)
