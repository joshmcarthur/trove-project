from django.contrib import admin

from trove.models import Activity


@admin.register(Activity)
class ActivityAdmin(admin.ModelAdmin):
    list_display = (
        "id",
        "kind",
        "agent",
        "outcome",
        "subject",
        "error",
        "started",
        "ended",
    )
    list_filter = ("outcome", "kind")
    search_fields = ("id", "agent", "error")
    readonly_fields = (
        "id",
        "kind",
        "agent",
        "subject",
        "params",
        "cost",
        "outcome",
        "error",
        "started",
        "ended",
    )
    ordering = ("-ended",)

    def has_add_permission(self, request):
        return False

    def has_delete_permission(self, request, obj=None):
        return False

    def save_model(self, request, obj, form, change):
        return
