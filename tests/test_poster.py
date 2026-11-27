from pathlib import Path

import pytest
from django.core.files.uploadedfile import SimpleUploadedFile
from PIL import Image

from plugins.wall.queries import wall_items
from tests.factories import seed_dev_ingest_token
from trove.core import get_core, reset_core
from trove.models import Activity, Claim, Item, Link
from trove.tasks.execute import execute_derive

TOKEN = {"HTTP_AUTHORIZATION": "Bearer dev-shortcut-token"}
FIXTURES = Path(__file__).parent / "fixtures"
TINY_MP4 = FIXTURES / "tiny.mp4"

pytestmark = pytest.mark.django_db(databases=["default", "data"])


@pytest.fixture(autouse=True)
def ingest_token():
    seed_dev_ingest_token()


@pytest.fixture
def blob_dir(settings, tmp_path):
    path = tmp_path / "blobs"
    settings.TROVE = {**settings.TROVE, "BLOB_DIR": path}
    return path


@pytest.fixture
def core():
    reset_core()
    loaded = get_core()
    yield loaded
    reset_core()


def video(path=TINY_MP4, name="lamp.mp4", content_type="video/mp4"):
    return SimpleUploadedFile(name, path.read_bytes(), content_type)


def audio_only_mp4():
    def u32(n):
        return n.to_bytes(4, "big")

    def box(typ, payload):
        return u32(8 + len(payload)) + typ + payload

    identity = (
        u32(0x00010000) + u32(0) * 3 + u32(0x00010000) + u32(0) * 3 + u32(0x40000000)
    )
    mvhd = box(
        b"mvhd",
        b"\x00" * 12
        + u32(1000)
        + u32(500)
        + u32(0x00010000)
        + b"\x01\x00\x00\x00"
        + u32(0) * 2
        + identity
        + b"\x00" * 24
        + u32(2),
    )
    return box(b"ftyp", b"isom" + u32(0) + b"isommp41") + box(b"moov", mvhd)


def ingest_video(client, upload, title="lamp"):
    response = client.post("/ingest/", {"file": upload, "title": title}, **TOKEN)
    assert response.status_code == 201, response.content
    return Item.objects.get(id=response.json()["id"])


def derive_poster(core, item):
    execute_derive(core.processors["poster"], item.id)


def test_poster_processor_consumes_video(core):
    processor = core.processors["poster"]
    assert processor.consumes == "capture.video"
    assert processor.provides == ["capture.image"]


def test_tiny_mp4_emits_a_poster_image(client, blob_dir, core):
    item = ingest_video(client, video())
    derive_poster(core, item)
    still = Item.objects.get(type="capture.image")
    values = still.current_values()
    sha = values["$blob"]
    with Image.open(blob_dir / sha[:2] / sha) as img:
        assert img.size[0] > 0
        assert img.size[1] > 0
    link = Link.objects.live().get(rel="derived_from")
    assert link.source_id == still.id
    assert link.target_id == item.id
    act = Activity.objects.get(agent="poster@1", subject=item, outcome="ok")
    assert act.kind == "derive"


def test_duration_stays_the_ingest_claim(client, blob_dir, core):
    item = ingest_video(client, video())
    derive_poster(core, item)
    duration = Claim.objects.filter(item=item, path="duration")
    assert duration.count() == 1
    claim = duration.get()
    assert claim.value == 1
    assert claim.activity.kind == "ingest"
    assert claim.activity.agent != "poster@1"


def test_audio_only_video_fails_derive_and_keeps_the_blob(client, blob_dir, core):
    upload = SimpleUploadedFile("hum.mp4", audio_only_mp4(), "video/mp4")
    item = ingest_video(client, upload)
    sha = item.current_values()["$blob"]
    original = (blob_dir / sha[:2] / sha).read_bytes()
    derive_poster(core, item)
    act = Activity.objects.get(agent="poster@1", subject=item)
    assert act.outcome == "failed"
    assert (blob_dir / sha[:2] / sha).read_bytes() == original
    assert not Item.objects.filter(type="capture.image").exists()
    assert item.current_values()["duration"] == 0.5


def test_corrupt_video_fails_derive_and_keeps_the_blob(client, blob_dir, core):
    item = ingest_video(
        client, SimpleUploadedFile("clip.webm", b"not a container", "video/webm")
    )
    sha = item.current_values()["$blob"]
    original = (blob_dir / sha[:2] / sha).read_bytes()
    derive_poster(core, item)
    act = Activity.objects.get(agent="poster@1", subject=item)
    assert act.outcome == "failed"
    assert (blob_dir / sha[:2] / sha).read_bytes() == original
    assert not Item.objects.filter(type="capture.image").exists()


def test_wall_shows_the_poster_on_the_video(client, blob_dir, core):
    item = ingest_video(client, video())
    derive_poster(core, item)
    still = Item.objects.get(type="capture.image")
    payload = wall_items()["items"]
    ids = {row["id"] for row in payload}
    assert item.id in ids
    assert still.id not in ids
    row = next(entry for entry in payload if entry["id"] == item.id)
    link = next(entry for entry in row["links"] if entry["id"] == still.id)
    assert link["rel"] == "derived_from"
    assert link["type"] == "capture.image"
    assert link["values"]["$blob"]


def test_palette_can_consume_the_poster(client, blob_dir, core):
    item = ingest_video(client, video())
    derive_poster(core, item)
    still = Item.objects.get(type="capture.image")
    execute_derive(core.processors["palette"], still.id)
    assert Item.objects.filter(type="colour").exists()
