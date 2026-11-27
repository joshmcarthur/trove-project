from django.core.management.base import BaseCommand, CommandError

from trove.services.tokens import mint_ingest_token


class Command(BaseCommand):
    help = "Create a long-lived ingest token with write:capture scope."

    def add_arguments(self, parser):
        parser.add_argument("verb", choices=["create"])
        parser.add_argument("name", help="Client name recorded as the activity agent.")

    def handle(self, *args, **options):
        verb = options["verb"]
        if verb != "create":
            raise CommandError(f"Unknown action: {verb}")
        try:
            raw = mint_ingest_token(options["name"])
        except ValueError as exc:
            raise CommandError(str(exc)) from exc
        self.stdout.write(raw)
