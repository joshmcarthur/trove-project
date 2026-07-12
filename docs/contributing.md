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
5. Open the PR with a [Conventional Commits](https://www.conventionalcommits.org/) title (enforced by CI).

## Commit messages

Trove squash-merges PRs. **The PR title becomes the commit on `main`**, which
release-please uses to decide version bumps and changelog entries.

PR titles must follow Conventional Commits. CI validates every PR title.

### Prefixes

| Prefix | Release impact | Example |
|--------|----------------|---------|
| `feat:` | Minor version bump | `feat: add mqtt reconnect backoff` |
| `fix:` | Patch version bump | `fix: handle SQLITE_BUSY in module router` |
| `feat!:` / `fix!:` | Major version bump | `feat!: rename journal.path config key` |
| `docs:` | No release | `docs: update installation guide` |
| `chore:`, `ci:`, `test:`, `refactor:` | No release | `chore: bump golangci-lint` |

Optional scope: `fix(ingest): correct Content-Type handling`.

### Good vs bad titles

Good:

- `feat: add checksums to release artifacts`
- `fix: journal migration for schema_ref column`
- `docs: document brew install path`

Bad (CI will fail):

- `Add mqtt support`
- `fix stuff`
- `WIP`
- `Feature: foo`

### Repository settings

Maintainers: enable **Allow squash merging** and **Default to pull request title
for squash merge commits** under Settings → General → Pull Requests.

### Release automation

Stable releases use [release-please](https://github.com/googleapis/release-please)
and [GoReleaser](https://goreleaser.com/). Merge the release-please PR when ready
to ship; CI tags the release and builds binaries, checksums, `.deb`/`.rpm`, and
Docker images.

**First-time setup on an existing repo:** `release-please-config.json` includes
`bootstrap-sha` so release-please ignores pre-conventional commit history. Remove
`bootstrap-sha` after the first release PR merges.

**Repository settings required:**

1. Settings → General → Pull Requests: squash merge + default to PR title
2. Settings → Actions → General → Workflow permissions: enable **Allow GitHub
   Actions to create and approve pull requests** (release-please opens Release PRs
   with `GITHUB_TOKEN`; without this setting the workflow fails even when commit
   history is valid)

Non-conventional commits in old history are skipped (logged as warnings). They do
not block release-please once `bootstrap-sha` is set.

Repository secrets for full package-manager support:

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Provided by Actions — releases and ghcr.io |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Push Formula to `joshmcarthur/homebrew-trove` |

Create the `homebrew-trove` GitHub repo with a `Formula/` directory before the
first stable release. Without the tap token, GoReleaser still publishes GitHub
Release assets; Homebrew formula push is skipped.

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
