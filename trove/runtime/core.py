"""The plugin registry. Plugins are folders with a manifest.json and setup()."""

from __future__ import annotations

import fnmatch
import importlib.util
import json
import logging
import sys
import threading
import types
from pathlib import Path

from django.conf import settings
from django.core.exceptions import ImproperlyConfigured
from django.db.utils import OperationalError, ProgrammingError
from pydantic import BaseModel

from trove.models import ConfigEntry
from trove.registry.processor import REGISTRY, Processor
from trove.runtime.loaded_plugin import LoadedPlugin

log = logging.getLogger("trove")


class Core:
    def __init__(self):
        self.types: dict[str, type[BaseModel]] = {}
        self.processors: dict[str, Processor] = {}
        self.plugins: dict[str, LoadedPlugin] = {}
        self._current: LoadedPlugin | None = None

    def add_type(self, name: str, model: type[BaseModel]):
        self.types[name] = model

    def add_processor(self, processor: Processor):
        self.processors[processor.name] = processor

    def add_urls(self, patterns: list):
        self._current.urls.extend(patterns)

    def processors_for(self, item_type: str) -> list[Processor]:
        return [
            p
            for p in self.processors.values()
            if fnmatch.fnmatch(item_type, p.consumes)
        ]

    def provided(self, p: Processor) -> list[str]:
        return list(self.types) if p.provides == REGISTRY else list(p.provides)

    def validate(self, type_name: str, values: dict):
        model = self.types.get(type_name)
        if model is not None:
            model.model_validate(
                {k: v for k, v in values.items() if not k.startswith("$")}
            )

    def describe(self) -> list[dict]:
        return [
            {
                "name": name,
                "description": (model.__doc__ or "").strip(),
                "schema": model.model_json_schema(),
            }
            for name, model in self.types.items()
        ]

    def load(self):
        manifests = _discover(settings.TROVE["PLUGIN_DIRS"])
        entries = _config_entries()
        for domain in _dependency_order(manifests):
            path, manifest = manifests[domain]
            lp = self.plugins[domain] = LoadedPlugin(domain, manifest)
            entry = entries.get(domain)
            missing = [
                d
                for d in manifest.get("dependencies", [])
                if self.plugins.get(d) is None or self.plugins[d].status != "loaded"
            ]
            if entry is not None and not entry.enabled:
                lp.status = "disabled"
                continue
            if missing:
                lp.status, lp.error = "disabled", f"needs {', '.join(missing)}"
                continue
            snapshot = (dict(self.types), dict(self.processors))
            self._current = lp
            try:
                _import(domain, path).setup(self, entry or ConfigEntry(domain=domain))
            except Exception as exc:  # noqa: BLE001
                self.types, self.processors = snapshot
                if manifest.get("kind") == "core":
                    raise ImproperlyConfigured(
                        f"core plugin {domain!r} failed: {exc}"
                    ) from exc
                lp.status, lp.error, lp.urls = "failed", repr(exc), []
                log.exception("plugin %s failed to load; disabled", domain)
            finally:
                self._current = None
        self._check_cycles()

    def _check_cycles(self):
        procs = list(self.processors.values())
        edges = {
            p.name: [
                q.name
                for q in procs
                if q.name != p.name
                and any(fnmatch.fnmatch(t, q.consumes) for t in self.provided(p))
            ]
            for p in procs
        }
        state = {}

        def visit(n, trail):
            if state.get(n) == "open":
                raise ImproperlyConfigured(
                    "processor loop: " + " -> ".join(trail + [n])
                )
            if state.get(n) == "done":
                return
            state[n] = "open"
            for m in edges[n]:
                visit(m, trail + [n])
            state[n] = "done"

        for p in procs:
            visit(p.name, [])


def _discover(dirs) -> dict[str, tuple[Path, dict]]:
    found = {}
    for d in map(Path, dirs):
        for manifest_path in sorted(d.glob("*/manifest.json")) if d.exists() else []:
            manifest = json.loads(manifest_path.read_text())
            found.setdefault(manifest["domain"], (manifest_path.parent, manifest))
    return found


def _dependency_order(manifests) -> list[str]:
    order, seen = [], set()

    def visit(domain):
        if domain in seen or domain not in manifests:
            return
        seen.add(domain)
        for dep in manifests[domain][1].get("dependencies", []):
            visit(dep)
        order.append(domain)

    for domain in sorted(manifests):
        visit(domain)
    return order


def _import(domain: str, path: Path):
    parent = "trove_plugins"
    if parent not in sys.modules:
        pkg = types.ModuleType(parent)
        pkg.__path__ = []
        sys.modules[parent] = pkg
    name = f"{parent}.{domain}"
    if name in sys.modules:
        return sys.modules[name]
    spec = importlib.util.spec_from_file_location(
        name, path / "__init__.py", submodule_search_locations=[str(path)]
    )
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def _config_entries() -> dict:
    try:
        return {e.domain: e for e in ConfigEntry.objects.all()}
    except (OperationalError, ProgrammingError):
        return {}


_core: Core | None = None
_lock = threading.Lock()


def get_core() -> Core:
    global _core
    with _lock:
        if _core is None:
            core = Core()
            core.load()
            _core = core
        return _core


def reset_core():
    """Tests only."""
    global _core
    _core = None
