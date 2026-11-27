# Trove

Trove is a self-hosted, single-user collector. Send it a file or a link and it
catalogues it into typed, linked, searchable items. Originals are never
modified.

This is a Django 6.1 rewrite. The previous Go implementation lives on the
[`legacy-go`](https://github.com/joshmcarthur/trove-project/tree/legacy-go)
branch.

- **Docs:** https://joshmcarthur.github.io/trove-project/
- **Specification:** [docs/spec.md](docs/spec.md)
- **Work queue:** GitHub issues on the `v1` milestone

Version 1 is a weekend build: ingest, a wall, palette extraction, and an agent
that fills a handful of code-declared types. Roadmap items in the spec are not
in scope.

## Development

Python 3.13, [uv](https://docs.astral.sh/uv/), Django 6.1, Django Tasks, and
django-ox. Pin everything. Ruff, pytest, pytest-django, factory-boy, and
Zensical live in the `dev` dependency group.

```bash
uv sync --group dev
uv run playwright install chromium-headless-shell
uv run ruff format --check .
uv run ruff check .
uv run pytest
uv run zensical serve
```

`playwright install chromium-headless-shell` is Playwright's pinned browser, not
the Chrome you log into. pytest launches it for one local JS-rendered fixture.
CI installs the same browser with `--with-deps` before pytest. Without that
binary, the JS fixture test is skipped and the rest of the suite still uses
fakes.

Do not use `manage.py test`. Put Django model graphs in `tests/factories.py`.
`manage.py backup` writes a tarball; restore is unpack plus migrate. The steps
are in [docs/getting-started.md](docs/getting-started.md).

CI on pull requests to `main` runs `uv sync --frozen --group dev`, installs
`chromium-headless-shell`, then ruff format, ruff check, and pytest.

## Contributing

1. Read [docs/spec.md](docs/spec.md). Version 1 is the weekend build only.
2. Open the GitHub issue you are implementing. Issues on milestone `v1` are
   the work queue.
3. Do not implement roadmap items (MCP, embeddings, proposals, Telegram,
   purge, a native iOS app, data-defined types).
4. Follow django-for-joshmcarthur file layout (small files, one Django app)
   except where this project overrides it. Kernel packages are `models/`,
   `views/`, `forms/`, `services/`, `tasks/`, `queries/`, and `plugins/`.
   Put one class in each file, and re-export from the package `__init__.py`.

| Concern | Skill default | Trove v1 |
|---------|---------------|----------|
| Framework | Django 6 | Django 6.1 |
| Jobs | Django Tasks + django-ox | Django Tasks + django-ox (`OxTask` on `default`) |
| UI | Turbo + Stimulus | Wall is Preact + htm, vendored, no build step |
| Config | settings.py | `ConfigEntry` in the `default` database; facts in `data` |

The kernel issue must merge first: it ports the spike so the 13 core tests
pass, and other `v1` issues stay blocked until that lands. One issue per
branch.

The spike is the source for the recorder, rank rules, and plugin loading.
Do not port the spike `jobs.py` thread pool; persist derive jobs with Django
Tasks and django-ox. Storage matches the spike: two SQLite databases and a
`ConfigEntry` model, not `.storage/` JSON files.

PR titles are Conventional Commits (`feat:`, `fix:`, `docs:`). Plugins read
through the query layer and write through the recorder only.

## License

GNU GPLv3 — see [LICENSE](LICENSE).
