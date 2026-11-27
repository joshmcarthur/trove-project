from django.db.models import Q

from trove.models import Item, Link


def wall_items() -> dict:
    payload = []
    items = (
        Item.objects.exclude(type="colour")
        .select_related("activity")
        .prefetch_related("claims__activity", "subject_activities")
        .order_by("created")
    )
    for item in items:
        colours = []
        derived = (
            Link.objects.live()
            .filter(target=item, rel="derived_from")
            .select_related("source")
        )
        for link in derived:
            if link.source.type != "colour":
                continue
            hexv = link.source.current_values().get("hex")
            if hexv:
                colours.append(hexv)
        payload.append(
            {
                "id": item.id,
                "type": item.type,
                "values": item.current_values(),
                "colours": colours,
                "provenance": provenance_for(item),
            }
        )
    return {"items": payload}


def provenance_for(item: Item) -> list[dict]:
    seen: dict[str, dict] = {}

    def add(activity):
        if activity is None or activity.id in seen:
            return
        seen[activity.id] = {
            "kind": activity.kind,
            "agent": activity.agent,
            "when": activity.ended.isoformat(),
        }

    add(item.activity)
    for claim in item.claims.all():
        add(claim.activity)
    for link in Link.objects.filter(Q(source=item) | Q(target=item)).select_related(
        "activity"
    ):
        add(link.activity)
    for activity in item.subject_activities.all():
        add(activity)
    return sorted(
        seen.values(), key=lambda row: (row["when"], row["kind"], row["agent"])
    )
