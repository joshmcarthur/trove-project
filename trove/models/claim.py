from django.db import models

from .claim_query_set import ClaimQuerySet
from .fact import Fact
from .ids import new_id


class Claim(Fact):
    """One value at one path on one item, from a person or a processor."""

    class Rank(models.IntegerChoices):
        DEPRECATED = 0
        NORMAL = 1
        PREFERRED = 2

    id = models.CharField(
        primary_key=True, max_length=32, default=new_id, editable=False
    )
    item = models.ForeignKey(
        "trove.Item", on_delete=models.PROTECT, related_name="claims"
    )
    path = models.CharField(max_length=255)
    value = models.JSONField()
    rank = models.SmallIntegerField(choices=Rank.choices, default=Rank.NORMAL)
    activity = models.ForeignKey(
        "trove.Activity", on_delete=models.PROTECT, related_name="claims"
    )
    created = models.DateTimeField(auto_now_add=True)

    objects = ClaimQuerySet.as_manager()

    class Meta:
        indexes = [models.Index(fields=["item", "path"])]
