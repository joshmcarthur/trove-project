# Trove — Agent Guide

Trove is a personal event journal (Go). Capture broadly, store simply, converse
to retrieve.

## Before you write code

1. Check [docs/roadmap.md](docs/roadmap.md) for what is Supported vs Planned.
2. Open the relevant [docs/planning/](docs/planning/) page — it is the
   implementation brief.
3. Read affected concept pages and [docs/open-items.md](docs/open-items.md) for
   undecided design choices.
4. Do not implement anything listed in [docs/non-goals.md](docs/non-goals.md).

## Architecture

| Topic | Doc |
|-------|-----|
| Full specification | [docs/spec.md](docs/spec.md) |
| Events & types | [docs/concepts/events.md](docs/concepts/events.md) |
| Journal (SQLite) | [docs/concepts/journal.md](docs/concepts/journal.md) |
| Blob storage | [docs/concepts/blobs.md](docs/concepts/blobs.md) |
| Sources | [docs/concepts/sources.md](docs/concepts/sources.md) |
| Module system | [docs/concepts/modules.md](docs/concepts/modules.md) |
| Query / MCP | [docs/concepts/query.md](docs/concepts/query.md) |

## Development methodology

- **Build order:** follow milestone sequence in [docs/roadmap.md](docs/roadmap.md)
  (spec §11).
- **Per-feature workflow:** planning page → implement in listed `internal/`
  package → update roadmap status and acceptance criteria in the same PR.
- **Scope discipline:** v0 validates capture + conversational retrieval only;
  defer Later items until after the two-week live test.

## Conventions

| Topic | Doc / location |
|-------|----------------|
| Contributing & local commands | [docs/contributing.md](docs/contributing.md) |
| Building external modules | [docs/building-modules.md](docs/building-modules.md) |
| Go formatting & linting | `make fmt`, `make lint` (see Makefile) |
| Tests | `make test` — table-driven, race detector enabled in CI |
| Config format | TOML per [docs/getting-started/configuration.md](docs/getting-started/configuration.md) |
| Commit messages / PR titles | [docs/contributing.md](docs/contributing.md) — Conventional Commits (`feat:`, `fix:`, …) |
| License | GPLv3 |

## Pull requests

- Use **Conventional Commits** for PR titles: `feat: …`, `fix: …`, `docs: …`, etc.
- Never use free-form titles like `Add mqtt support` or `WIP` — CI rejects them.
- Breaking changes: `feat!:` or `fix!:` (e.g. `feat!: rename journal.path config key`).
- The PR title becomes the squash-merge commit on `main`, which drives release-please versioning.
- See [docs/contributing.md](docs/contributing.md) for the full prefix table.

## Go layout

```
cmd/trove/          — main binary
internal/journal/   — SQLite event store
internal/blob/      — content-addressed blob store
internal/modules/   — module discovery + go-plugin runtime
internal/query/     — internal RPC + MCP wrapper
internal/config/    — TOML config loader
```

## After implementing a feature

1. Mark acceptance criteria on the planning page.
2. Update status in [docs/roadmap.md](docs/roadmap.md).
3. Run `make check` before opening a PR.
4. Use a Conventional Commit PR title (`feat:`, `fix:`, `docs:`, etc.).

## Cursor Cloud specific instructions

Toolchain is Go 1.26 + `golangci-lint` 2.12.2 (pinned in `.mise.toml`); both are
preinstalled in the VM image. The base image also ships an older system `go`, so
`go`/`golangci-lint` are symlinked from `/usr/local/bin` to take precedence — do
not remove those symlinks. The startup script runs `go mod download` only.

Standard commands are in [docs/contributing.md](docs/contributing.md) /
`Makefile`: `make build`, `make lint`, `make test`, `make check`.

Non-obvious caveats:

- `make build` produces `bin/trove` plus each first-party module binary at
  `modules/<name>/module` (all gitignored). Built-in modules: `http-ingest`,
  `mcp-query`, and `type-catalog`. The host binary only discovers a
  module when its `module` binary sits next to its `manifest.toml`, so re-run
  `make build` after editing any module before running `trove`.
- `trove` requires `-config <path>` (there is no default); `trove -version` is
  the only subcommand that runs without one. Minimal working config:

  ```toml
  [journal]
  path = "/tmp/trove/trove.db"
  [blobs]
  backend = "filesystem"
  path = "/tmp/trove/blobs"
  [modules]
  paths = ["/workspace/modules"]
  [http]
  listen = ":8080"
  ```

- On startup the `telegram-source` module exits and is restarted on a backoff
  loop (`telegram bot not running`) unless a bot token is configured — this is
  expected noise, not a failure. `mqtt-source` similarly needs a reachable
  broker. The HTTP gateway, ingest, and MCP query paths work without any of that.
- Smoke test the core loop: `POST /ingest/:source` (returns `204`) then query
  `POST /mcp` (Streamable HTTP JSON-RPC) with the `search_events` tool.
- `make test` runs `go test -race ./...`. `TestRouterCatchesUpViaPollAfterPubSubDrop`
  in `internal/modules` can flake with SQLite `SQLITE_BUSY` under full parallel
  race load; it passes reliably when run alone (`go test -race -run <name>
  ./internal/modules/`).
- Deno 2.9 (docs site under `./docs`, `make docs-serve`) is optional and not
  installed by default; it is not needed to build/test/run the app.
