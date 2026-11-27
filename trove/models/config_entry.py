from django.db import models


class ConfigEntry(models.Model):
    """Plugin enablement and options. Lives in the config (default) database."""

    domain = models.CharField(primary_key=True, max_length=64)
    enabled = models.BooleanField(default=True)
    data = models.JSONField(default=dict)
    options = models.JSONField(default=dict)
    version = models.PositiveSmallIntegerField(default=1)

    class Meta:
        verbose_name = "plugin"
        verbose_name_plural = "plugins"
