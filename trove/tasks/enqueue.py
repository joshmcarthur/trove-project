from django.tasks import task

from trove.models import Item
from trove.runtime.core import get_core
from trove.tasks.transient_error import TransientError

LIVE = 10
BACKFILL = 0


def derive_backoff(exception, task_result):
    if isinstance(exception, TransientError):
        if exception.retry_after is not None:
            return exception.retry_after
        return min(5 * 2 ** (task_result.attempts - 1), 300)
    return None


@task(max_attempts=3, backoff=derive_backoff)
def derive_item(processor_name: str, item_id: str):
    # execute -> record -> enqueue; a module-level import would loop.
    from trove.tasks.execute import execute_derive

    p = get_core().processors[processor_name]
    execute_derive(p, item_id)


def enqueue_derive(processor_name: str, item_id: str, priority=LIVE):
    derive_item.using(priority=priority).enqueue(processor_name, item_id)


def enqueue_for_items(item_ids, priority=LIVE):
    if not item_ids:
        return
    core = get_core()
    for item in Item.objects.filter(id__in=item_ids):
        for p in core.processors_for(item.type):
            enqueue_derive(p.name, item.id, priority=priority)
