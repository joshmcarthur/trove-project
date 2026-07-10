# Trove iOS Shortcuts

Importable Shortcuts and JSON payload templates for capturing events via
[HTTP ingest](../../docs/planning/http-ingest.md).

## Import a Shortcut

Signed `.shortcut` files live in [`signed/`](./signed/). They are committed to
the repo after a maintainer signs them on a Mac (see below).

1. Open a signed file on your iPhone (AirDrop, Files, or tap a GitHub raw link).
2. Tap **Add Shortcut** and answer the import question with your ingest URL, e.g.
   `https://trove.tailnet.ts.net/ingest/shortcuts` (full path, HTTPS on cellular).
3. Run once to verify, then enable Share Sheet or other triggers as needed.

| Shortcut | File | Trigger |
|----------|------|---------|
| Trove Share Sheet | `signed/trove-share-sheet.shortcut` | Share Sheet |
| Trove Quick Note | `signed/trove-quick-note.shortcut` | App / Siri |
| Trove URL Bookmark | `signed/trove-url-bookmark.shortcut` | Safari share |
| Trove Location Check-in | `signed/trove-location-checkin.shortcut` | App icon |

Full guide: [docs/getting-started/ios-shortcuts.md](../../docs/getting-started/ios-shortcuts.md).

## Payload templates

See [`payloads/`](./payloads/) for example JSON bodies. Each Shortcut builds
equivalent JSON and POSTs to `/ingest/shortcuts`.

## Maintainers

**Edit `unsigned/` only** — never hand-edit `signed/`.

### Signing (macOS + iCloud required)

Apple's `shortcuts sign` validates files through iCloud. You need:

1. A Mac with the Shortcuts app
2. **Signed in to iCloud** (System Settings → Apple Account)
3. Outbound network access

```bash
# Regenerate unsigned sources after editing generate_unsigned.py
python3 examples/ios-shortcuts/generate_unsigned.py

# Sign (fails without iCloud)
./examples/ios-shortcuts/sign.sh

# Commit unsigned + signed together
git add examples/ios-shortcuts/unsigned/ examples/ios-shortcuts/signed/
```

Unsigned sources are XML plists from [`generate_unsigned.py`](./generate_unsigned.py).

**GitHub-hosted macOS runners cannot sign** — they are not logged into iCloud.
There is no automated signing in CI today. If you change `unsigned/`, run
`sign.sh` locally and commit `signed/` in the same PR.

### Troubleshooting

- **`shortcuts sign` fails / asks for iCloud** — sign in to iCloud on the Mac and retry.
- **Unsigned file rejected on import** — use a file from `signed/`, not `unsigned/`.
- **Shortcuts fails on cellular** — ingest URL must be HTTPS and reachable.
- **400 from Trove** — body must be valid JSON; check Dictionary → JSON wiring.
- **Host URL** — use the full ingest path ending in `/ingest/shortcuts`.
