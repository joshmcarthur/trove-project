import sys
from pathlib import Path

from django.conf import settings
from django.core.management.base import BaseCommand


class Command(BaseCommand):
    help = "Print supervisord config for manage.py trove."

    def handle(self, *args, **options):
        base = Path(settings.BASE_DIR)
        manage = base / "manage.py"
        self.stdout.write(
            "\n".join(
                [
                    "[program:trove]",
                    f"command={sys.executable} {manage} trove",
                    f"directory={base}",
                    "autostart=true",
                    "autorestart=true",
                    "stopasgroup=true",
                    "killasgroup=true",
                    "",
                ]
            )
        )
