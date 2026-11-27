from __future__ import annotations

import fnmatch

from trove.models import Activity, Item
from trove.runtime.core import get_core
from trove.tasks.derive import BACKFILL, enqueue_derive


def reconcile() -> int:
    """Enqueue a derive task for each item that still needs one at this version."""
    core = get_core()
    count = 0
    types = list(Item.objects.values_list("type", flat=True).distinct())
    for p in core.processors.values():
        matching = [t for t in types if fnmatch.fnmatch(t, p.consumes)]
        done = Activity.objects.filter(
            kind="derive",
            agent=p.agent,
            outcome=Activity.Outcome.OK,
        ).values("subject_id")
        for item_id in (
            Item.objects.filter(type__in=matching)
            .exclude(id__in=done)
            .values_list("id", flat=True)
        ):
            enqueue_derive(p.name, item_id, priority=BACKFILL)
            count += 1
    return count
