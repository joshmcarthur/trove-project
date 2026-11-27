import json

from django.core.management.base import BaseCommand, CommandError

from trove.models import Claim, Item
from trove.services.owner import OWNER_USERNAME
from trove.services.recorder import record


class Command(BaseCommand):
    help = "Append a preferred claim to an item path."

    def add_arguments(self, parser):
        parser.add_argument("item_id", help="ID of the item to update.")
        parser.add_argument("path", help="Claim path, for example title or base.")
        parser.add_argument(
            "value",
            help="Claim value. JSON if valid, otherwise a string.",
        )

    def handle(self, *args, **options):
        path = options["path"]
        if not path:
            raise CommandError("path is required")
        try:
            item = Item.objects.get(id=options["item_id"])
        except Item.DoesNotExist as exc:
            raise CommandError(f"Item {options['item_id']} does not exist") from exc
        value = _parse_value(options["value"])
        with record("edit", OWNER_USERNAME, subject=item) as rec:
            rec.claim(item, path, value, rank=Claim.Rank.PREFERRED)
        self.stdout.write(f"{item.id} {path}")


def _parse_value(raw: str):
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return raw
