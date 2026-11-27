import pytest
from pydantic import ValidationError

from tests.factories import ItemFactory
from trove.core import Processor, get_core, reset_core
from trove.models import Item
from trove.tasks import Emit, Result
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])

AGENT_TYPES = ("bulb", "reference_shot", "link", "screenshot")
COLOUR_KEYS = {"hex", "colour", "color", "rgb"}


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


def test_hand_written_types_are_registered(core):
    for name in AGENT_TYPES:
        assert name in core.types


def test_describe_yields_json_schema_without_colour(core):
    descriptions = {row["name"]: row for row in core.describe()}
    for name in AGENT_TYPES:
        row = descriptions[name]
        assert row["description"]
        schema = row["schema"]
        assert schema.get("type") == "object"
        properties = schema.get("properties") or {}
        assert properties
        lowered = {key.lower() for key in properties}
        assert COLOUR_KEYS.isdisjoint(properties)
        assert COLOUR_KEYS.isdisjoint(lowered)


def test_derive_can_fill_types_without_colour(core):
    source = ItemFactory()

    def fill(item, ctx):
        return Result(
            emits=[
                Emit(
                    type="bulb",
                    key="e27",
                    claims={"base": "E27", "shape": "A60"},
                ),
                Emit(
                    type="reference_shot",
                    key="lamp",
                    claims={"subject": "hallway lamp"},
                ),
                Emit(
                    type="link",
                    key="spec",
                    claims={
                        "url": "https://example.com/lamp",
                        "title": "lamp spec",
                    },
                ),
                Emit(
                    type="screenshot",
                    key="settings",
                    claims={"surface": "Settings"},
                ),
            ]
        )

    execute_derive(
        Processor("fill_types", 1, "capture.*", list(AGENT_TYPES), fill),
        source.id,
    )
    bulb = Item.objects.get(type="bulb")
    shot = Item.objects.get(type="reference_shot")
    link = Item.objects.get(type="link")
    shot_ui = Item.objects.get(type="screenshot")
    assert bulb.current_values()["base"] == "E27"
    assert shot.current_values()["subject"] == "hallway lamp"
    assert link.current_values()["url"] == "https://example.com/lamp"
    assert shot_ui.current_values()["surface"] == "Settings"
    assert "hex" not in bulb.current_values()
    assert "hex" not in shot_ui.current_values()


def test_colour_claim_is_rejected_on_agent_types(core):
    for name in AGENT_TYPES:
        with pytest.raises(ValidationError):
            core.validate(name, {"hex": "#336699"})
