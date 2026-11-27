# Trove core spike (Django 5.2 LTS)

This directory is the storage and jobs spike that version 1 ports. It is not
a runnable project. The zip only contained these files. Reconstruct the rest
from `test_core.py` and [docs/spec.md](../../spec.md).

`ingest.py` was `__init__.py` in the zip. It is the ingest plugin.

## What the spike proved

- Append-only facts with SQLite triggers
- Rank then recency for claims
- Link assert and retract
- Plugin load with rollback on failure
- Processor cycle check
- In-process jobs with reconcile and retries
- Palette and a stub agent, keyed emits keeping identity across version bumps

## What to change when porting

- One SQLite database, not the two-database router in `routers.py`
- `.storage/` JSON for plugin enablement, not a `ConfigEntry` model
- Keep `record()`, `ClaimQuerySet.current()`, `Core.load()`, `Runtime.reconcile()`

## Commands the full spike used

```
uv venv && uv pip install "django>=5.2,<5.3" pydantic pillow uvicorn httpx
python manage.py migrate
python manage.py test tests            # 13 tests, now in test_core.py
python manage.py trove                 # one process: plugins, jobs, web on :8123
```

Not in the spike: django-oauth-toolkit / allauth passkeys (a dev token stands
in), the wall SPA, `manage.py backup`, supervisord rendering, a real model
call in the agent plugin.
