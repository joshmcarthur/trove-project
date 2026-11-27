from __future__ import annotations

from django.utils import timezone

from trove.models import Claim, ConfigEntry, Item, Link
from trove.registry.processor import Processor
from trove.services.recorder import record, record_failure
from trove.tasks.context import Context
from trove.tasks.emit import Emit
from trove.tasks.enqueue import already_done
from trove.tasks.result import Result
from trove.tasks.transient_error import TransientError


def execute_derive(p: Processor, item_id: str, *, attempt=1, max_attempts=1):
    """One attempt. django-ox retries TransientError."""
    item = Item.objects.get(id=item_id)
    if already_done(p, item):
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
            _apply(rec, item, result)
    except Exception as exc:  # noqa: BLE001
        record_failure(
            "derive", p.agent, item, exc, started, result.params, result.cost
        )


def _apply(rec, item: Item, result: Result):
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
