# Roadmap

Version 1 is a weekend build that can pass a two-week live test. Everything
else waits until the test says it is worth building.

## Version 1

- Five fact tables with SQLite triggers, the activity API, the query layer,
  a code-type registry, and plugin loading.
- Owner passkey, OAuth for the wall, long-lived tokens, and ingest with file
  metadata. The iOS Shortcut posts a file and gets 201.
- Derive jobs with reconcile and retries, palette extraction, and an agent
  that fills a handful of hand-written types.
- The wall SPA, Django admin, and a one-command backup.

Kill criterion for the live test: after two weeks, would you miss it if it
vanished?

Auth is the largest piece of the weekend. If it runs long, the wall can start
behind Tailscale with the owner session alone and gain OAuth later.

## Later, only when needed

1. MCP add-on, with OAuth for MCP clients.
2. Regions and crops, with overlap-based identity.
3. Embeddings and learning from corrections.
4. Two-stage extraction, data types, proposals, and whole-collection backfill.
5. Per-lane budgets, purge, and upcasting.
6. Telegram add-on.
7. A native iOS app: sign in, register the device like Home Assistant's
   `mobile_app`, and post files from a share extension.
8. A plain-file mirror; SQLite's own `vec1` when it reaches 1.0.

Do not implement roadmap items on a version 1 branch. The spec has the full
write-up; this page is only the version 1 vs later cut.
