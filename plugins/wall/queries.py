from trove.models import Item, Link


def wall_items() -> dict:
    payload = []
    for item in Item.objects.exclude(type="colour").order_by("created"):
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
            }
        )
    return {"items": payload}
