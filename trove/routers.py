"""Facts and OxTask live on data. Config, auth, and the rest live on default."""

from django.apps import apps


def _db_for(model):
    if model._meta.app_label == "django_ox":
        return "data"
    return getattr(model, "trove_database", "default")


class TroveRouter:
    def db_for_read(self, model, **hints):
        return _db_for(model)

    db_for_write = db_for_read

    def allow_relation(self, a, b, **hints):
        return _db_for(a._meta.model) == _db_for(b._meta.model)

    def allow_migrate(self, db, app_label, model_name=None, **hints):
        if "target_db" in hints:
            return db == hints["target_db"]
        if app_label == "django_ox":
            return db == "data"
        if model_name is None:
            return db == "default"
        try:
            model = apps.get_model(app_label, model_name)
        except LookupError:
            return db == "default"
        return db == _db_for(model)
