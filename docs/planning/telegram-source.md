---
title: Telegram source
parent: Planning
nav_order: 13
---

# Telegram source

**Status:** Supported\
**Milestone:** 3 — MCP query\
**Spec:** [Sources §6](../spec.md#6-sources), [Deferred capture §3](../spec.md#3-core-concepts)\
**Package:** `modules/telegram-source`, `pkg/classify`

## Goal

Capture text and media from Telegram via a personal bot. Default UX is
**capture-first**: send content, receive a journal event ID, then classify in the
same chat before starting another capture. Power-user slash commands skip the
picker when desired.

## Interfaces

Source module — long-polling Telegram Bot API; uses `trovemodule.Core` for:

- `Put(bytes)` — store media blobs
- `Emit(event)` — fast-path typed events
- `classify.CapturePendingWithResult` — deferred capture (`trove://type/classify/pending/1`)
- `classify.Classify` — classify by `source_event_id`

`provides`: `trove://type/classify/*`, `trove://type/note/*`

## Default flow

1. User sends text, photo, document, or voice to the bot (DM or allowed chat).
2. Module stores media via `core.Put`, emits `trove://type/classify/pending/1` with
   `source = "telegram"`, returns event ID in chat.
3. Inline keyboard (`[[bot.types]]`) offers target types.
4. User picks a type; optional field prompts from matching `[[bot.commands]]`.
5. `classify.Classify` emits typed event + `trove://type/classify/assigned/1`; session cleared.

## One pending per chat

| Situation | Behaviour |
|-----------|-----------|
| Idle + content | Capture → pending + ID → classify flow |
| Active pending + new content | Reject with active ID; user must classify or `/cancel` |
| `/cancel` | Session cleared; pending event remains in journal |
| Classify complete | Session cleared; chat returns to idle |

## Power-user commands

Registered via `[[bot.commands]]` with `fast_path = true`:

| Command | Behaviour |
|---------|-----------|
| `/note` | Emit `trove://type/note/quick/1` directly (optional field prompts) |
| `/bookmark` | Emit `trove://type/note/bookmark/1` directly |
| `/classify <id> <type>` | Classify any pending event by ID |
| `/cancel` | End current session |

## Config (`manifest.toml`)

Module-specific keys (core ignores them). Settings may also be supplied via
`[modules.settings.telegram-source]` or `[modules.config]` in `trove.toml` — see
[Configuration](../getting-started/configuration.md#module-settings-overlays).

```toml
name     = "telegram-source"
version  = "1.0"
kind     = "source"
provides = ["trove://type/classify/*", "trove://type/note/*"]

bot_token_env    = "TELEGRAM_BOT_TOKEN"
allowed_chat_ids = [123456789]
poll_timeout_sec = 30
max_file_bytes   = 52428800
session_ttl_min  = 30

[[bot.types]]
label       = "Quick note"
target_type = "trove://type/note/quick/1"

[[bot.commands]]
command     = "note"
description = "Quick note (skip picker)"
target_type = "trove://type/note/quick/1"
fast_path   = true
```

- `bot_token` or `bot_token_env` — Bot API token (prefer env var)
- `allowed_chat_ids` — required allowlist; other chats silently ignored
- `[[bot.types]]` — inline keyboard after capture
- `[[bot.commands]]` — slash commands; fields apply to matching `target_type`

## Capture payload

`trove://type/classify/pending/1` payload (JSON):

```json
{
  "time": "2026-07-10T10:00:00Z",
  "blob_ref": "sha256:…",
  "text": "optional caption or body",
  "message_id": 123,
  "chat_id": 456,
  "message_kind": "text|photo|document|voice",
  "from": { "id": 789, "username": "you", "first_name": "…" },
  "file": { "file_id": "…", "file_name": "note.pdf", "mime_type": "application/pdf", "size": 1024 }
}
```

Top-level `time` and `blob_ref` are peeled into event metadata by `pkg/classify`.

## Acceptance criteria

- [x] Module starts under go-plugin supervision
- [x] Text message to allowed chat creates one `trove://type/classify/pending/1` event and shows ID
- [x] Photo/document/voice stored via `core.Put` with `blob_ref` on pending event
- [x] Inline type pick → `classify.Classify` → typed event + `trove://type/classify/assigned/1`
- [x] Second capture while one open → blocked with active ID
- [x] `/classify <id> <type>` works on pending event
- [x] `/note` fast path emits typed event without pending
- [x] `/cancel` clears session; pending remains classifiable via MCP/HTTP
- [x] `blob_ref` preserved through classify
- [x] Messages from non-allowed chats are ignored
- [x] Invalid/missing config fails at startup with clear error
- [x] `make check` passes

## Dependencies

- **Blocked by:** module runtime, blob store (`core.Put`), deferred capture (`pkg/classify`)
- **Blocks:** Telegram capture in two-week live test

## Deferred

- Webhook transport (needs public HTTPS / http-gateway)
- Stickers, video messages, locations, polls
- Multiple concurrent pendings per chat
- Auto-classification without user pick

## See also

- [Deferred capture](./deferred-capture.md)
- [Getting started: Telegram](../getting-started/telegram.md)
