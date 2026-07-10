---
title: Blobs
parent: Concepts
nav_order: 3
---

# Blob storage

Large content — photos, audio, raw sensor dumps — is never inlined in
`payload`. It is stored separately and referenced by `blob_ref` on the event.

See [spec §5](../spec.md#5-blob-storage).

## Interface

```go
type BlobStore interface {
    Put(ctx context.Context, data io.Reader) (ref string, err error)
    Get(ctx context.Context, ref string) (io.ReadCloser, error)
    Range(ctx context.Context, ref string, start, end int64) (io.ReadCloser, error)
    Enumerate(ctx context.Context) (<-chan string, error)
}
```

- `ref` is a content hash (`sha256-<hex>`) for deduplication and integrity.
- v0 backend: local filesystem with hash-prefix directories.
- Later: S3-compatible, B2 — same interface, swapped implementation.

No sync/replication in v0; off-Pi backup is an external `rclone`/`restic` job.

Primary use case: photo/attachment capture via iOS Shortcuts share sheet.
Upload bytes with `PUT /blobs` on http-ingest, then reference the returned
`blob_ref` on an ingest event.

## Implementation

**Status:** Supported — [planning/blobs.md](../planning/blobs.md)
