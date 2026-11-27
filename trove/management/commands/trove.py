import threading

from django.conf import settings
from django.core.management import call_command
from django.core.management.base import BaseCommand

from trove.runtime.core import get_core
from trove.services.reconcile import reconcile


class Command(BaseCommand):
    help = "Start the web server and one ox worker."

    def add_arguments(self, parser):
        parser.add_argument(
            "--drain",
            action="store_true",
            help="Reconcile, run queued work, then exit.",
        )

    def handle(self, *args, **options):
        get_core()
        n = reconcile()
        self.stdout.write(f"reconcile submitted {n} jobs")
        if options["drain"]:
            call_command(
                "ox_worker",
                processes=1,
                concurrency=settings.TROVE["WORKER_CONCURRENCY"],
                database="data",
                batch=True,
            )
            return

        worker = threading.Thread(target=self._run_worker, daemon=True)
        worker.start()
        import uvicorn

        uvicorn.run("config.asgi:application", host="0.0.0.0", port=8123)

    def _run_worker(self):
        call_command(
            "ox_worker",
            processes=1,
            concurrency=settings.TROVE["WORKER_CONCURRENCY"],
            database="data",
        )
