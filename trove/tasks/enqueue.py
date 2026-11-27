from datetime import timedelta

from django.tasks import task

from trove.models import Activity, Item
from trove.registry.processor import Processor
from trove.runtime.core import get_core
from trove.tasks.transient_error import TransientError

LIVE = 10
BACKFILL = 0
MAX_ATTEMPTS = 3


def already_done(p: Processor, item: Item) -> bool:
    return Activity.objects.filter(
        kind="derive",
        agent=p.agent,
        subject=item,
        outcome=Activity.Outcome.OK,
    ).exists()


def derive_backoff(exception, task_result):
    if isinstance(exception, TransientError):
        if exception.retry_after is not None:
            return timedelta(seconds=exception.retry_after)
        return min(5 * 2 ** (task_result.attempts - 1), 300)
    return None


@task(max_attempts=MAX_ATTEMPTS, backoff=derive_backoff, takes_context=True)
def derive_item(context, processor_name: str, item_id: str):
    # execute -> record -> enqueue; a module-level import would loop.
    from trove.tasks.execute import execute_derive

    p = get_core().processors[processor_name]
    execute_derive(p, item_id, attempt=context.attempt, max_attempts=MAX_ATTEMPTS)


def enqueue_derive(processor_name: str, item_id: str, priority=LIVE) -> bool:
    p = get_core().processors[processor_name]
    item = Item.objects.get(id=item_id)
    if already_done(p, item):
        return False
    derive_item.using(priority=priority).enqueue(processor_name, item_id)
    return True


def enqueue_for_items(item_ids, priority=LIVE):
    if not item_ids:
        return
    core = get_core()
    for item in Item.objects.filter(id__in=item_ids):
        for p in core.processors_for(item.type):
            enqueue_derive(p.name, item.id, priority=priority)
