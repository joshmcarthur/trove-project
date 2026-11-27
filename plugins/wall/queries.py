from django.db.models import Exists, OuterRef, Q

from trove.models import Blob, Item, Link


def wall_items() -> dict:
    has_derived_parent = Exists(
        Link.objects.live().filter(source_id=OuterRef("pk"), rel="derived_from")
    )
    items = list(
        Item.objects.exclude(type="colour")
        .annotate(_derived=has_derived_parent)
        .filter(_derived=False)
        .select_related("activity")
        .prefetch_related("claims__activity", "subject_activities")
        .order_by("created")
    )
    item_ids = {item.id for item in items}
    live_links = list(
        Link.objects.live()
        .filter(Q(source_id__in=item_ids) | Q(target_id__in=item_ids))
        .select_related("source", "target")
        .prefetch_related("source__claims", "target__claims")
    )
    content_types = _blob_types(live_links)
    links_by_item: dict[str, list] = {item.id: [] for item in items}
    for link in live_links:
        if link.source_id in links_by_item:
            links_by_item[link.source_id].append(
                _link_row(link, link.source, content_types)
            )
        if link.target_id in links_by_item and link.target_id != link.source_id:
            links_by_item[link.target_id].append(
                _link_row(link, link.target, content_types)
            )
    payload = []
    for item in items:
        rows = links_by_item[item.id]
        colours = []
        for row in rows:
            if row["rel"] != "derived_from" or row["type"] != "colour":
                continue
            hexv = row["values"].get("hex")
            if hexv:
                colours.append(hexv)
        payload.append(
            {
                "id": item.id,
                "type": item.type,
                "values": item.current_values(),
                "colours": colours,
                "links": rows,
                "provenance": provenance_for(item),
            }
        )
    return {"items": payload}


def _blob_types(links: list[Link]) -> dict[str, str]:
    shas = []
    for link in links:
        for end in (link.source, link.target):
            sha = end.current_values().get("$blob")
            if isinstance(sha, str) and sha:
                shas.append(sha)
    if not shas:
        return {}
    return {
        blob.sha256: blob.content_type for blob in Blob.objects.filter(sha256__in=shas)
    }


def _link_row(link: Link, item: Item, content_types: dict[str, str]) -> dict:
    other = link.target if link.source_id == item.id else link.source
    values = other.current_values()
    row = {
        "rel": link.rel,
        "id": other.id,
        "type": other.type,
        "values": values,
    }
    sha = values.get("$blob")
    if isinstance(sha, str):
        ctype = content_types.get(sha)
        if ctype:
            row["content_type"] = ctype
    return row


def provenance_for(item: Item) -> list[dict]:
    seen: dict[str, dict] = {}

    def add(activity):
        if activity is None or activity.id in seen:
            return
        row = {
            "kind": activity.kind,
            "agent": activity.agent,
            "when": activity.ended.isoformat(),
        }
        params = activity.params or {}
        model = params.get("model")
        if model:
            row["model"] = model
        prompt_hash = params.get("prompt_hash")
        if prompt_hash:
            row["prompt_hash"] = prompt_hash
        if activity.cost is not None:
            row["cost"] = activity.cost
        seen[activity.id] = row

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
