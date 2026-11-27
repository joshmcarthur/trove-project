from django.db import models


class SetupCode(models.Model):
    """One-time owner claim code. Lives in the config (default) database."""

    code = models.CharField(max_length=32, unique=True)
    used_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
