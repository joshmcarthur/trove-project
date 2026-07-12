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

## Commands

`make build` compiles `bin/trove` with built-in `http-ingest`, `mcp-query`, and
`type-catalog` modules, plus optional first-party module binaries under
`modules/<name>/module`. Built-in modules need no `[modules].paths` entry. For
MQTT, Telegram, http-gateway, and other external modules, point `[modules].paths`
at the parent `modules/` directory (or an install tree with the same layout).

| Command | Purpose |
|---------|---------|
| `make fmt` | `go fmt` + goimports |
| `make lint` | golangci-lint |
| `make test` | `go test -race -cover ./...` |
| `make build` | `bin/trove` (built-ins) and external module binaries |
| `make check` | fmt + lint + test |
| `make proto` | regenerate `api/proto` → `internal/modules/rpc` |
| `make docs` | build Lume site |
| `make docs-serve` | serve docs locally |

Per-module build targets (also run as part of `make build`):
`build-http-gateway`, `build-http-ingest`, `build-mqtt-source`,
`build-telegram-source`, `build-mcp-query`, `build-capture-classifier`,
`build-type-catalog`.

## Workflow

1. Implement in the Go package listed on the planning page.
2. Check off acceptance criteria on that planning page.
3. Update status in [roadmap.md](./roadmap.md).
4. Run `make check` before opening a PR.

## Docs

The docs site is the living plan. When you land a feature, update roadmap status
and planning acceptance criteria in the same PR — do not leave docs stale.

## iOS Shortcuts

Importable capture Shortcuts live in [`examples/ios-shortcuts/`](../examples/ios-shortcuts/).

- Edit unsigned sources via [`generate_unsigned.py`](../examples/ios-shortcuts/generate_unsigned.py)
  or files in `unsigned/` — never hand-edit `signed/*.shortcut`.
- **Signing requires macOS with iCloud signed in.** Run
  [`sign.sh`](../examples/ios-shortcuts/sign.sh) locally, then commit `signed/`
  in the same PR. GitHub-hosted runners cannot sign.
- After changing unsigned sources: `python3 examples/ios-shortcuts/generate_unsigned.py`,
  `./examples/ios-shortcuts/sign.sh`, commit both `unsigned/` and `signed/`.

## License

Contributions are under GPLv3, same as the project.
