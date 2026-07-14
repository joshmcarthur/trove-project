---
title: Try in a day
parent: Getting started
nav_order: 1
description: A guided afternoon experiment — install Trove, capture typed events, and query them back via MCP.
---

# Try Trove in a day

This is a single afternoon experiment. By the end you will have installed Trove,
captured several typed events using built-in shortcut types, and retrieved them
conversationally through MCP.

> **Goal:** prove the capture → journal → query loop works on your machine before
> committing to a longer run.

```mermaid
flowchart LR
  subgraph morning [Setup]
    install[Install trove]
    config[Write trove.toml]
    run[Start trove]
  end
  subgraph afternoon [Capture]
    curl[POST /records]
    types[5 builtin shortcut types]
  end
  subgraph evening [Query]
    mcp[Connect MCP client]
    search[search_records]
    get[get_record]
  end
  install --> config --> run --> curl --> types --> mcp --> search --> get
```

## What you need

- A laptop running Linux, macOS, or Windows (WSL)
- About two hours (phone optional for the last step)
- [Cursor](https://cursor.com) or another MCP client (optional but recommended)

## Phase 1 — Setup (~30 min)

### Install Trove

Download a release binary for your platform — see
[Installation](./installation.md). Or build from source if you prefer:

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
```

Verify:

```bash
./bin/trove -version
```

### Create a config file

Run `trove init` in your working directory (or `trove init --dir /path/to/data`).
It writes a default `trove.toml` and creates a `blobs/` directory. When a
`modules/` subdirectory is present (for example in a source checkout), `./modules`
is included in `[modules].paths` automatically.

Alternatively, save this as `trove.toml` in your working directory:

```toml
[journal]
path = "./trove.db"

[blobs]
backend = "filesystem"
path = "./blobs"

[modules]
paths = ["./modules"]

[http]
listen = "127.0.0.1:8080"
```

Binding to `127.0.0.1` keeps ingest and MCP on localhost during the experiment.
See [Configuration](./configuration.md) for the full reference.

### Start Trove

From the repo root (after `make build` so module binaries exist):

```bash
./bin/trove -config ./trove.toml
```

Leave this running in a terminal. You should see the HTTP gateway start on port
8080.

### Smoke test

In another terminal:

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"test","text":"hello trove"}'
```

Expected: `201`. This creates a record with the default type
`trove://type/http/ingest/received/1`.

- [ ] Trove starts without errors
- [ ] Smoke test returns `201`

## Phase 2 — Capture menu (~45 min)

Trove ships five **built-in shortcut types** for common capture patterns. Post
one record of each kind to `POST /records`. The `source` field in the JSON body
(`shortcuts`) identifies the capture origin.

Run these from another terminal while `trove` is running:

### Quick note

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"shortcuts","type":"trove://type/shortcuts/note/created/1","text":"my first note"}'
```

Type: `trove://type/shortcuts/note/created/1` — expect `201`.

### Share sheet (URL + text)

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"shortcuts","type":"trove://type/shortcuts/share/saved/1","title":"Example page","url":"https://example.com/article","text":"saved for later","content_type":"url"}'
```

Type: `trove://type/shortcuts/share/saved/1` — expect `201`.

### URL bookmark

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"shortcuts","type":"trove://type/shortcuts/url/saved/1","url":"https://example.com","title":"Example Site"}'
```

Type: `trove://type/shortcuts/url/saved/1` — expect `201`.

### Location check-in

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"shortcuts","type":"trove://type/shortcuts/location/checked/1","latitude":37.7749,"longitude":-122.4194,"label":"Home"}'
```

Type: `trove://type/shortcuts/location/checked/1` — expect `201`.

### Clipboard

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -X POST "http://127.0.0.1:8080/records" \
  -H "Content-Type: application/json" \
  -d '{"source":"shortcuts","type":"trove://type/shortcuts/clipboard/saved/1","text":"copied text from the experiment"}'
```

Type: `trove://type/shortcuts/clipboard/saved/1` — expect `201`.

### Quick verify (optional)

List registered types from the CLI:

```bash
./bin/trove -config ./trove.toml types list
```

Or run the smoke script that posts all six payloads at once:

```bash
./examples/day-one/smoke.sh http://127.0.0.1:8080
```

Example payloads also live in
[`examples/ios-shortcuts/payloads/`](https://github.com/joshmcarthur/trove/tree/main/examples/ios-shortcuts/payloads).

- [ ] Six records captured (five shortcut types + one default ingest)
- [ ] Every `curl` returned `204`

## Phase 3 — Query back (~30 min)

### Connect Cursor

Create or edit `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global):

```json
{
  "mcpServers": {
    "trove": {
      "url": "http://127.0.0.1:8080/mcp"
    }
  }
}
```

Reload Cursor (Settings → MCP). The `trove` server should show as connected with
at least three core tools. Full setup: [MCP client setup](./mcp-client.md).

### Try these queries

Ask your MCP client to call:

**Search by keyword:**

```json
{ "query": "my first note" }
```

Tool: `search_records` — should return the quick note record.

**Get by record_ref:**

Use a `record_ref` from the search results.

Tool: `get_record` — should return the folded record body.

**List incomplete:**

Tool: `list_incomplete_records` — shows records without a resolved type.

**List types** (when `type-catalog` module is loaded):

Tool: `list_types` — enumerates all registered builtin types.

- [ ] MCP client connects to `http://127.0.0.1:8080/mcp`
- [ ] `search_records` finds "my first note"
- [ ] `get_record` returns a captured record
- [ ] `list_incomplete_records` works when applicable

## Phase 4 — Phone optional (~30 min)

The shortcut types above are the same contracts used by importable iOS
Shortcuts. If you have an iPhone:

1. Import [Trove Quick Note](https://github.com/joshmcarthur/trove/blob/main/examples/ios-shortcuts/signed/trove-quick-note.shortcut)
   or another signed Shortcut — see [iOS Shortcuts](./ios-shortcuts.md).
2. Point it at your `POST /records` URL (Tailscale HTTPS recommended for cellular).
3. Run it once and confirm the record appears via `search_records`.

Skip this phase if you do not have a phone handy — the `curl` captures are enough
to validate the loop.

- [ ] (Optional) One Shortcut capture appears in MCP search

## Phase 5 — Wrap-up (~10 min)

### You did it if…

- [ ] Trove runs locally with your config
- [ ] At least six typed records are in the journal
- [ ] MCP `search_records` returns something you captured
- [ ] `get_record` returns a folded record body
- [ ] (Optional) One iOS Shortcut capture worked

### What next?

- **[Two-week live test](./live-test.md)** — use Trove as your daily journal and
  note what retrieval gaps you hit
- **[iOS Shortcuts](./ios-shortcuts.md)** — set up Share Sheet, bookmark, and
  location capture on your phone
- **[Concepts](../concepts.md)** — how revisions, records, the journal, and modules fit together
- **[Roadmap](../roadmap.md)** — what is supported today and what comes later

## Security note

Trove has **no authentication** on ingest or MCP by default. For this experiment,
`127.0.0.1` is the right bind address. Do not expose `:8080` to the public internet
without enabling `[http.auth]` — see [network auth](../planning/auth.md).
