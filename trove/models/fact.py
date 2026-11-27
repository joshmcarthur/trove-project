from django.db import models

from .append_only_error import AppendOnlyError
from .append_only_query_set import AppendOnlyQuerySet


class Fact(models.Model):
    trove_database = "data"
    objects = AppendOnlyQuerySet.as_manager()

    class Meta:
        abstract = True

    def save(self, *args, **kwargs):
        if not self._state.adding:
            raise AppendOnlyError("fact tables are append-only")
        super().save(*args, **kwargs)

    def delete(self, *args, **kwargs):
        raise AppendOnlyError("fact tables are append-only")
