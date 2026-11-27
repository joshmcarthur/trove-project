from django.db import router
from django_ox.actions import expire_lease
from django_ox.models import OxTask
from django_ox.worker import Worker


def reap_abandoned() -> int:
    """Put RUNNING tasks back on the queue after their worker has died.

    A killed worker leaves a lease that has not expired yet. The reaper
    only takes expired leases, so expire those rows first, then reap.
    """
    alias = router.db_for_write(OxTask)
    running = list(
        OxTask.objects.using(alias)
        .filter(status=OxTask.Status.RUNNING)
        .values_list("pk", flat=True)
    )
    for pk in running:
        expire_lease(pk)
    return Worker(db_alias=alias, lock_timeout=0).reap()
