"""The only write path. Work inside record() is one activity, or nothing."""

from __future__ import annotations

import contextlib
from dataclasses import dataclass, field

from django.db import transaction
from django.utils import timezone

from trove.models import Activity, Blob, Claim, Item, Link
from trove.runtime.core import get_core
from trove.services.blobs import write_blob
from trove.tasks.enqueue import enqueue_for_items

DATA = "data"


@dataclass
class Recorder:
    kind: str
    agent: str
    subject: Item | None = None
    params: dict = field(default_factory=dict)
    cost: float | None = None
    _items: list = field(default_factory=list)
    _claims: list = field(default_factory=list)
    _links: list = field(default_factory=list)
    _blobs: dict = field(default_factory=dict)

    def item(self, type_name: str) -> Item:
        item = Item(type=type_name)
        self._items.append(item)
        return item

    def claim(self, item: Item, path: str, value, rank=Claim.Rank.NORMAL):
        self._claims.append(Claim(item=item, path=path, value=value, rank=rank))

    def link(self, source: Item, target: Item, rel: str) -> Link:
        link = Link(source=source, target=target, rel=rel)
        self._links.append(link)
        return link

    def retract(self, link: Link):
        self._links.append(
            Link(source=link.source, target=link.target, rel=link.rel, retracts=link)
        )

    def blob(self, data: bytes, content_type: str) -> str:
        sha = write_blob(data)
        self._blobs[sha] = (len(data), content_type)
        return sha

    def _flush(self, started):
        core = get_core()
        for item in self._items:
            core.validate(
                item.type, {c.path: c.value for c in self._claims if c.item is item}
            )
        activity = Activity.objects.create(
            kind=self.kind,
            agent=self.agent,
            subject=self.subject,
            params=self.params,
            cost=self.cost,
            outcome=Activity.Outcome.OK,
            started=started,
            ended=timezone.now(),
        )
        for sha, (size, ctype) in self._blobs.items():
            if not Blob.objects.filter(sha256=sha).exists():
                Blob.objects.create(sha256=sha, size=size, content_type=ctype)
        for obj in [*self._items, *self._claims, *self._links]:
            obj.activity = activity
            obj.save()
        return activity


@contextlib.contextmanager
def record(
    kind: str, agent: str, subject: Item | None = None, params: dict | None = None
):
    rec, started = Recorder(kind, agent, subject, params or {}), timezone.now()
    try:
        yield rec
    except Exception as exc:
        record_failure(kind, agent, subject, exc, started, rec.params, rec.cost)
        raise
    with transaction.atomic(using=DATA):
        rec.activity = rec._flush(started)
        enqueue_for_items([i.id for i in rec._items])


def record_failure(kind, agent, subject, exc, started, params=None, cost=None):
    Activity.objects.create(
        kind=kind,
        agent=agent,
        subject=subject,
        params=params or {},
        cost=cost,
        outcome=Activity.Outcome.FAILED,
        error=repr(exc)[:4000],
        started=started,
        ended=timezone.now(),
    )
