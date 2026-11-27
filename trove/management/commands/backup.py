from pathlib import Path

from django.conf import settings
from django.core.management.base import BaseCommand, CommandError
from django.db import connections

from trove.services.backup import write_tarball


class Command(BaseCommand):
    help = "Write a tarball of both SQLite databases and the blob files."

    def add_arguments(self, parser):
        parser.add_argument("output", type=Path, help="Path to the tarball to write.")

    def handle(self, *args, **options):
        output: Path = options["output"]
        config_path = Path(settings.DATABASES["default"]["NAME"])
        data_path = Path(settings.DATABASES["data"]["NAME"])
        _reject_memory(config_path)
        _reject_memory(data_path)
        connections.close_all()
        write_tarball(
            output,
            config_path=config_path,
            data_path=data_path,
            blob_dir=Path(settings.TROVE["BLOB_DIR"]),
        )
        self.stdout.write(str(output))


def _reject_memory(path: Path):
    text = str(path)
    if text == ":memory:" or "mode=memory" in text:
        raise CommandError("backup needs SQLite files, not an in-memory database")
