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
| License | GPLv3 |

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
