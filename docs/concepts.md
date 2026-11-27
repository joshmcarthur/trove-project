# Concepts

Trove stores an append-only collection: facts are never updated in place,
originals are never modified, and meaning is layered on as claims and links.

## Items

An item is a stable identity: an original you ingested, or something a
processor emitted (a colour, a region, a type proposal). An original is
created by an ingest activity. An emitted item is created by a derive
activity and linked to its source with `derived_from`.

## Blobs

A blob is a content-addressed file: sha256, size, content type. Items point
at blobs through claims (for example `$blob`). Format-specific facts (width,
page count, duration) are claims on the item, not columns on `blobs`.

## Claims

A claim is one value at one path on one item, written by a processor or a
person, always inside an activity. A human correction is a preferred claim.
Output from processors and AI has normal rank. Deprecated claims are hidden.

## Links

A link is a relation between two items (`derived_from`, `fits`, and so on).
Links are asserted and retracted. An unlink is a new row whose `retracts`
points at the original. Processors follow the same rule as people.

## Activities

Every write happens inside an activity: ingest, derive, edit, decide, purge.
The activity records who or what acted, when, with which model and prompt,
at what cost, and how it ended. Activities are never purged. They are the
derivation state, the cost ledger, and the failure record.

## Rank

Conflicting values resolve by rank, then recency. Preferred beats normal;
deprecated is hidden. Within a rank, newest wins. A newer AI claim never
overrides a preferred human one. Rank is the archival rule that stops a
re-derive from stomping a correction.

## Plugins

Plugins are Django-side folders with a `manifest.json` and a `setup`
function, in the Home Assistant style. They declare types, processors, views,
and optional add-on processes. They read through the query layer and write
through the recorder only.

Ingest is the only core plugin; Trove will not start without it. Wall, palette, and agent are built-in plugins in version 1. MCP, embeddings, proposals, and Telegram are on the roadmap.

## Jobs

Processors run as derive jobs; derivation state lives in activities. When an
item is created, matching processors are submitted. Reconcile finds items
that still need a successful derive at the current processor version.
Transient errors retry; a permanent failure ends the activity as `failed`
and shows in the admin.
