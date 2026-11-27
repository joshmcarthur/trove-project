from .append_only_query_set import AppendOnlyQuerySet


class LinkQuerySet(AppendOnlyQuerySet):
    def live(self):
        retracted = self.filter(retracts__isnull=False).values("retracts_id")
        return self.filter(retracts__isnull=True).exclude(id__in=retracted)
