from django.core.management.base import BaseCommand

from trove.models import Activity
from trove.runtime.core import get_core


class Command(BaseCommand):
    help = "Show plugin load status and recent failed activities."

    def handle(self, *args, **options):
        self.stdout.write("plugins")
        for domain, plugin in get_core().plugins.items():
            line = f"  {domain}  {plugin.status}"
            if plugin.error:
                line += f"  {plugin.error}"
            self.stdout.write(line)

        self.stdout.write("failures")
        failures = Activity.objects.filter(outcome=Activity.Outcome.FAILED).order_by(
            "-ended"
        )[:20]
        if not failures:
            self.stdout.write("  none")
            return
        for activity in failures:
            error = activity.error.replace("\n", " ").strip()
            self.stdout.write(
                f"  {activity.ended.isoformat()}  {activity.agent}  "
                f"{activity.kind}  {error}"
            )
