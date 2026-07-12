---
title: Two-week live test
parent: Getting started
nav_order: 8
---

# Two-week live test

Operational runbook for milestone 4 (spec §11.4): run Trove as your daily
journal for two weeks and validate capture plus conversational retrieval.

## Goals

1. **Capture broadly** — HTTP ingest, iOS Shortcuts, Telegram, MQTT as available.
2. **Retrieve conversationally** — use MCP (`search_events`, `summarize_range`) from
   Cursor or another client at least daily.
3. **Note gaps** — record what you asked for that Trove could not answer.

## Day 0 — setup checklist

Complete [Try in a day](./try-in-a-day.md) first if you have not already — it
covers install, config, first captures, and MCP query in one afternoon. Then use
this checklist before starting the two-week run:

- [ ] Build host and modules: `make build`
- [ ] Create `trove.toml` with `[journal]`, `[blobs]`, `[http]`, `[modules].paths`
- [ ] Point `[modules].paths` at the repo `modules/` directory (or install path)
- [ ] Start `trove -config /path/to/trove.toml` and confirm it stays running
- [ ] Verify HTTP ingest: `curl -X POST http://127.0.0.1:8080/ingest/test -d '{}'`
  expects `204`
- [ ] Connect MCP client to `http://127.0.0.1:8080/mcp` — see
  [MCP client setup](./mcp-client.md)
- [ ] Import at least one iOS Shortcut — see [iOS Shortcuts](./ios-shortcuts.md)
- [ ] (Optional) Configure [Telegram](./telegram.md) or MQTT source

### Security note

By default Trove has **no authentication** on ingest, blobs, or MCP. For the
live test, bind to **localhost** or a **trusted tailnet** (Tailscale). Optional
gateway auth is **Supported** — enable when exposing beyond a trusted network:

```toml
[http.auth]
validator = "module.http-gateway.bearer"

[modules.settings.http-gateway]
token_env = "TROVE_HTTP_TOKEN"
```

See [network auth](../planning/auth.md). Do not expose `:8080` to the public
internet without auth or a reverse proxy with TLS.

### Example config

```toml
[journal]
path = "./trove.db"
# retention_days = 90  # optional; prunes events older than N days

[blobs]
path = "./blobs"

[modules]
paths = ["./modules"]

[http]
listen = "127.0.0.1:8080"
```

Binding to `127.0.0.1` limits exposure to the local machine during the live test.

## Daily routine (days 1–14)

### Capture (at least once per day)

Pick one or more:

| Source | Action |
|--------|--------|
| iOS Shortcut | Run Quick Note or Share Sheet capture |
| HTTP | `POST /ingest/:source` or `POST /capture/:source` for deferred classify |
| Telegram | Send a message or photo to your bot |
| MQTT | Confirm sensor/event traffic appears (if configured) |

### Retrieve (at least once per day)

In your MCP client, try:

1. `summarize_range` for today
2. `search_events` with a keyword from something you captured
3. `get_event` on a specific ULID from search results

If using deferred capture, try `list_unclassified_captures` and `classify_event`.

### Log friction

Keep a simple notes file (outside Trove) with:

- Questions you asked that returned nothing useful
- Capture flows that failed or were too slow
- Features you wished existed

## Validation checklist (end of week 2)

- [ ] At least **50 events** captured across **2+ sources**
- [ ] MCP search found events you remember capturing
- [ ] `summarize_range` reflects recent activity accurately
- [ ] Blob + photo flow tested at least once (`PUT /blobs` → ingest with `blob_ref`)
- [ ] Deferred capture tested at least once (`POST /capture/...` → classify)
- [ ] Trove survived at least one restart without data loss
- [ ] Documented top 3 retrieval failures and top 3 missing features

## Known limitations during live test

| Limitation | Workaround |
|------------|------------|
| No auth by default | Localhost or tailnet; or enable `[http.auth].validator` — see [auth](../planning/auth.md) |
| MQTT reconnect | Restart `trove` if broker was down at startup |
| Retention not enabled by default | Set `[journal].retention_days` to prune old events — see [journal planning](../planning/journal.md) |
| `get_event` does not inline blob bytes | Note `blob_ref`; fetch blob separately if needed |

## After the live test

1. Review friction notes against [roadmap](../roadmap.md) **Later** items.
2. Decide whether embeddings, HA tap, remote modules, or sinks are worth building.
3. File issues or planning updates for anything that blocked daily use.

## See also

- [Quick Start](./quick-start.md)
- [Roadmap](../roadmap.md) — milestone 4 status
- [iOS Shortcuts](./ios-shortcuts.md)
- [Telegram](./telegram.md)
