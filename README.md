# Trove

Trove is a self-hosted, single-user collector. Send it a file or a link and it
catalogues it into typed, linked, searchable items. Originals are never
modified.

This is a Django 5.2 rewrite. The previous Go implementation lives on the
[`legacy-go`](https://github.com/joshmcarthur/trove-project/tree/legacy-go)
branch.

- **Specification:** [docs/spec.md](docs/spec.md)
- **Work queue:** GitHub issues on the `v1` milestone

Version 1 is a weekend build: ingest, a wall, palette extraction, and an agent
that fills a handful of code-declared types. Roadmap items in the spec are not
in scope.

## Development

Python 3.13, [uv](https://docs.astral.sh/uv/), Django 5.2 LTS. Pin everything.
Ruff, pytest, pytest-django, and factory-boy live in the `dev` dependency group.

```bash
uv sync --group dev
uv run ruff format --check .
uv run ruff check .
uv run pytest
```

Do not use `manage.py test`. Put Django model graphs in `tests/factories.py`.

CI on pull requests to `main` runs `uv sync --frozen --group dev`, then ruff
format, ruff check, and pytest.

## Contributing

1. Read [docs/spec.md](docs/spec.md). Version 1 is the weekend build only.
2. Open the GitHub issue you are implementing. Issues on milestone `v1` are
   the work queue.
3. Do not implement roadmap items (MCP, embeddings, proposals, Telegram,
   purge, a native iOS app, data-defined types).
4. Follow django-for-joshmcarthur file layout (small files, one Django app)
   except where this project overrides it.

| Concern | Skill default | Trove v1 |
|---------|---------------|----------|
| Framework | Django 6 | Django 5.2 LTS |
| Jobs | Django Tasks + django-ox | In-process thread pool, state in `activities` |
| UI | Turbo + Stimulus | Wall is Preact + htm, vendored, no build step |
| Config | settings.py | `ConfigEntry` in the `default` database; facts in `data` |

The kernel issue must merge first: it ports the spike so the 13 core tests
pass, and other `v1` issues stay blocked until that lands. One issue per
branch.

The spike is the source for the recorder, rank rules, plugin loading, and
jobs. Storage matches the spike: two SQLite databases and a `ConfigEntry`
model, not `.storage/` JSON files.

PR titles are Conventional Commits (`feat:`, `fix:`, `docs:`). Plugins read
through the query layer and write through the recorder only.

## License

GNU GPLv3 — see [LICENSE](LICENSE).
