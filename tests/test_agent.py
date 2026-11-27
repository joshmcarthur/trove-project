import json
from urllib.request import Request

import pytest

from plugins.agent.run import DEFAULT_MODEL, fillable_types
from tests.conftest import FakeResponse, completion_body, http_error
from tests.factories import ConfigEntryFactory, ItemFactory
from trove.core import REGISTRY, get_core, reset_core
from trove.models import Activity, Item
from trove.tasks import TransientError
from trove.tasks.execute import execute_derive

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


def test_agent_provides_registry(core):
    processor = core.processors["agent"]
    assert processor.provides == REGISTRY
    assert processor.consumes == "capture.*"


def test_agent_plus_registry_still_loads(core):
    processor = core.processors["agent"]
    provided = core.provided(processor)
    assert processor.provides == REGISTRY
    assert "capture.image" not in provided
    assert "colour" in provided


def test_describe_still_includes_colour_and_capture(core):
    names = {row["name"] for row in core.describe()}
    assert "colour" in names
    assert "capture.image" in names
    assert "capture.file" in names
    assert "capture.link" in names


def test_prompt_types_skip_colour_and_capture(core):
    names = {row["name"] for row in fillable_types(core)}
    assert "colour" not in names
    assert not any(name.startswith("capture.") for name in names)
    assert {"bulb", "reference_shot", "link"} <= names
    blob = json.dumps(fillable_types(core))
    assert "hex" not in blob.lower()


def test_prompt_sent_to_model_omits_colour_schema(core, monkeypatch):
    seen = {}

    def fake_urlopen(request: Request, timeout=None):
        seen["body"] = json.loads(request.data.decode())
        return FakeResponse()

    monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)
    execute_derive(core.processors["agent"], ItemFactory().id)
    user = seen["body"]["messages"][1]["content"]
    catalog = json.loads(user.split("Types:\n", 1)[1].split("\n\nJSON shape", 1)[0])
    names = {row["name"] for row in catalog}
    assert "colour" not in names
    assert not any(name.startswith("capture.") for name in names)
    assert "hex" not in json.dumps(catalog).lower()


def test_model_fills_are_recorded_and_colour_is_dropped(core, monkeypatch):
    payload = {
        "fills": [
            {
                "type": "bulb",
                "key": "e27",
                "claims": {"base": "E27", "shape": "A60"},
            },
            {
                "type": "colour",
                "key": "#336699",
                "claims": {"hex": "#336699"},
            },
            {
                "type": "capture.image",
                "key": "shot",
                "claims": {"title": "nope"},
            },
        ]
    }

    def fake_urlopen(request, timeout=None):
        return FakeResponse(
            completion_body(json.dumps(payload), cost=0.012, model=DEFAULT_MODEL)
        )

    monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)
    item = ItemFactory()
    execute_derive(core.processors["agent"], item.id)
    bulb = Item.objects.get(type="bulb")
    assert bulb.current_values()["base"] == "E27"
    assert Item.objects.filter(type="colour").count() == 0
    assert Item.objects.filter(type="capture.image").count() == 1
    act = Activity.objects.get(agent="agent@1", subject=item, outcome="ok")
    assert act.cost == 0.012
    assert act.params["model"] == DEFAULT_MODEL
    assert len(act.params["prompt_hash"]) == 64


def test_endpoint_and_model_come_from_config(core, monkeypatch):
    seen = {}

    def fake_urlopen(request: Request, timeout=None):
        seen["url"] = request.full_url
        seen["body"] = json.loads(request.data.decode())
        return FakeResponse(completion_body("{}", model="local-small"))

    monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)
    ConfigEntryFactory(
        domain="agent",
        options={
            "base_url": "https://example.test/v1",
            "model": "local-small",
        },
    )
    item = ItemFactory()
    execute_derive(core.processors["agent"], item.id)
    assert seen["url"] == "https://example.test/v1/chat/completions"
    assert seen["body"]["model"] == "local-small"
    act = Activity.objects.get(agent="agent@1", subject=item, outcome="ok")
    assert act.params["model"] == "local-small"


def test_missing_key_fails_the_derive(core, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("OPENROUTER_API_KEY", raising=False)
    item = ItemFactory()
    execute_derive(core.processors["agent"], item.id)
    act = Activity.objects.get(agent="agent@1", subject=item)
    assert act.outcome == "failed"
    assert "API key" in act.error


def test_rate_limit_is_transient(core, monkeypatch):
    def fake_urlopen(request, timeout=None):
        raise http_error(429, {"Retry-After": "1.5"})

    monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)
    with pytest.raises(TransientError) as caught:
        execute_derive(core.processors["agent"], ItemFactory().id)
    assert caught.value.retry_after == 1.5
    assert not Activity.objects.filter(agent="agent@1", outcome="ok").exists()
