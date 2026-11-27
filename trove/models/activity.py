from django.db import models

from .fact import Fact
from .ids import new_id


class Activity(Fact):
    class Outcome(models.TextChoices):
        OK = "ok"
        FAILED = "failed"

    id = models.CharField(
        primary_key=True, max_length=32, default=new_id, editable=False
    )
    kind = models.CharField(max_length=32)
    agent = models.CharField(max_length=255)
    subject = models.ForeignKey(
        "trove.Item",
        on_delete=models.PROTECT,
        related_name="subject_activities",
        null=True,
        blank=True,
    )
    params = models.JSONField(default=dict)
    cost = models.FloatField(null=True, blank=True)
    outcome = models.CharField(max_length=16, choices=Outcome.choices)
    error = models.TextField(blank=True, default="")
    started = models.DateTimeField()
    ended = models.DateTimeField()
