# Getting started

Trove is a Django collector. You send it files; it stores typed, linked items
and shows them on a wall.

## Install

You need Python 3.13 and [uv](https://docs.astral.sh/uv/). Clone the repo,
then install the project and the `dev` group (ruff, pytest, Zensical):

```bash
uv sync --group dev
```

Pin versions, and run Zensical with `uv run zensical` rather than a global
install. Deno and Lume stay gone.

## First run

Toolchain checks:

```bash
uv run ruff format --check .
uv run ruff check .
uv run pytest
```

Do not use `manage.py test`. Preview this site with `uv run zensical serve`.

## Ingest

An authenticated client posts a file plus optional metadata to `/ingest/`.
That creates a blob, an item, and claims inside an `ingest` activity.
Metadata you supply (title, notes) is a preferred claim. A successful
capture returns 201.

Mint a long-lived token with scope `write:capture` (`manage.py token create`
or the admin). Shortcuts cannot run a browser login, so they use that token
the way Home Assistant uses long-lived tokens. Bind to localhost or a
Tailscale network until auth is in place.

## iOS Shortcut

Share a photo or file into Capture. The Shortcut posts it to `/ingest/` with
a `write:capture` token and shows the 201 JSON body, which has `id` and
`type`. This replaces the Go `/records` JSON path.

The workflow is `ios/Capture.plist`. iOS 15 and later will only import a
signed `.shortcut`. On a Mac signed into iCloud:

```bash
plutil -convert binary1 -o /tmp/Capture.shortcut ios/Capture.plist
shortcuts sign --mode anyone --input /tmp/Capture.shortcut --output ~/Desktop/Capture.shortcut
```

AirDrop `~/Desktop/Capture.shortcut` to the iPhone and open it. Import asks
for the Trove base URL with no trailing slash (`https://trove.example.ts.net`)
and the token printed by `uv run python manage.py token create shortcut:iphone`.
Register the owner passkey at `/setup` before you mint that token.

Title, notes, and a page URL are blank Text actions in the shortcut. Leave
them, edit them, or replace them with Ask for Input. Fields you send become
preferred claims. Camera-roll HEIC is a normal file upload, and a native
share extension is on the roadmap.

## Two-week live test

Version 1 is meant to survive two weeks of real use. Kill criterion: after two
weeks, would you miss it if it vanished? If not, stop and fall back to a
Google Photos album.

During the test:

- Capture at least once a day (Shortcut or `curl` to `/ingest/`).
- Browse the wall. MCP is on the roadmap, so it is not part of the test.
- Note gaps. Leave roadmap items (MCP, embeddings, Telegram, purge) until
  after the test.

Auth is the large piece. If it runs long, the wall can sit behind Tailscale
with the owner session alone and gain OAuth later.
