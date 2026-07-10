---
title: Contributing
nav_order: 10
---

# Contributing

## Before you start

1. Read the [roadmap](./roadmap.md) and pick a **Planned** feature whose
   dependencies are met.
2. Open the matching [planning](./planning/index.md) page — it is the
   implementation brief.
3. AI agents: see [AGENTS.md](https://github.com/joshmcarthur/trove/blob/main/AGENTS.md).

## Prerequisites

- Go 1.26+ ([mise](https://mise.jdx.dev/) + `.mise.toml` recommended)
- `golangci-lint` 2.12+ for `make lint`
- Deno 2.9+ (only under `./docs` for the documentation site)
- `golangci-lint` for `make lint`
- Deno (only for docs under `./docs`)

## Commands

| Command | Purpose |
|---------|---------|
| `make fmt` | `go fmt` + goimports |
| `make lint` | golangci-lint |
| `make test` | `go test -race -cover ./...` |
| `make build` | `bin/trove` |
| `make check` | fmt + lint + test |
| `make docs` | build Lume site |
| `make docs-serve` | serve docs locally |

## Workflow

1. Implement in the Go package listed on the planning page.
2. Check off acceptance criteria on that planning page.
3. Update status in [roadmap.md](./roadmap.md).
4. Run `make check` before opening a PR.

## Docs

The docs site is the living plan. When you land a feature, update roadmap status
and planning acceptance criteria in the same PR — do not leave docs stale.

## License

Contributions are under GPLv3, same as the project.
