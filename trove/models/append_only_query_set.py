from django.db import models

from .append_only_error import AppendOnlyError


class AppendOnlyQuerySet(models.QuerySet):
    def update(self, **kwargs):
        raise AppendOnlyError("fact tables are append-only")

    def delete(self):
        raise AppendOnlyError("fact tables are append-only")
