from django.db import models

from .fact import Fact


class Blob(Fact):
    sha256 = models.CharField(primary_key=True, max_length=64)
    size = models.PositiveBigIntegerField()
    content_type = models.CharField(max_length=255)
    created = models.DateTimeField(auto_now_add=True)
