# Trove — Agent Guide

Trove is a self-hosted collector (Django 5.2 LTS). Capture files and links,
store typed and linked items, view them on a wall.

## Before you write code

1. Read [docs/spec.md](docs/spec.md). Version 1 is the weekend build only.
2. Open the GitHub issue you are implementing. Issues on milestone `v1` are
   the work queue.
3. Do not implement Roadmap items (MCP, embeddings, proposals, telegram,
   purge, native iOS app, data-defined types).
4. Follow django-for-joshmcarthur file layout (small files, one Django app)
   **except** where the spec overrides it.

## Spec overrides of django-for-joshmcarthur

| Concern | Skill default | Trove v1 |
|---------|---------------|----------|
| Framework | Django 6 | Django 5.2 LTS |
| Jobs | Django Tasks + django-ox | In-process thread pool, state in `activities` |
| UI | Turbo + Stimulus | Wall is Preact + htm, vendored, no build step |
| Config | settings.py | Core settings in `.storage/` JSON files |

## Architecture

| Topic | Location |
|-------|----------|
| Full specification | [docs/spec.md](docs/spec.md) |
| Spike of storage and jobs | [docs/planning/spike/](docs/planning/spike/) |
| Go tree (reference only) | `legacy-go` branch |

```
config/                 settings, urls, asgi
trove/
  models/               item, blob, claim, link, activity (one file each)
  queries.py            current values (rank, then newest), live links
  activity.py           the only write path
  registry.py           code types, describe()
  jobs.py               thread pool, reconcile, retries, cycle check
  storage.py            .storage/ JSON
  auth.py               setup code, owner passkey, OAuth scopes
  plugins.py            loader: manifest.json, setup(), enable/disable
  backup.py             manage.py backup
  processes.py          renders supervisord config
  admin.py
plugins/
  ingest/               core
  wall/ palette/ agent/ built-in, version 1
.storage/
```

## Work queue

1. The **kernel** issue must merge first. It ports the spike so the 13 core
   tests pass.
2. Other `v1` issues stay blocked until the kernel is on `main`.
3. One issue per branch. Conventional Commits for PR titles (`feat:`, `fix:`,
   `docs:`).
4. Plugins read through `queries.py` and write through the activity API only.

## Kernel vs spec

The spike is the source for recorder, rank, plugin loading, and jobs. The spec
wins on config:

- One SQLite database in WAL mode, not the spike's two-database router.
- Plugin enablement and core settings live in `.storage/` JSON files, not a
  `ConfigEntry` model.
- `django-oauth-toolkit` keeps OAuth clients and tokens in its own tables.

## Development

Python via `uv`. Django 5.2 LTS. Pin everything.

```bash
uv venv && uv pip install -r requirements.txt
python manage.py migrate
python manage.py test
python manage.py trove   # once the kernel lands
```

## Pull requests

- Conventional Commits for PR titles.
- Never use free-form titles. CI will reject them once commitlint returns.
- Breaking changes: `feat!:` or `fix!:`.

## License

GPLv3.
