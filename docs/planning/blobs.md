---
title: Blob store
parent: Planning
nav_order: 7
---

# Blob store

**Status:** Later\
**Milestone:** After two-week live test\
**Spec:** [Blob storage §5](../spec.md#5-blob-storage)\
**Package:** `internal/blob`

## Goal

Content-addressed storage for large attachments, referenced from events via
`blob_ref`.

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
- Wire into config `[blobs]` section

## Acceptance criteria

- [ ] Put returns stable ref for same content (dedup)
- [ ] Get round-trips bytes
- [ ] Range supports partial reads
- [ ] Enumerate lists all refs

## Dependencies

- **Blocks:** events with attachments
- **Blocked by:** config, decision to need blobs post live-test

## Open questions

- S3 vs B2 priority — [open-items.md](../open-items.md)
