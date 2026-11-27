"""In-process background work, the Home Assistant way: lanes of worker threads inside
the one core process. No queue table. Durability comes from reconcile, which finds
items with no successful derive activity at a processor's current version."""
from __future__ import annotations

import fnmatch
import itertools
import logging
import queue
import threading
import time
from dataclasses import dataclass, field

from django.conf import settings
from django.db import close_old_connections, connections
from django.utils import timezone

from .core import Processor, get_core
from .models import Activity, Claim, Item, Link

log = logging.getLogger("trove.jobs")
LIVE, BACKFILL = 0, 10


class TransientError(Exception):
    """Raise from a processor for errors worth retrying (429, 5xx, timeouts)."""
    def __init__(self, msg="", retry_after: float | None = None):
        super().__init__(msg)
        self.retry_after = retry_after


@dataclass
class Emit:
    type: str
    key: str                                   # identity within (source item, type)
    claims: dict = field(default_factory=dict)
    blob: bytes | None = None
    content_type: str = "application/octet-stream"


@dataclass
class Result:
    claims: dict = field(default_factory=dict)  # values on the item itself
    emits: list[Emit] = field(default_factory=list)
    cost: float | None = None
    params: dict = field(default_factory=dict)  # model, prompt hash, ...


class Context:
    """The narrow surface a processor sees."""
    def __init__(self, options: dict):
        self.options = options

    def values(self, item: Item) -> dict:
        return item.current_values()

    def blob_path(self, sha: str):
        from .services.blobs import blob_path
        return blob_path(sha)


# --- running one job -----------------------------------------------------------------

def already_done(p: Processor, item: Item) -> bool:
    return Activity.objects.filter(kind="derive", agent=p.agent, subject=item,
                                   outcome=Activity.Outcome.OK).exists()


def run_job(p: Processor, item_id: str, sleep=time.sleep):
    from .services.recorder import record, record_failure
    item = Item.objects.get(id=item_id)
    if already_done(p, item):
        return
    ctx = Context(_options(p))
    last_exc = None
    for attempt in range(1, p.attempts + 1):
        started = timezone.now()
        try:
            result = p.run(item, ctx) or Result()
        except TransientError as exc:
            last_exc = exc
            if attempt < p.attempts:
                sleep(exc.retry_after or min(2 ** attempt, 60))
            continue
        except Exception as exc:  # noqa: BLE001  permanent
            record_failure("derive", p.agent, item, exc, started, {"attempt": attempt})
            return
        try:
            with record("derive", p.agent, subject=item, params=result.params) as rec:
                rec.cost = result.cost
                _apply(rec, item, result)
        except Exception as exc:  # noqa: BLE001  e.g. output failed type validation
            record_failure("derive", p.agent, item, exc, started, result.params, result.cost)
        return
    record_failure("derive", p.agent, item, last_exc, timezone.now(), {"attempts": p.attempts})


def _apply(rec, item: Item, result: Result):
    for path, value in result.claims.items():
        rec.claim(item, path, value)
    for emit in result.emits:
        target = _existing_emit(item, emit) or rec.item(emit.type)
        if target._state.adding:                       # new: link it and record its key
            rec.link(target, item, "derived_from")
            rec.claim(target, "$key", emit.key)
        for path, value in emit.claims.items():
            rec.claim(target, path, value)
        if emit.blob is not None:
            rec.claim(target, "$blob", rec.blob(emit.blob, emit.content_type))


def _existing_emit(source: Item, emit: Emit) -> Item | None:
    derived = Link.objects.live().filter(target=source, rel="derived_from").values("source_id")
    claim = (Claim.objects.filter(path="$key", value=emit.key, item__type=emit.type,
                                  item_id__in=derived).select_related("item").first())
    return claim.item if claim else None


def _options(p: Processor) -> dict:
    from .models import ConfigEntry
    entry = ConfigEntry.objects.filter(domain=p.name).first()
    return entry.options if entry else {}


# --- lanes ---------------------------------------------------------------------------

class Lane:
    def __init__(self, name: str, size: int, runtime: "Runtime"):
        self.name, self.q, self.runtime = name, queue.PriorityQueue(), runtime
        self.threads = [threading.Thread(target=self._work, name=f"lane-{name}-{i}", daemon=True)
                        for i in range(size)]

    def _work(self):
        while True:
            _, _, job = self.q.get()
            if job is None:
                return
            p, item_id = job
            try:
                close_old_connections()
                run_job(p, item_id)
            except Exception:  # noqa: BLE001
                log.exception("job %s on %s crashed", p.agent, item_id)
            finally:
                connections.close_all()                # this thread's connections only
                self.runtime._done(p, item_id)
                self.q.task_done()


class Runtime:
    def __init__(self):
        self.core = get_core()
        sizes = settings.TROVE["LANES"]
        self.lanes = {n: Lane(n, s, self) for n, s in sizes.items()}
        self._seq = itertools.count()
        self._inflight: set = set()
        self._cv = threading.Condition()

    def start(self):
        for lane in self.lanes.values():
            for t in lane.threads:
                t.start()

    def submit(self, p: Processor, item_id: str, priority=LIVE):
        with self._cv:
            if (p.name, item_id) in self._inflight:
                return
            self._inflight.add((p.name, item_id))
        self.lanes[p.lane].q.put((priority, next(self._seq), (p, item_id)))

    def _done(self, p, item_id):
        with self._cv:
            self._inflight.discard((p.name, item_id))
            self._cv.notify_all()

    def notify(self, item_ids):
        for item in Item.objects.filter(id__in=item_ids):
            for p in self.core.processors_for(item.type):
                self.submit(p, item.id, LIVE)

    def reconcile(self) -> int:
        count = 0
        types = list(Item.objects.values_list("type", flat=True).distinct())
        for p in self.core.processors.values():
            matching = [t for t in types if fnmatch.fnmatch(t, p.consumes)]
            done = Activity.objects.filter(kind="derive", agent=p.agent,
                                           outcome=Activity.Outcome.OK).values("subject_id")
            for item_id in (Item.objects.filter(type__in=matching).exclude(id__in=done)
                            .values_list("id", flat=True)):
                self.submit(p, item_id, BACKFILL)
                count += 1
        return count

    def wait_idle(self, timeout=60) -> bool:
        with self._cv:
            return self._cv.wait_for(lambda: not self._inflight, timeout)

    def stop(self):
        for lane in self.lanes.values():
            for _ in lane.threads:
                lane.q.put((99, next(self._seq), None))
            for t in lane.threads:
                t.join(timeout=10)


_runtime: Runtime | None = None


def start() -> Runtime:
    global _runtime
    _runtime = Runtime()
    _runtime.start()
    return _runtime


def stop():
    global _runtime
    if _runtime:
        _runtime.stop()
    _runtime = None


def notify(item_ids):
    """Called after an activity commits. Outside `manage.py trove`, reconcile catches up."""
    if _runtime is not None:
        _runtime.notify(item_ids)
