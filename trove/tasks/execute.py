from __future__ import annotations

import threading
import weakref

from django.utils import timezone

from trove.models import Activity, Claim, ConfigEntry, Item, Link
from trove.registry.processor import Processor
from trove.services.recorder import record, record_failure
from trove.tasks.context import Context
from trove.tasks.emit import Emit
from trove.tasks.result import Result
from trove.tasks.transient_error import TransientError


class _KeyedLock:
    __slots__ = ("lock", "__weakref__")

    def __init__(self):
        self.lock = threading.Lock()


_lock_registry = threading.Lock()
_derive_locks: weakref.WeakValueDictionary[tuple[str, str], _KeyedLock] = (
    weakref.WeakValueDictionary()
)


def _lock_for(processor_name: str, item_id: str) -> _KeyedLock:
    key = (processor_name, item_id)
    with _lock_registry:
        keyed = _derive_locks.get(key)
        if keyed is None:
            keyed = _KeyedLock()
            _derive_locks[key] = keyed
        return keyed


def execute_derive(p: Processor, item_id: str, *, attempt=1, max_attempts=1):
    """One attempt. django-ox retries TransientError."""
    keyed = _lock_for(p.name, item_id)
    with keyed.lock:
        item = Item.objects.get(id=item_id)
        if Activity.successful_derives(p.agent, subject_id=item_id).exists():
            return
        ctx = Context(_options(p))
        started = timezone.now()
        try:
            result = p.run(item, ctx) or Result()
        except TransientError as exc:
            if attempt >= max_attempts:
                record_failure(
                    "derive", p.agent, item, exc, started, {"attempts": max_attempts}
                )
            raise
        except Exception as exc:  # noqa: BLE001
            record_failure("derive", p.agent, item, exc, started, {"attempt": 1})
            return
        try:
            with record("derive", p.agent, subject=item, params=result.params) as rec:
                rec.cost = result.cost
                _apply(rec, item, result, p.name)
        except Exception as exc:  # noqa: BLE001
            record_failure(
                "derive", p.agent, item, exc, started, result.params, result.cost
            )


def _apply(rec, item: Item, result: Result, processor_name: str):
    emitted: dict[tuple[str, str], Item] = {}
    for path, value in result.claims.items():
        rec.claim(item, path, value)
    for emit in result.emits:
        target = _existing_emit(item, emit) or rec.item(emit.type)
        if target._state.adding:
            rec.link(target, item, "derived_from")
            rec.claim(target, "$key", emit.key)
        for path, value in emit.claims.items():
            rec.claim(target, path, value)
        if emit.blob is not None:
            rec.claim(target, "$blob", rec.blob(emit.blob, emit.content_type))
        emitted[(emit.type, emit.key)] = target
    _sync_result_links(rec, item, result, emitted, processor_name)


def _sync_result_links(
    rec,
    item: Item,
    result: Result,
    emitted: dict[tuple[str, str], Item],
    processor_name: str,
):
    wanted: set[tuple[str, str, str]] = set()
    for spec in result.links:
        source = _link_end(spec.source, emitted)
        target = _link_end(spec.target, emitted)
        key = (source.pk, target.pk, spec.rel)
        if key in wanted:
            continue
        wanted.add(key)
        if not (
            Link.objects.live()
            .filter(source_id=source.pk, target_id=target.pk, rel=spec.rel)
            .exists()
        ):
            rec.link(source, target, spec.rel)
    previous = (
        Link.objects.live()
        .filter(
            activity__kind="derive",
            activity__agent__startswith=f"{processor_name}@",
            activity__subject=item,
        )
        .exclude(rel="derived_from")
    )
    for link in previous:
        if (link.source_id, link.target_id, link.rel) not in wanted:
            rec.retract(link)


def _link_end(end: Item | Emit, emitted: dict[tuple[str, str], Item]) -> Item:
    if isinstance(end, Emit):
        try:
            return emitted[(end.type, end.key)]
        except KeyError as exc:
            raise ValueError(
                "Result.links refers to emit "
                f"{end.type!r}/{end.key!r} that was not emitted"
            ) from exc
    if isinstance(end, Item):
        return end
    raise TypeError(f"Result.links needs Item or Emit, not {type(end)!r}")


def _existing_emit(source: Item, emit: Emit) -> Item | None:
    derived = (
        Link.objects.live()
        .filter(target=source, rel="derived_from")
        .values("source_id")
    )
    claim = (
        Claim.objects.filter(
            path="$key",
            value=emit.key,
            item__type=emit.type,
            item_id__in=derived,
        )
        .select_related("item")
        .first()
    )
    return claim.item if claim else None


def _options(p: Processor) -> dict:
    entry = ConfigEntry.objects.filter(domain=p.name).first()
    return entry.options if entry else {}
