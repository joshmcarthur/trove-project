from django.contrib import admin

from trove.models import ConfigEntry
from trove.runtime.core import get_core


@admin.register(ConfigEntry)
class ConfigEntryAdmin(admin.ModelAdmin):
    list_display = ("domain", "enabled", "version", "load_status")
    change_list_template = "admin/trove/configentry/change_list.html"

    def changelist_view(self, request, extra_context=None):
        extra_context = extra_context or {}
        extra_context["plugin_status"] = list(get_core().plugins.values())
        return super().changelist_view(request, extra_context)

    @admin.display(description="load status")
    def load_status(self, obj):
        plugin = get_core().plugins.get(obj.domain)
        if plugin is None:
            return "—"
        return plugin.status
