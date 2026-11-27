# Trove

Trove is a self-hosted, single-user collector. Send it a file or a link and it
catalogues it into typed, linked, searchable items. Originals are never
modified.

This is a Django 5.2 rewrite. The previous Go implementation lives on the
[`legacy-go`](https://github.com/joshmcarthur/trove-project/tree/legacy-go)
branch.

- **Specification:** [docs/spec.md](docs/spec.md)
- **Agent guide:** [AGENTS.md](AGENTS.md)
- **Work queue:** GitHub issues on the `v1` milestone

Version 1 is a weekend build: ingest, a wall, palette extraction, and an agent
that fills a handful of code-declared types. Roadmap items in the spec are not
in scope.

## License

GNU GPLv3 — see [LICENSE](LICENSE).
