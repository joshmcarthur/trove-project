from django.db import models

from .base import AppendOnlyQuerySet, Fact, new_id


class ClaimQuerySet(AppendOnlyQuerySet):
    def current(self) -> dict:
        """Resolve to {path: value}: highest rank wins, then newest.

        A deprecated claim hides earlier claims of the same value at that path.
        """
        winners, hidden = {}, set()
        for c in self.order_by("-created", "-id"):          # newest first
            if c.rank == Claim.Rank.DEPRECATED:
                hidden.add((c.path, repr(c.value)))
                continue
            if (c.path, repr(c.value)) in hidden:
                continue
            best = winners.get(c.path)
            if best is None or c.rank > best.rank:
                winners[c.path] = c
        return {path: c.value for path, c in winners.items()}


class Claim(Fact):
    """One value at one path on one item, from a person or a processor."""

    class Rank(models.IntegerChoices):
        DEPRECATED = 0
        NORMAL = 1
        PREFERRED = 2

    id = models.CharField(primary_key=True, max_length=32, default=new_id, editable=False)
    item = models.ForeignKey("trove.Item", on_delete=models.PROTECT, related_name="claims")
    path = models.CharField(max_length=255)
    value = models.JSONField()
    rank = models.SmallIntegerField(choices=Rank.choices, default=Rank.NORMAL)
    activity = models.ForeignKey("trove.Activity", on_delete=models.PROTECT, related_name="claims")
    created = models.DateTimeField(auto_now_add=True)

    objects = ClaimQuerySet.as_manager()

    class Meta:
        indexes = [models.Index(fields=["item", "path"])]
