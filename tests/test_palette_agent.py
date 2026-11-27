import io
import json
from pathlib import Path

import pytest
from django.core.files.uploadedfile import SimpleUploadedFile
from PIL import Image
from pydantic import ValidationError

from tests.conftest import FakeResponse, completion_body
from tests.factories import seed_dev_ingest_token
from trove.core import get_core, reset_core
from trove.models import Claim, Item, Link
from trove.services.recorder import record
from trove.tasks.execute import execute_derive

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}
TINY_PDF = Path(__file__).parent / "fixtures" / "tiny.pdf"

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture(autouse=True)
def ingest_token():
    seed_dev_ingest_token()


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


def png_bytes(colour="#336699", second=None):
    img = Image.new("RGB", (100, 100), colour)
    if second is not None:
        img.paste(second, (0, 0, 40, 100))
    buf = io.BytesIO()
    img.save(buf, "PNG")
    return buf.getvalue()


def capture_image(data, title=None):
    with record("ingest", "test") as rec:
        item = rec.item("capture.image")
        rec.claim(item, "$blob", rec.blob(data, "image/png"))
        rec.claim(item, "width", 100)
        rec.claim(item, "height", 100)
        if title:
            rec.claim(item, "title", title, rank=Claim.Rank.PREFERRED)
    return item


def colour_hexes(item):
    derived = Link.objects.live().filter(target=item, rel="derived_from")
    hexes = []
    for link in derived:
        if link.source.type != "colour":
            continue
        hexv = link.source.current_values().get("hex")
        if hexv:
            hexes.append(hexv)
    return sorted(hexes)


def agent_fills(monkeypatch, fills):
    def fake_urlopen(request, timeout=None):
        return FakeResponse(completion_body(json.dumps({"fills": fills})))

    monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)


def test_screenshot_and_photo_have_no_colour_fields(core):
    descriptions = {row["name"]: row for row in core.describe()}
    for name in ("screenshot", "reference_shot"):
        properties = descriptions[name]["schema"].get("properties") or {}
        lowered = {key.lower() for key in properties}
        assert "hex" not in lowered
        assert "colour" not in lowered
        assert "color" not in lowered
        with pytest.raises(ValidationError):
            core.validate(name, {"hex": "#336699"})


def test_palette_hexes_survive_the_agent(core, blob_dir, monkeypatch):
    photo = capture_image(png_bytes(second=(230, 120, 40)), title="hallway lamp")
    chrome = capture_image(png_bytes("#111111", second=(40, 40, 40)), title="settings")
    execute_derive(core.processors["palette"], photo.id)
    execute_derive(core.processors["palette"], chrome.id)
    before_photo = colour_hexes(photo)
    before_chrome = colour_hexes(chrome)
    assert before_photo == ["#336699", "#e67828"]
    assert before_chrome
    colour_count = Item.objects.filter(type="colour").count()
    agent_fills(
        monkeypatch,
        [
            {
                "type": "reference_shot",
                "key": "lamp",
                "claims": {"subject": "a lamp in a shop"},
            },
            {
                "type": "colour",
                "key": "#ff0000",
                "claims": {"hex": "#ff0000"},
            },
        ],
    )
    execute_derive(core.processors["agent"], photo.id)
    agent_fills(
        monkeypatch,
        [
            {
                "type": "screenshot",
                "key": "settings",
                "claims": {"surface": "Settings"},
            },
            {
                "type": "colour",
                "key": "#ff0000",
                "claims": {"hex": "#ff0000"},
            },
        ],
    )
    execute_derive(core.processors["agent"], chrome.id)
    assert colour_hexes(photo) == before_photo
    assert colour_hexes(chrome) == before_chrome
    assert Item.objects.filter(type="colour").count() == colour_count
    shot = Item.objects.get(type="reference_shot")
    chrome_type = Item.objects.get(type="screenshot")
    assert shot.current_values()["subject"] == "a lamp in a shop"
    assert chrome_type.current_values()["surface"] == "Settings"
    assert "hex" not in shot.current_values()
    assert "hex" not in chrome_type.current_values()


def test_preferred_title_and_colour_survive_rederive(core, blob_dir, monkeypatch):
    item = capture_image(png_bytes(second=(230, 120, 40)), title="mine")
    execute_derive(core.processors["palette"], item.id)
    colour = next(
        link.source
        for link in Link.objects.live().filter(target=item, rel="derived_from")
        if link.source.type == "colour"
    )
    with record("edit", "user") as rec:
        rec.claim(colour, "hex", "#000000", rank=Claim.Rank.PREFERRED)
        rec.claim(item, "title", "still mine", rank=Claim.Rank.PREFERRED)
    agent_fills(
        monkeypatch,
        [
            {
                "type": "reference_shot",
                "key": "lamp",
                "claims": {"subject": "hallway lamp"},
            },
            {
                "type": "link",
                "key": "page",
                "claims": {"title": "model title", "url": "https://example.com"},
            },
        ],
    )
    execute_derive(core.processors["agent"], item.id)
    with record("derive", "agent@1", subject=item) as rec:
        rec.claim(item, "title", "model title")
        rec.claim(colour, "hex", "#336699")
    assert item.current_values()["title"] == "still mine"
    assert colour.current_values()["hex"] == "#000000"


def test_palette_runs_on_a_pdf_still(core, blob_dir, client, monkeypatch):
    upload = SimpleUploadedFile("spec.pdf", TINY_PDF.read_bytes(), "application/pdf")
    response = client.post("/ingest/", {"file": upload, "title": "spec"}, **TOKEN)
    assert response.status_code == 201, response.content
    document = Item.objects.get(id=response.json()["id"])
    execute_derive(core.processors["pdf"], document.id)
    still = Item.objects.get(type="capture.image")
    execute_derive(core.processors["palette"], still.id)
    before = colour_hexes(still)
    assert before
    agent_fills(
        monkeypatch,
        [
            {
                "type": "screenshot",
                "key": "spec",
                "claims": {"surface": "PDF spec"},
            },
            {
                "type": "colour",
                "key": "#00ff00",
                "claims": {"hex": "#00ff00"},
            },
        ],
    )
    execute_derive(core.processors["agent"], still.id)
    assert colour_hexes(still) == before
    assert document.current_values()["title"] == "spec"
    assert still.type == "capture.image"
