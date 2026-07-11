---
title: Telegram
parent: Getting started
nav_order: 5
---

# Telegram capture

Send text and media to a personal Telegram bot; Trove stores a `classify.pending`
event and guides you to classify it in the same chat. See
[planning/telegram-source](../planning/telegram-source.md) for the full design.

## Prerequisites

1. Build Trove and configure `[modules].paths` — see [Quick Start](./quick-start.md).
2. `[blobs]` configured in `trove.toml` (media is stored via `core.Put`).
3. A Telegram bot token from [@BotFather](https://t.me/BotFather).

## 1. Create a bot

1. Open Telegram and message [@BotFather](https://t.me/BotFather).
2. Send `/newbot` and follow the prompts.
3. Copy the HTTP API token.

## 2. Find your chat ID

Message [@userinfobot](https://t.me/userinfobot) or send any message to your bot
and inspect a one-off `getUpdates` response. Your user ID is the `chat.id` for
direct messages.

## 3. Configure the module

Edit `modules/telegram-source/manifest.toml`:

```toml
bot_token_env    = "TELEGRAM_BOT_TOKEN"
allowed_chat_ids = [123456789]   # your Telegram user/chat ID

[[bot.types]]
label       = "Quick note"
target_type = "note.quick"

[[bot.types]]
label       = "Bookmark"
target_type = "note.bookmark"
```

Set the token in your environment (recommended):

```bash
export TELEGRAM_BOT_TOKEN="123456:ABC…"
```

Or set `bot_token` directly in the manifest (do not commit secrets).

## 4. Build and start

```bash
make build
```

Ensure `modules/` is in `[modules].paths` in `trove.toml`, then start Trove:

```bash
./bin/trove
```

## 5. Test the flow

1. Open a DM with your bot.
2. Send a photo or text message.
3. The bot replies with `Captured 01J…` and type buttons.
4. Tap **Quick note** (or another configured type).
5. Answer any field prompts, or `/skip` optional fields.
6. Confirm with `Logged as note.quick (01J…)`.

Query the journal via MCP `search_events` or the capture-classifier
`GET /pending` endpoint.

## Power-user commands

| Command | Use |
|---------|-----|
| `/note hello` | Log a quick note without the picker |
| `/bookmark` | Start a bookmark with field prompts |
| `/classify 01J… note.quick` | Classify a pending event by ID |
| `/cancel` | Abandon the in-chat session (pending event stays in journal) |

## Security

- Only chats in `allowed_chat_ids` are processed; others are ignored silently.
- Keep your bot username private and prefer `bot_token_env` over inline tokens.
- Telegram Bot API has no inbound auth — the allowlist is the security boundary.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Module fails at startup | `TELEGRAM_BOT_TOKEN` set; `allowed_chat_ids` non-empty; at least one `[[bot.types]]` |
| Bot does not respond | Chat ID in allowlist; Trove running; module healthcheck OK |
| "Finish classifying …" | Complete classification or `/cancel` before sending new content |
| Large file rejected | `max_file_bytes` (default 50 MiB) |
