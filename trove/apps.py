from django.apps import AppConfig
from pillow_heif import register_heif_opener


class TroveConfig(AppConfig):
    name = "trove"
    default = True
    default_auto_field = "django.db.models.BigAutoField"

    def ready(self):
        register_heif_opener()
