from .append_only_query_set import AppendOnlyQuerySet


class ClaimQuerySet(AppendOnlyQuerySet):
    def current(self) -> dict:
        """Highest rank wins, then newest. A deprecated claim hides that value."""
        from .claim import Claim

        winners, hidden = {}, set()
        for c in self.order_by("-created", "-id"):
            if c.rank == Claim.Rank.DEPRECATED:
                hidden.add((c.path, repr(c.value)))
                continue
            if (c.path, repr(c.value)) in hidden:
                continue
            best = winners.get(c.path)
            if best is None or c.rank > best.rank:
                winners[c.path] = c
        return {path: c.value for path, c in winners.items()}
