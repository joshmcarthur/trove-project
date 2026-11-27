from django.db import models

from .fact import Fact
from .ids import new_id
from .link_query_set import LinkQuerySet


class Link(Fact):
    id = models.CharField(
        primary_key=True, max_length=32, default=new_id, editable=False
    )
    source = models.ForeignKey(
        "trove.Item",
        on_delete=models.PROTECT,
        related_name="links_out",
    )
    target = models.ForeignKey(
        "trove.Item",
        on_delete=models.PROTECT,
        related_name="links_in",
    )
    rel = models.CharField(max_length=64)
    retracts = models.ForeignKey(
        "self",
        on_delete=models.PROTECT,
        null=True,
        blank=True,
        related_name="retractions",
    )
    activity = models.ForeignKey(
        "trove.Activity", on_delete=models.PROTECT, related_name="links"
    )
    created = models.DateTimeField(auto_now_add=True)

    objects = LinkQuerySet.as_manager()
