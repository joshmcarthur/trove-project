import subprocess
import sys
from pathlib import Path

from django.conf import settings
from django.core.management import call_command
from django.core.management.base import BaseCommand
from django.db import connections

from trove.runtime.core import get_core
from trove.services.clients import ensure_wall_client
from trove.services.reap import reap_abandoned
from trove.services.reconcile import reconcile
from trove.services.setup_code import announce_setup


class Command(BaseCommand):
    help = "Start the web server and one ox worker."

    def add_arguments(self, parser):
        parser.add_argument(
            "--drain",
            action="store_true",
            help="Reconcile, run queued work, then exit.",
        )

    def handle(self, *args, **options):
        announce_setup(self.stdout)
        ensure_wall_client()
        get_core()
        reap_abandoned()
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

        connections.close_all()
        worker = subprocess.Popen(
            [
                sys.executable,
                str(Path(settings.BASE_DIR) / "manage.py"),
                "ox_worker",
                "--processes",
                "1",
                "--concurrency",
                str(settings.TROVE["WORKER_CONCURRENCY"]),
                "--database",
                "data",
            ]
        )
        try:
            import uvicorn

            uvicorn.run("config.asgi:application", host="0.0.0.0", port=8123)
        finally:
            worker.terminate()
            worker.wait()
