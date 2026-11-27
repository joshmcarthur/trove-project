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

The iOS capture path in version 1 is a share-sheet Shortcut: it posts the
shared file to `/ingest/` with the long-lived token and expects 201. A native
share extension is roadmap.

There is no importable `.shortcut` in this tree yet. Build one that POSTs the
file and the token. The Go `/records` JSON path is gone.

## Two-week live test

Version 1 is meant to survive two weeks of real use. Kill criterion: after two
weeks, would you miss it if it vanished? If not, stop and fall back to a
Google Photos album.

During the test:

- Capture at least once a day (Shortcut or `curl` to `/ingest/`).
- Browse the wall. MCP is roadmap, so it is not part of the test.
- Note gaps. Leave roadmap items (MCP, embeddings, Telegram, purge) until
  after the test.

Auth is the large piece. If it runs long, the wall can sit behind Tailscale
with the owner session alone and gain OAuth later.
