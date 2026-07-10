---
title: Blob store
parent: Planning
nav_order: 7
---

# Blob store

**Status:** Planned\
**Milestone:** 2b — alongside MQTT and live-test prep\
**Spec:** [Blob storage §5](../spec.md#5-blob-storage)\
**Package:** `internal/blob`

## Goal

Content-addressed storage for large attachments, referenced from events via
`blob_ref`. Primary use case: **iOS Shortcuts share-sheet photo capture** —
upload image bytes, store content-addressed, set `blob_ref` on the journal event
(not inline in `payload`).

## Interfaces

```go
type BlobStore interface {
    Put(ctx context.Context, data io.Reader) (ref string, err error)
    Get(ctx context.Context, ref string) (io.ReadCloser, error)
    Range(ctx context.Context, ref string, start, end int64) (io.ReadCloser, error)
    Enumerate(ctx context.Context) (<-chan string, error)
}
```

## Implementation notes

- v0 backend: filesystem with hash-prefix layout (`/data/blobs/ab/cd/abcd...`)
- `ref` format: `sha256-<hex>`
- Implement `BlobStore` in `internal/blob` and wire `[blobs]` config in
  `cmd/trove/main.go`
- Add HTTP blob upload to http-ingest module: `PUT /blobs` returning
  `{ "blob_ref": "sha256-..." }` (dedicated endpoint, not multipart on ingest)
- Update share-sheet Shortcut docs to upload photo then POST event with
  `blob_ref`
- `blob_ref` is already accepted by journal and HTTP ingest today; blob bytes
  are not stored or resolved until this lands
- `get_event` MCP tool: optionally include blob metadata or serve URL once blobs
  exist (follow-up acceptance criterion)

## Acceptance criteria

- [ ] `Put`/`Get` round-trip on filesystem backend
- [ ] Put returns stable ref for same content (dedup)
- [ ] Range supports partial reads
- [ ] Enumerate lists all refs
- [ ] `PUT /blobs` on http-ingest returns stable `blob_ref`
- [ ] Event with `blob_ref` references retrievable bytes
- [ ] Share-sheet Shortcut recipe documents photo upload path

## Dependencies

- **Blocks:** photo/attachment capture via iOS Shortcuts
- **Blocked by:** config loader (`[blobs]` section parsed but not wired)

## Open questions

- S3 vs B2 priority — [open-items.md](../open-items.md)
