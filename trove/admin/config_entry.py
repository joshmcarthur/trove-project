from django.contrib import admin

from trove.models import ConfigEntry


@admin.register(ConfigEntry)
class ConfigEntryAdmin(admin.ModelAdmin):
    list_display = ("domain", "enabled", "version")
