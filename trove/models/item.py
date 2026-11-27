from django.db import models

from .fact import Fact
from .ids import new_id


class Item(Fact):
    id = models.CharField(
        primary_key=True, max_length=32, default=new_id, editable=False
    )
    type = models.CharField(max_length=255)
    activity = models.ForeignKey(
        "trove.Activity",
        on_delete=models.PROTECT,
        related_name="items",
    )
    created = models.DateTimeField(auto_now_add=True)

    def current_values(self) -> dict:
        return self.claims.current()
