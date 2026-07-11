---
title: iOS Shortcuts
parent: Getting started
nav_order: 4
---

# iOS Shortcuts

Capture events from your iPhone by POSTing JSON to Trove's
[HTTP ingest](../planning/http-ingest.md) module. Shortcuts are a **client** of
the generic ingest endpoint — not a Trove module.

## Prerequisites

1. Build Trove and configure `[modules].paths` — see [Quick Start](./quick-start.md).
2. HTTP gateway listening on `[http].listen` (default `:8080` in core `trove.toml`).
   Ingest routes are declared in the http-ingest module manifest.
3. Your phone can reach the ingest URL (local network, Tailscale, or HTTPS on the
   public internet).

## Import a Shortcut

Signed `.shortcut` files live in
[`examples/ios-shortcuts/signed/`](https://github.com/joshmcarthur/trove/tree/main/examples/ios-shortcuts/signed).
A maintainer signs them on a **Mac with iCloud signed in** and commits them to the
repo (hosted CI cannot sign).

| Shortcut | Use |
|----------|-----|
| [Trove Share Sheet](https://github.com/joshmcarthur/trove/blob/main/examples/ios-shortcuts/signed/trove-share-sheet.shortcut) | Share URLs, text, or images from any app |
| [Trove Quick Note](https://github.com/joshmcarthur/trove/blob/main/examples/ios-shortcuts/signed/trove-quick-note.shortcut) | Dictate or type a quick note |
| [Trove URL Bookmark](https://github.com/joshmcarthur/trove/blob/main/examples/ios-shortcuts/signed/trove-url-bookmark.shortcut) | Save a URL from Safari |
| [Trove Location Check-in](https://github.com/joshmcarthur/trove/blob/main/examples/ios-shortcuts/signed/trove-location-checkin.shortcut) | Log current location with optional label |

> If a link 404s, signed files have not been committed yet. Build from
> [`unsigned/`](https://github.com/joshmcarthur/trove/tree/main/examples/ios-shortcuts/unsigned)
> or follow the manual recipes below.

**To import:**

1. Open a `.shortcut` file on your iPhone (AirDrop, Files, or Safari).
2. Tap **Add Shortcut**.
3. Enter your **Trove ingest URL** when prompted, e.g.
   `https://trove.example.com/ingest/shortcuts` (full path, HTTPS on cellular).
4. Run the Shortcut once to verify, then enable Share Sheet or other triggers.

## Endpoint contract

```
POST https://<host>/ingest/shortcuts
Content-Type: application/json

{ ... }
```

- The `:source` path segment (`shortcuts`) becomes the event `source` field.
- Body must be valid JSON (object, array, or primitive).
- Optional top-level fields peeled into event metadata: `type`, `time` (RFC3339),
  `blob_ref`. Everything else stays in `payload`.
- Default event `type` if omitted: `http.ingest.received`.
- Success: `204 No Content`.

## Network and auth

By default HTTP ingest has **no authentication**. When `[http.auth].validator` is
set (e.g. `module.http-gateway.bearer`), add an `Authorization` header with
`Bearer <token>` to ingest and blob requests. See
[network auth planning](../planning/auth.md). For a home server, common setups are:

- **Local network** — `http://192.168.x.x:8080/ingest/shortcuts` (Wi‑Fi only;
  Shortcuts may block plain HTTP on cellular).
- **Tailscale** — HTTPS via your tailnet hostname (recommended for phone capture).
- **Public HTTPS** — reverse proxy with TLS in front of `:8080`.

See [network auth planning](../planning/auth.md) for auth options once exposing
beyond a trusted tailnet.

## Photo attachments

For images and other binary content from the share sheet:

1. `PUT https://<host>/blobs` with raw image bytes → receive `{ "blob_ref": "sha256-..." }`
2. `POST https://<host>/ingest/shortcuts` with JSON including `blob_ref` and metadata
   (`type`, `title`, `content_type`, etc.)

Binary content must not be inlined in the ingest JSON body.

### Share Sheet with photo

1. **Trigger:** Share Sheet (enable URLs, text, images).
2. **If** Shortcut Input is an image:
   - **Get Contents of URL** — Method PUT, URL `https://YOUR_HOST/blobs`, Request Body: Shortcut Input.
   - **Get Dictionary from Input** (from response JSON) → `blob_ref`.
3. **Dictionary** — `type`: `shortcuts.share.saved`, plus `blob_ref` (if image), `text`, `url`, `title`, `content_type` as available.
4. **Get Contents of URL** — Method POST, URL `https://YOUR_HOST/ingest/shortcuts`,
   Headers `Content-Type: application/json`, Request Body: Dictionary.

Text-only and URL-only share captures skip step 2 and POST JSON directly (see below).

## Event type conventions

Use explicit `type` values so MCP search can find captures later:

| Shortcut use | Suggested `type` | Payload fields |
|--------------|------------------|----------------|
| Share sheet capture | `shortcuts.share.saved` | `title`, `url`, `text`, `content_type` |
| Quick note / dictation | `shortcuts.note.created` | `text`, optional `tags[]` |
| URL bookmark | `shortcuts.url.saved` | `url`, `title` |
| Location check-in | `shortcuts.location.checked` | `latitude`, `longitude`, `label` |
| Clipboard save | `shortcuts.clipboard.saved` | `text` |
| Quick capture (classify later) | *(via capture-classifier)* | POST `https://<host>/capture/shortcuts` with arbitrary JSON body |

Example payloads: [`examples/ios-shortcuts/payloads/`](../examples/ios-shortcuts/payloads/).

### Quick capture (classify later)

Use the [capture-classifier](../planning/deferred-capture.md) module when you want
to store data before choosing a semantic type:

1. **Trigger:** App icon or Siri.
2. **Ask for Input** — multiline text.
3. **Dictionary** — `text`: Provided Input.
4. **Get Contents of URL** — Method POST, URL `https://YOUR_HOST/capture/shortcuts`,
   Headers `Content-Type: application/json`, Request Body: Dictionary.

Classify later via MCP `classify_event` or `POST /classify`.

## Manual recipes

Build your own Shortcut if you prefer full control.

### Share Sheet → Trove (text or URL)

1. **Trigger:** Share Sheet (enable URLs, text, images).
2. **Dictionary** — `type`: `shortcuts.share.saved`, `text`: Shortcut Input.
3. **Get Contents of URL** — Method POST, URL `https://YOUR_HOST/ingest/shortcuts`,
   Headers `Content-Type: application/json`, Request Body: Dictionary.

For images, use the [photo flow](#share-sheet-with-photo) above.

### Quick Note

1. **Trigger:** App icon or Siri.
2. **Ask for Input** — multiline text.
3. **Dictionary** — `type`: `shortcuts.note.created`, `text`: Provided Input.
4. **Get Contents of URL** — POST JSON body (Dictionary).

### URL Bookmark

1. **Trigger:** Share Sheet (Safari).
2. **Dictionary** — `type`: `shortcuts.url.saved`, `url`: Shortcut Input.
3. **Get Contents of URL** — POST JSON body.

### Location Check-in

1. **Trigger:** App icon.
2. **Get Current Location**.
3. **Ask for Input** — optional label.
4. **Dictionary** — `type`: `shortcuts.location.checked`, location fields.
5. **Get Contents of URL** — POST JSON body.

## Limitations

- **No authentication by default** — enable `[http.auth].validator` or use localhost /
  a trusted tailnet. See [network auth](../planning/auth.md).
- **10 MiB** request body limit (`max_body_bytes` in HTTP ingest manifest).
- **Photos in Share Sheet** — use the [photo flow](#share-sheet-with-photo); the
  importable Shortcut captures text/URL metadata by default.

## Verify capture

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://localhost:8080/ingest/shortcuts" \
  -H "Content-Type: application/json" \
  -d '{"type":"shortcuts.note.created","text":"hello from curl"}'
# expect 204
```

Once MCP is connected, use `search_events` to find captured notes. The MCP query
server is **Supported** — see [MCP query planning](../planning/mcp-query.md).

## Next steps

- [Quick Start](./quick-start.md) — run `trove` locally
- [HTTP ingest planning](../planning/http-ingest.md) — endpoint details
- [examples/ios-shortcuts/](../examples/ios-shortcuts/) — payloads and maintainer docs
