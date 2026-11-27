from django.core.management.base import BaseCommand

from trove.services.setup_code import announce_setup, issue_setup_code


class Command(BaseCommand):
    help = "Print the unused owner setup code, creating one if needed."

    def handle(self, *args, **options):
        code = issue_setup_code()
        if code is None:
            self.stderr.write("The owner passkey is already registered.")
            return
        announce_setup(self.stdout)
