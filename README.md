# Trove

Trove is a personal event journal: a single Go binary that captures typed
events from the sources in your life into one durable, queryable store, with a
conversational MCP interface as the primary way to get information back out.

> Capture broadly, store simply, converse to retrieve.

- **Documentation:** [trove docs site](https://joshmcarthur.github.io/trove/) (or `make docs-serve` locally)
- **Specification:** [docs/spec.md](docs/spec.md)
- **Agent guide:** [AGENTS.md](AGENTS.md)
- **Roadmap:** [docs/roadmap.md](docs/roadmap.md)

## Development

Requires Go 1.26+ (recommended: [mise](https://mise.jdx.dev/) with the
project `.mise.toml`).

```bash
make check        # fmt, lint, test
make build        # bin/trove
make docs-serve   # local docs site (requires Deno in ./docs)
```

Recommended VS Code extension: [Go](https://marketplace.visualstudio.com/items?itemName=golang.go).

## License

GNU GPLv3 — see [LICENSE](LICENSE).
